package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	redis "github.com/redis/go-redis/v9"
)

const (
	minBenchmarkBusinessID         uint64 = 9_000_000_000_000
	defaultBenchmarkRoundID        uint64 = 9_000_000_000_101
	defaultBenchmarkClassID        uint64 = 9_000_000_000_301
	defaultBenchmarkStudentIDStart uint64 = 9_100_000_000_000

	benchmarkDepartmentID  uint64 = 9_000_000_000_001
	benchmarkMajorID       uint64 = 9_000_000_000_002
	benchmarkTeacherID     uint64 = 9_000_000_000_003
	benchmarkTermID        uint64 = 9_000_000_000_004
	benchmarkCourseID      uint64 = 9_000_000_000_005
	benchmarkCourseCredits        = 3.0
)

type prepareConfig struct {
	MySQLDSN        string
	RedisAddr       string
	RedisPassword   string
	RedisDB         int
	RoundID         uint64
	TeachingClassID uint64
	StudentIDStart  uint64
	Users           int
	Capacity        int
	CreditLimit     float64
	CourseLimit     int
	BatchSize       int
	Timeout         time.Duration
	ConfirmReset    bool
}

type prepareReport struct {
	PreparedStudents int
	StudentIDEnd     uint64
	Capacity         int
}

func runPrepareCommand(args []string, stdout, stderr io.Writer) error {
	config, err := parsePrepareConfig(args, stderr)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)
	defer cancel()
	report, err := prepareBenchmarkData(ctx, config)
	if err != nil {
		return err
	}

	fmt.Fprintln(stdout, "CourseForge benchmark data prepared")
	fmt.Fprintf(stdout, "  round:            %d\n", config.RoundID)
	fmt.Fprintf(stdout, "  teaching class:   %d\n", config.TeachingClassID)
	fmt.Fprintf(stdout, "  students:         %d\n", report.PreparedStudents)
	fmt.Fprintf(stdout, "  student ID range: %d-%d\n", config.StudentIDStart, report.StudentIDEnd)
	fmt.Fprintf(stdout, "  class capacity:   %d\n", report.Capacity)
	fmt.Fprintf(stdout, "  credit limit:     %.1f\n", config.CreditLimit)
	fmt.Fprintf(stdout, "  course limit:     %d\n", config.CourseLimit)
	fmt.Fprintln(stdout, "  Redis quota/seat: ready")
	return nil
}

func parsePrepareConfig(args []string, output io.Writer) (prepareConfig, error) {
	config := prepareConfig{}
	flags := flag.NewFlagSet("benchmark prepare", flag.ContinueOnError)
	flags.SetOutput(output)

	flags.StringVar(&config.MySQLDSN, "mysql-dsn", os.Getenv("COURSEFORGE_BENCHMARK_MYSQL_DSN"), "courseforge MySQL DSN")
	flags.StringVar(&config.RedisAddr, "redis-addr", envOrDefault("COURSEFORGE_BENCHMARK_REDIS_ADDR", "127.0.0.1:6379"), "Redis 地址")
	flags.StringVar(&config.RedisPassword, "redis-password", os.Getenv("COURSEFORGE_BENCHMARK_REDIS_PASSWORD"), "Redis 密码（推荐通过环境变量传入）")
	flags.IntVar(&config.RedisDB, "redis-db", 0, "Redis DB")
	flags.Uint64Var(&config.RoundID, "round-id", defaultBenchmarkRoundID, "选课轮次 ID")
	flags.Uint64Var(&config.TeachingClassID, "teaching-class-id", defaultBenchmarkClassID, "教学班 ID")
	flags.Uint64Var(&config.StudentIDStart, "student-id-start", defaultBenchmarkStudentIDStart, "压测学生起始 ID")
	flags.IntVar(&config.Users, "users", 1000, "压测学生数量")
	flags.IntVar(&config.Capacity, "capacity", 1000, "教学班容量")
	flags.Float64Var(&config.CreditLimit, "credit-limit", 20.0, "每个学生的学分上限")
	flags.IntVar(&config.CourseLimit, "course-limit", 8, "每个学生的课程门数上限")
	flags.IntVar(&config.BatchSize, "batch-size", 500, "单次批量写入学生数")
	flags.DurationVar(&config.Timeout, "timeout", 2*time.Minute, "整个数据准备过程超时")
	flags.BoolVar(&config.ConfirmReset, "confirm-reset", false, "确认重置指定压测轮次、教学班和学生数据")

	if err := flags.Parse(args); err != nil {
		return prepareConfig{}, err
	}
	if flags.NArg() != 0 {
		return prepareConfig{}, fmt.Errorf("不支持位置参数: %s", strings.Join(flags.Args(), " "))
	}
	if err := config.validate(); err != nil {
		return prepareConfig{}, err
	}
	return config, nil
}

func (c prepareConfig) validate() error {
	if !c.ConfirmReset {
		return errors.New("prepare 会修改压测数据，请添加 --confirm-reset")
	}
	parsedDSN, err := mysqlDriver.ParseDSN(strings.TrimSpace(c.MySQLDSN))
	if err != nil {
		return fmt.Errorf("mysql-dsn 无效: %w", err)
	}
	if parsedDSN.DBName != "courseforge" {
		return fmt.Errorf("mysql-dsn 必须连接 courseforge，当前数据库为 %q", parsedDSN.DBName)
	}
	if strings.TrimSpace(c.RedisAddr) == "" {
		return errors.New("redis-addr 不能为空")
	}
	if c.RedisDB < 0 {
		return errors.New("redis-db 不能小于 0")
	}
	if c.RoundID < minBenchmarkBusinessID ||
		c.TeachingClassID < minBenchmarkBusinessID ||
		c.StudentIDStart < minBenchmarkBusinessID {
		return fmt.Errorf("round-id、teaching-class-id 和 student-id-start 必须使用 %d 以上的压测保留 ID", minBenchmarkBusinessID)
	}
	if c.Users <= 0 {
		return errors.New("users 必须大于 0")
	}
	if uint64(c.Users-1) > math.MaxUint64-c.StudentIDStart {
		return errors.New("student-id-start 与 users 组合发生溢出")
	}
	if c.Capacity <= 0 {
		return errors.New("capacity 必须大于 0")
	}
	if c.CreditLimit < benchmarkCourseCredits || math.IsNaN(c.CreditLimit) || math.IsInf(c.CreditLimit, 0) {
		return fmt.Errorf("credit-limit 必须至少为 %.1f", benchmarkCourseCredits)
	}
	if c.CourseLimit <= 0 || c.CourseLimit > math.MaxUint16 {
		return errors.New("course-limit 必须在 1 到 65535 之间")
	}
	if c.BatchSize <= 0 || c.BatchSize > 5000 {
		return errors.New("batch-size 必须在 1 到 5000 之间")
	}
	if c.Timeout <= 0 {
		return errors.New("timeout 必须大于 0")
	}
	return nil
}

func prepareBenchmarkData(ctx context.Context, config prepareConfig) (prepareReport, error) {
	database, err := openBenchmarkDB(ctx, config.MySQLDSN)
	if err != nil {
		return prepareReport{}, fmt.Errorf("连接 courseforge: %w", err)
	}
	defer database.Close()

	studentIDEnd := config.StudentIDStart + uint64(config.Users-1)
	if err := ensureBenchmarkStudentRangeSafe(ctx, database, config.StudentIDStart, studentIDEnd); err != nil {
		return prepareReport{}, err
	}
	if err := prepareCourseSelectionData(ctx, database, config, studentIDEnd); err != nil {
		return prepareReport{}, err
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})
	defer redisClient.Close()
	if err := redisClient.Ping(ctx).Err(); err != nil {
		return prepareReport{}, fmt.Errorf("连接 Redis: %w", err)
	}
	if err := resetAndPreheatSelectionRedis(ctx, redisClient, config); err != nil {
		return prepareReport{}, err
	}

	return prepareReport{
		PreparedStudents: config.Users,
		StudentIDEnd:     studentIDEnd,
		Capacity:         config.Capacity,
	}, nil
}

func openBenchmarkDB(ctx context.Context, dsn string) (*sql.DB, error) {
	database, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(4)
	database.SetMaxIdleConns(2)
	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, err
	}
	return database, nil
}

func ensureBenchmarkStudentRangeSafe(
	ctx context.Context,
	database *sql.DB,
	start uint64,
	end uint64,
) error {
	var count int
	err := database.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		   FROM student
		  WHERE id BETWEEN ? AND ?
		    AND student_no NOT LIKE 'BENCH-%'`,
		start,
		end,
	).Scan(&count)
	if err != nil {
		return fmt.Errorf("检查压测学生ID范围: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("学生ID范围 %d-%d 内存在 %d 条非 benchmark 数据，拒绝重置", start, end, count)
	}
	return nil
}

func prepareCourseSelectionData(
	ctx context.Context,
	database *sql.DB,
	config prepareConfig,
	studentIDEnd uint64,
) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("开始准备事务: %w", err)
	}
	defer tx.Rollback()

	resetStatements := []string{
		`DELETE oe FROM outbox_event oe
		  JOIN selection_application sa ON sa.application_id = oe.aggregate_id
		 WHERE oe.aggregate_type = 'selection_application'
		   AND sa.student_id BETWEEN ? AND ?`,
		`DELETE FROM selection_event WHERE student_id BETWEEN ? AND ?`,
		`DELETE FROM student_course_enrollment WHERE student_id BETWEEN ? AND ?`,
		`DELETE FROM selection_application WHERE student_id BETWEEN ? AND ?`,
		`DELETE FROM student_selection_quota WHERE round_id = ? AND student_id BETWEEN ? AND ?`,
	}
	for index, statement := range resetStatements {
		args := []any{config.StudentIDStart, studentIDEnd}
		if index == len(resetStatements)-1 {
			args = []any{config.RoundID, config.StudentIDStart, studentIDEnd}
		}
		if _, err := tx.ExecContext(ctx, statement, args...); err != nil {
			return fmt.Errorf("重置选课压测数据: %w", err)
		}
	}

	if err := upsertBenchmarkCatalog(ctx, tx, config); err != nil {
		return err
	}
	if err := upsertBenchmarkStudents(ctx, tx, config); err != nil {
		return err
	}
	if err := upsertBenchmarkQuotas(ctx, tx, config); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交准备事务: %w", err)
	}
	return nil
}

func upsertBenchmarkCatalog(ctx context.Context, tx *sql.Tx, config prepareConfig) error {
	statements := []struct {
		query string
		args  []any
	}{
		{
			`INSERT INTO department
				(id, department_code, department_name, state)
			 VALUES (?, 'BENCH-DEPT', 'Benchmark Department', 'active')
			 ON DUPLICATE KEY UPDATE department_name = VALUES(department_name), state = 'active'`,
			[]any{benchmarkDepartmentID},
		},
		{
			`INSERT INTO major
				(id, department_id, major_code, major_name, degree_type, state)
			 VALUES (?, ?, 'BENCH-MAJOR', 'Benchmark Major', 'bachelor', 'active')
			 ON DUPLICATE KEY UPDATE department_id = VALUES(department_id), state = 'active'`,
			[]any{benchmarkMajorID, benchmarkDepartmentID},
		},
		{
			`INSERT INTO teacher
				(id, teacher_no, teacher_name, department_id, title, state)
			 VALUES (?, 'BENCH-TEACHER', 'Benchmark Teacher', ?, 'Professor', 'active')
			 ON DUPLICATE KEY UPDATE department_id = VALUES(department_id), state = 'active'`,
			[]any{benchmarkTeacherID, benchmarkDepartmentID},
		},
		{
			`INSERT INTO academic_term
				(id, term_code, term_name, start_date, end_date, state)
			 VALUES (?, 'BENCH-TERM', 'Benchmark Term', CURDATE(), DATE_ADD(CURDATE(), INTERVAL 180 DAY), 'active')
			 ON DUPLICATE KEY UPDATE
				start_date = VALUES(start_date), end_date = VALUES(end_date), state = 'active'`,
			[]any{benchmarkTermID},
		},
		{
			`INSERT INTO course
				(id, course_code, course_name, department_id, credits, total_hours, course_type, description, state)
			 VALUES (?, 'BENCH-COURSE', 'Benchmark Course', ?, ?, 48, 'elective', 'Course selection benchmark fixture', 'active')
			 ON DUPLICATE KEY UPDATE
				department_id = VALUES(department_id), credits = VALUES(credits), state = 'active'`,
			[]any{benchmarkCourseID, benchmarkDepartmentID, benchmarkCourseCredits},
		},
		{
			`INSERT INTO teaching_class
				(id, class_code, term_id, course_id, teacher_id, campus, classroom,
				 capacity, selected_count, state, version)
			 VALUES (?, ?, ?, ?, ?, 'benchmark', 'benchmark', ?, 0, 'open', 0)
			 ON DUPLICATE KEY UPDATE
				term_id = VALUES(term_id), course_id = VALUES(course_id),
				teacher_id = VALUES(teacher_id), capacity = VALUES(capacity),
				selected_count = 0, state = 'open', version = version + 1`,
			[]any{
				config.TeachingClassID,
				fmt.Sprintf("BENCH-%d", config.TeachingClassID),
				benchmarkTermID,
				benchmarkCourseID,
				benchmarkTeacherID,
				config.Capacity,
			},
		},
		{
			`INSERT INTO selection_round
				(id, term_id, round_code, round_name, start_time, end_time,
				 default_credit_limit, default_course_limit, state)
			 VALUES (?, ?, ?, 'Benchmark Round',
				 DATE_SUB(NOW(3), INTERVAL 1 DAY), DATE_ADD(NOW(3), INTERVAL 7 DAY),
				 ?, ?, 'open')
			 ON DUPLICATE KEY UPDATE
				term_id = VALUES(term_id), start_time = VALUES(start_time),
				end_time = VALUES(end_time),
				default_credit_limit = VALUES(default_credit_limit),
				default_course_limit = VALUES(default_course_limit), state = 'open'`,
			[]any{
				config.RoundID,
				benchmarkTermID,
				fmt.Sprintf("BENCH-%d", config.RoundID),
				decimalOnePlace(config.CreditLimit),
				config.CourseLimit,
			},
		},
		{
			`INSERT INTO selection_round_class
				(round_id, teaching_class_id, state)
			 VALUES (?, ?, 'open')
			 ON DUPLICATE KEY UPDATE state = 'open'`,
			[]any{config.RoundID, config.TeachingClassID},
		},
	}
	for _, statement := range statements {
		if _, err := tx.ExecContext(ctx, statement.query, statement.args...); err != nil {
			return fmt.Errorf("准备选课目录数据: %w", err)
		}
	}
	return nil
}

func upsertBenchmarkStudents(ctx context.Context, tx *sql.Tx, config prepareConfig) error {
	for start := 0; start < config.Users; start += config.BatchSize {
		end := min(start+config.BatchSize, config.Users)
		count := end - start
		query := `INSERT INTO student
			(id, student_no, student_name, major_id, grade_year, degree_type, state)
			VALUES ` + repeatedValues(count, "(?, ?, ?, ?, ?, 'bachelor', 'active')") + `
			ON DUPLICATE KEY UPDATE
				student_no = VALUES(student_no),
				student_name = VALUES(student_name),
				major_id = VALUES(major_id),
				grade_year = VALUES(grade_year),
				state = 'active'`
		args := make([]any, 0, count*5)
		for index := start; index < end; index++ {
			studentID := config.StudentIDStart + uint64(index)
			args = append(
				args,
				studentID,
				fmt.Sprintf("BENCH-%d", studentID),
				fmt.Sprintf("Benchmark Student %d", index+1),
				benchmarkMajorID,
				time.Now().Year(),
			)
		}
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return fmt.Errorf("批量准备压测学生: %w", err)
		}
	}
	return nil
}

func upsertBenchmarkQuotas(ctx context.Context, tx *sql.Tx, config prepareConfig) error {
	for start := 0; start < config.Users; start += config.BatchSize {
		end := min(start+config.BatchSize, config.Users)
		count := end - start
		query := `INSERT INTO student_selection_quota
			(round_id, term_id, student_id, credit_limit, selected_credits,
			 course_limit, selected_course_count, version)
			VALUES ` + repeatedValues(count, "(?, ?, ?, ?, 0, ?, 0, 0)") + `
			ON DUPLICATE KEY UPDATE
				term_id = VALUES(term_id),
				credit_limit = VALUES(credit_limit),
				selected_credits = 0,
				course_limit = VALUES(course_limit),
				selected_course_count = 0,
				version = version + 1`
		args := make([]any, 0, count*5)
		for index := start; index < end; index++ {
			args = append(
				args,
				config.RoundID,
				benchmarkTermID,
				config.StudentIDStart+uint64(index),
				decimalOnePlace(config.CreditLimit),
				config.CourseLimit,
			)
		}
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return fmt.Errorf("批量准备学生选课额度: %w", err)
		}
	}
	return nil
}

func resetAndPreheatSelectionRedis(
	ctx context.Context,
	client *redis.Client,
	config prepareConfig,
) error {
	for index := 0; index < config.Users; index++ {
		studentID := config.StudentIDStart + uint64(index)
		pattern := fmt.Sprintf(
			"courseforge:selection:result:%d:%d:*",
			config.RoundID,
			studentID,
		)
		if err := deleteRedisPattern(ctx, client, pattern); err != nil {
			return fmt.Errorf("清理学生 %d 的选课幂等结果: %w", studentID, err)
		}
	}

	pipe := client.Pipeline()
	pipe.Set(
		ctx,
		fmt.Sprintf("courseforge:selection:class:seat:%d", config.TeachingClassID),
		config.Capacity,
		0,
	)
	for index := 0; index < config.Users; index++ {
		queueSelectionPreheat(
			ctx,
			pipe,
			config.RoundID,
			benchmarkTermID,
			benchmarkCourseID,
			config.StudentIDStart+uint64(index),
			creditUnits(config.CreditLimit),
			config.CourseLimit,
		)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("预热选课 Redis 资源: %w", err)
	}
	return nil
}

type selectionPreheatPipeline interface {
	Del(context.Context, ...string) *redis.IntCmd
	Set(context.Context, string, interface{}, time.Duration) *redis.StatusCmd
}

func queueSelectionPreheat(
	ctx context.Context,
	pipe selectionPreheatPipeline,
	roundID uint64,
	termID uint64,
	courseID uint64,
	studentID uint64,
	creditLimit int64,
	courseLimit int,
) {
	pipe.Del(
		ctx,
		fmt.Sprintf("courseforge:selection:pending:%d:%d", roundID, studentID),
		fmt.Sprintf("courseforge:selection:course:%d:%d:%d", termID, studentID, courseID),
	)
	pipe.Set(
		ctx,
		fmt.Sprintf("courseforge:selection:quota:credit:%d:%d", roundID, studentID),
		creditLimit,
		0,
	)
	pipe.Set(
		ctx,
		fmt.Sprintf("courseforge:selection:quota:course:%d:%d", roundID, studentID),
		courseLimit,
		0,
	)
}

func deleteRedisPattern(ctx context.Context, client *redis.Client, pattern string) error {
	var cursor uint64
	for {
		keys, next, err := client.Scan(ctx, cursor, pattern, 200).Result()
		if err != nil {
			return err
		}
		if len(keys) > 0 {
			if err := client.Del(ctx, keys...).Err(); err != nil {
				return err
			}
		}
		cursor = next
		if cursor == 0 {
			return nil
		}
	}
}

func repeatedValues(count int, value string) string {
	if count <= 0 {
		return ""
	}
	values := make([]string, count)
	for index := range values {
		values[index] = value
	}
	return strings.Join(values, ",")
}

func decimalOnePlace(value float64) string {
	return strconv.FormatFloat(value, 'f', 1, 64)
}

func creditUnits(value float64) int64 {
	return int64(math.Round(value * 10))
}

func envOrDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
