package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	redis "github.com/redis/go-redis/v9"
)

type verificationReport struct {
	Elapsed      time.Duration
	Expected     int64
	Acknowledged int64
	Scenario     benchmarkScenario
	Selection    selectionVerificationSnapshot
	Waitlist     waitlistVerificationSnapshot
}

type selectionVerificationSnapshot struct {
	Capacity             int64
	ClassSelected        int64
	Applications         int64
	SelectedApplications int64
	NonterminalApps      int64
	DistinctStudents     int64
	DistinctRequests     int64
	Enrollments          int64
	EnrollmentStudents   int64
	OutboxEvents         int64
	OutboxPublished      int64
	OutboxUnpublished    int64
	Notifications        int64
	RedisSeats           int64
	RedisPending         int64
}

type waitlistVerificationSnapshot struct {
	Entries          int64
	WaitingEntries   int64
	DistinctStudents int64
	DistinctRequests int64
}

func verifyBenchmark(
	ctx context.Context,
	config benchmarkConfig,
	summary benchmarkSummary,
) (verificationReport, error) {
	startedAt := time.Now()
	report := verificationReport{
		Acknowledged: summary.Stats.successfulOps,
		Scenario:     config.normalizedScenario(),
	}
	verifyCtx, cancel := context.WithTimeout(ctx, config.VerifyTimeout)
	defer cancel()

	database, err := openBenchmarkDB(verifyCtx, config.MySQLDSN)
	if err != nil {
		return report, fmt.Errorf("连接最终校验 MySQL: %w", err)
	}
	defer database.Close()

	if config.normalizedScenario() == scenarioWaitlist {
		report.Expected = int64(config.Users)
		for {
			report.Waitlist, err = readWaitlistVerificationSnapshot(verifyCtx, database, config)
			if err != nil {
				return report, err
			}
			if err = report.Waitlist.validate(report.Expected); err == nil {
				report.Elapsed = time.Since(startedAt)
				return report, nil
			}
			if waitErr := waitForVerificationPoll(verifyCtx, config.VerifyInterval); waitErr != nil {
				report.Elapsed = time.Since(startedAt)
				return report, fmt.Errorf("候补最终状态未在 %s 内收敛: %w; snapshot=%+v", config.VerifyTimeout, err, report.Waitlist)
			}
		}
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr:     config.RedisAddr,
		Password: config.RedisPassword,
		DB:       config.RedisDB,
	})
	defer redisClient.Close()
	if err := redisClient.Ping(verifyCtx).Err(); err != nil {
		return report, fmt.Errorf("连接最终校验 Redis: %w", err)
	}

	for {
		report.Selection, err = readSelectionVerificationSnapshot(
			verifyCtx,
			database,
			redisClient,
			config,
		)
		if err != nil {
			return report, err
		}
		report.Expected = min(int64(config.Users), report.Selection.Capacity)
		if fatalErr := report.Selection.fatalViolation(report.Expected); fatalErr != nil {
			report.Elapsed = time.Since(startedAt)
			return report, fatalErr
		}
		if err = report.Selection.validate(report.Expected); err == nil {
			report.Elapsed = time.Since(startedAt)
			return report, nil
		}
		if waitErr := waitForVerificationPoll(verifyCtx, config.VerifyInterval); waitErr != nil {
			report.Elapsed = time.Since(startedAt)
			return report, fmt.Errorf("选课最终状态未在 %s 内收敛: %w; snapshot=%+v", config.VerifyTimeout, err, report.Selection)
		}
	}
}

func readSelectionVerificationSnapshot(
	ctx context.Context,
	database *sql.DB,
	redisClient *redis.Client,
	config benchmarkConfig,
) (selectionVerificationSnapshot, error) {
	snapshot := selectionVerificationSnapshot{}
	studentIDEnd := config.StudentIDStart + uint64(config.Users-1)
	if err := database.QueryRowContext(
		ctx,
		`SELECT capacity, selected_count FROM teaching_class WHERE id = ?`,
		config.TeachingClassID,
	).Scan(&snapshot.Capacity, &snapshot.ClassSelected); err != nil {
		return snapshot, fmt.Errorf("查询教学班最终人数: %w", err)
	}

	if err := database.QueryRowContext(
		ctx,
		`SELECT
			COUNT(*),
			COALESCE(SUM(state = 'selected'), 0),
			COALESCE(SUM(state IN ('created', 'reserved', 'processing')), 0),
			COUNT(DISTINCT student_id),
			COUNT(DISTINCT request_id)
		 FROM selection_application
		 WHERE round_id = ?
		   AND teaching_class_id = ?
		   AND student_id BETWEEN ? AND ?`,
		config.RoundID,
		config.TeachingClassID,
		config.StudentIDStart,
		studentIDEnd,
	).Scan(
		&snapshot.Applications,
		&snapshot.SelectedApplications,
		&snapshot.NonterminalApps,
		&snapshot.DistinctStudents,
		&snapshot.DistinctRequests,
	); err != nil {
		return snapshot, fmt.Errorf("查询选课申请最终状态: %w", err)
	}

	if err := database.QueryRowContext(
		ctx,
		`SELECT COUNT(*), COUNT(DISTINCT student_id)
		 FROM student_course_enrollment
		 WHERE teaching_class_id = ?
		   AND student_id BETWEEN ? AND ?
		   AND state = 'enrolled'`,
		config.TeachingClassID,
		config.StudentIDStart,
		studentIDEnd,
	).Scan(&snapshot.Enrollments, &snapshot.EnrollmentStudents); err != nil {
		return snapshot, fmt.Errorf("查询正式选课最终状态: %w", err)
	}

	if err := database.QueryRowContext(
		ctx,
		`SELECT
			COUNT(*),
			COALESCE(SUM(oe.state = 'published'), 0),
			COALESCE(SUM(oe.state <> 'published'), 0)
		 FROM outbox_event oe
		 JOIN selection_application sa ON sa.application_id = oe.aggregate_id
		 WHERE oe.aggregate_type = 'selection_application'
		   AND oe.topic = 'enrollment.selection.notification'
		   AND sa.round_id = ?
		   AND sa.teaching_class_id = ?
		   AND sa.student_id BETWEEN ? AND ?`,
		config.RoundID,
		config.TeachingClassID,
		config.StudentIDStart,
		studentIDEnd,
	).Scan(
		&snapshot.OutboxEvents,
		&snapshot.OutboxPublished,
		&snapshot.OutboxUnpublished,
	); err != nil {
		return snapshot, fmt.Errorf("查询选课通知 Outbox 最终状态: %w", err)
	}

	if err := database.QueryRowContext(
		ctx,
		`SELECT COUNT(DISTINCT event_id)
		 FROM student_notification
		 WHERE type = 'selection_result'
		   AND student_id BETWEEN ? AND ?`,
		config.StudentIDStart,
		studentIDEnd,
	).Scan(&snapshot.Notifications); err != nil {
		return snapshot, fmt.Errorf("查询选课通知最终状态: %w", err)
	}

	seatKey := fmt.Sprintf("courseforge:selection:class:seat:%d", config.TeachingClassID)
	seats, err := redisClient.Get(ctx, seatKey).Int64()
	if err != nil {
		return snapshot, fmt.Errorf("查询 Redis 剩余名额 %q: %w", seatKey, err)
	}
	snapshot.RedisSeats = seats
	pending, err := countBenchmarkPendingKeys(ctx, redisClient, config)
	if err != nil {
		return snapshot, err
	}
	snapshot.RedisPending = pending
	return snapshot, nil
}

func readWaitlistVerificationSnapshot(
	ctx context.Context,
	database *sql.DB,
	config benchmarkConfig,
) (waitlistVerificationSnapshot, error) {
	snapshot := waitlistVerificationSnapshot{}
	studentIDEnd := config.StudentIDStart + uint64(config.Users-1)
	if err := database.QueryRowContext(
		ctx,
		`SELECT
			COUNT(*),
			COALESCE(SUM(state = 'waiting'), 0),
			COUNT(DISTINCT student_id),
			COUNT(DISTINCT request_id)
		 FROM selection_waitlist
		 WHERE round_id = ?
		   AND teaching_class_id = ?
		   AND student_id BETWEEN ? AND ?`,
		config.RoundID,
		config.TeachingClassID,
		config.StudentIDStart,
		studentIDEnd,
	).Scan(
		&snapshot.Entries,
		&snapshot.WaitingEntries,
		&snapshot.DistinctStudents,
		&snapshot.DistinctRequests,
	); err != nil {
		return snapshot, fmt.Errorf("查询候补最终状态: %w", err)
	}
	return snapshot, nil
}

func countBenchmarkPendingKeys(
	ctx context.Context,
	client *redis.Client,
	config benchmarkConfig,
) (int64, error) {
	pattern := fmt.Sprintf("courseforge:selection:pending:%d:*", config.RoundID)
	studentIDEnd := config.StudentIDStart + uint64(config.Users-1)
	var cursor uint64
	var count int64
	for {
		keys, next, err := client.Scan(ctx, cursor, pattern, 200).Result()
		if err != nil {
			return 0, fmt.Errorf("扫描 Redis pending: %w", err)
		}
		for _, key := range keys {
			separator := strings.LastIndexByte(key, ':')
			if separator < 0 {
				continue
			}
			studentID, err := strconv.ParseUint(key[separator+1:], 10, 64)
			if err == nil && studentID >= config.StudentIDStart && studentID <= studentIDEnd {
				count++
			}
		}
		cursor = next
		if cursor == 0 {
			return count, nil
		}
	}
}

func (s selectionVerificationSnapshot) fatalViolation(expected int64) error {
	if s.Capacity <= 0 {
		return errors.New("最终校验失败: 教学班容量必须大于 0")
	}
	if s.ClassSelected > s.Capacity || s.SelectedApplications > s.Capacity || s.Enrollments > s.Capacity {
		return fmt.Errorf("最终校验失败: 检测到超卖: snapshot=%+v", s)
	}
	if s.RedisSeats < 0 {
		return fmt.Errorf("最终校验失败: Redis 剩余名额为负数: %d", s.RedisSeats)
	}
	if s.Applications > expected || s.SelectedApplications > expected || s.Enrollments > expected || s.ClassSelected > expected {
		return fmt.Errorf("最终校验失败: 持久化成功数超过预期成功数 %d: snapshot=%+v", expected, s)
	}
	if s.Enrollments != s.EnrollmentStudents {
		return fmt.Errorf("最终校验失败: 同一学生存在重复正式选课: snapshot=%+v", s)
	}
	return nil
}

func (s selectionVerificationSnapshot) validate(expected int64) error {
	expectedSeats := s.Capacity - expected
	if s.Applications != expected ||
		s.SelectedApplications != expected ||
		s.NonterminalApps != 0 ||
		s.DistinctStudents != expected ||
		s.DistinctRequests != expected ||
		s.Enrollments != expected ||
		s.EnrollmentStudents != expected ||
		s.OutboxEvents != expected ||
		s.OutboxPublished != expected ||
		s.OutboxUnpublished != 0 ||
		s.Notifications != expected ||
		s.ClassSelected != expected ||
		s.RedisSeats != expectedSeats ||
		s.RedisPending != 0 {
		return fmt.Errorf("期望 %d 个成功操作和 %d 个剩余名额", expected, expectedSeats)
	}
	return nil
}

func (s waitlistVerificationSnapshot) validate(expected int64) error {
	if s.Entries != expected ||
		s.WaitingEntries != expected ||
		s.DistinctStudents != expected ||
		s.DistinctRequests != expected {
		return fmt.Errorf("期望 %d 条唯一 waiting 候补记录", expected)
	}
	return nil
}

func waitForVerificationPoll(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func printVerificationReport(report verificationReport) {
	fmt.Println()
	fmt.Println("Final-state verification")
	fmt.Printf("  scenario:          %s\n", report.Scenario)
	fmt.Printf("  acknowledged:      %d\n", report.Acknowledged)
	fmt.Printf("  expected success:  %d\n", report.Expected)
	fmt.Printf("  verification time: %s\n", report.Elapsed.Round(time.Millisecond))
	if report.Scenario == scenarioWaitlist {
		fmt.Printf("  waitlist entries:  %d\n", report.Waitlist.Entries)
		fmt.Printf("  waiting entries:   %d\n", report.Waitlist.WaitingEntries)
		fmt.Println("  result:             passed")
		return
	}
	fmt.Printf("  class capacity:    %d\n", report.Selection.Capacity)
	fmt.Printf("  class selected:    %d\n", report.Selection.ClassSelected)
	fmt.Printf("  applications:      %d\n", report.Selection.Applications)
	fmt.Printf("  enrollments:       %d\n", report.Selection.Enrollments)
	fmt.Printf("  Outbox published:  %d/%d\n", report.Selection.OutboxPublished, report.Selection.OutboxEvents)
	fmt.Printf("  notifications:     %d\n", report.Selection.Notifications)
	fmt.Printf("  Redis seats:       %d\n", report.Selection.RedisSeats)
	fmt.Printf("  Redis pending:     %d\n", report.Selection.RedisPending)
	fmt.Println("  result:             passed")
}
