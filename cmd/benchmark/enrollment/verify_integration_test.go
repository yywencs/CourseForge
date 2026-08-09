//go:build integration

package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	redis "github.com/redis/go-redis/v9"
)

func TestVerifyBenchmarkFinalStateAgainstMySQLAndRedis(t *testing.T) {
	mysqlDSN := integrationEnvOrDefault(
		"COURSEFORGE_INTEGRATION_MYSQL_DSN",
		"root:courseforge-integration@tcp(127.0.0.1:13306)/courseforge?charset=utf8mb4&parseTime=True&loc=Local&timeout=5s",
	)
	redisAddr := integrationEnvOrDefault(
		"COURSEFORGE_INTEGRATION_REDIS_ADDR",
		"127.0.0.1:16379",
	)
	prepare := prepareConfig{
		MySQLDSN:        mysqlDSN,
		RedisAddr:       redisAddr,
		RoundID:         defaultBenchmarkRoundID,
		TeachingClassID: defaultBenchmarkClassID,
		StudentIDStart:  defaultBenchmarkStudentIDStart,
		Users:           2,
		Capacity:        1,
		CreditLimit:     20,
		CourseLimit:     8,
		BatchSize:       2,
		Timeout:         30 * time.Second,
		ConfirmReset:    true,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := prepareBenchmarkData(ctx, prepare); err != nil {
		t.Fatalf("prepareBenchmarkData() error = %v", err)
	}

	database, err := openBenchmarkDB(ctx, mysqlDSN)
	if err != nil {
		t.Fatalf("openBenchmarkDB() error = %v", err)
	}
	defer database.Close()
	applicationID := "benchmark-verify-app"
	requestID := "benchmark-verify-request"
	enrollmentID := "benchmark-verify-enrollment"
	studentID := defaultBenchmarkStudentIDStart
	eventID := fmt.Sprintf("selection:%d:%s", studentID, applicationID)
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE teaching_class SET selected_count = 1 WHERE id = ?`,
		defaultBenchmarkClassID,
	); err != nil {
		_ = tx.Rollback()
		t.Fatalf("update teaching class: %v", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE student_selection_quota
		 SET selected_credits = 3.0, selected_course_count = 1
		 WHERE round_id = ? AND student_id = ?`,
		defaultBenchmarkRoundID,
		studentID,
	); err != nil {
		_ = tx.Rollback()
		t.Fatalf("update student quota: %v", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO selection_application
			(application_id, request_id, round_id, term_id, student_id, course_id,
			 teaching_class_id, credits, state, applied_at, completed_at, source)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 3.0, 'selected', NOW(3), NOW(3), 'web')`,
		applicationID,
		requestID,
		defaultBenchmarkRoundID,
		benchmarkTermID,
		studentID,
		benchmarkCourseID,
		defaultBenchmarkClassID,
	); err != nil {
		_ = tx.Rollback()
		t.Fatalf("insert selection application: %v", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO student_course_enrollment
			(enrollment_id, application_id, term_id, student_id, course_id,
			 teaching_class_id, credits, state, active_key, enrolled_at)
		 VALUES (?, ?, ?, ?, ?, ?, 3.0, 'enrolled', ?, NOW(3))`,
		enrollmentID,
		applicationID,
		benchmarkTermID,
		studentID,
		benchmarkCourseID,
		defaultBenchmarkClassID,
		fmt.Sprintf("%d:%d:%d", benchmarkTermID, studentID, benchmarkCourseID),
	); err != nil {
		_ = tx.Rollback()
		t.Fatalf("insert enrollment: %v", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox_event
			(event_id, aggregate_type, aggregate_id, topic, event_type, payload,
			 state, next_retry_at, published_at)
		 VALUES (?, 'selection_application', ?, 'enrollment.selection.notification',
			 'selection.result.persisted', JSON_OBJECT('student_id', ?),
			 'published', NOW(3), NOW(3))`,
		eventID,
		applicationID,
		studentID,
	); err != nil {
		_ = tx.Rollback()
		t.Fatalf("insert selection notification Outbox: %v", err)
	}
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO student_notification
			(event_id, student_id, type, title, content, payload, occurred_at)
		 VALUES (?, ?, 'selection_result', '选课成功', '测试通知',
			 JSON_OBJECT('application_id', ?), NOW(3))`,
		eventID,
		studentID,
		applicationID,
	); err != nil {
		_ = tx.Rollback()
		t.Fatalf("insert selection notification: %v", err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatalf("Commit() error = %v", err)
	}

	redisClient := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer redisClient.Close()
	seatKey := fmt.Sprintf("courseforge:selection:class:seat:%d", defaultBenchmarkClassID)
	if err := redisClient.Set(ctx, seatKey, 0, 0).Err(); err != nil {
		t.Fatalf("set Redis seat: %v", err)
	}

	config := benchmarkConfig{
		Scenario:        scenarioSelection,
		RoundID:         defaultBenchmarkRoundID,
		TeachingClassID: defaultBenchmarkClassID,
		StudentIDStart:  defaultBenchmarkStudentIDStart,
		Users:           2,
		MySQLDSN:        mysqlDSN,
		RedisAddr:       redisAddr,
		VerifyTimeout:   2 * time.Second,
		VerifyInterval:  10 * time.Millisecond,
	}
	report, err := verifyBenchmark(ctx, config, benchmarkSummary{Stats: benchmarkStats{successfulOps: 1}})
	if err != nil {
		t.Fatalf("verifyBenchmark() error = %v", err)
	}
	if report.Selection.Enrollments != 1 || report.Selection.RedisSeats != 0 {
		t.Fatalf("verification report = %+v", report)
	}
}

func integrationEnvOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
