//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"prizeforge/internal/domain/enrollment"
	"prizeforge/internal/infrastructure/adapter"
	"prizeforge/internal/infrastructure/repository/enrollmentrepo"
	"prizeforge/internal/job"
	"prizeforge/internal/listener"
	"prizeforge/pkg/xrand"
)

// TestEnrollmentRepositoryMinimalMainChain 使用真实 MySQL 和 Redis 验证：
// 原子预占 → 完成结果/Stream → RabbitMQ Confirm → MySQL幂等落库。
func TestEnrollmentRepositoryMinimalMainChain(t *testing.T) {
	const (
		departmentID  = uint64(990001)
		majorID       = uint64(990001)
		studentID     = uint64(990001)
		teacherID     = uint64(990001)
		termID        = uint64(990001)
		courseID      = uint64(990001)
		classID       = uint64(990001)
		roundID       = uint64(990001)
		applicationID = "019d0000000000000000000000000001"
		requestID     = "integration-selection-request-001"
	)

	now := time.Now().Truncate(time.Millisecond)
	seedEnrollmentIntegrationData(
		t,
		now,
		departmentID,
		majorID,
		studentID,
		teacherID,
		termID,
		courseID,
		classID,
		roundID,
	)
	cleanupEnrollmentRedis(
		t,
		roundID,
		termID,
		studentID,
		courseID,
		classID,
		requestID,
	)

	repo := enrollmentrepo.NewRepository(integrationCourseforgeDB, integrationRedis)
	round, err := repo.QuerySelectionRound(context.Background(), roundID)
	if err != nil || round == nil || !round.AcceptingAt(now) {
		t.Fatalf("QuerySelectionRound() = %#v, %v", round, err)
	}
	class, err := repo.QueryTeachingClass(context.Background(), roundID, classID)
	if err != nil || class == nil {
		t.Fatalf("QueryTeachingClass() = %#v, %v", class, err)
	}
	quota, err := repo.QueryStudentSelectionQuota(context.Background(), roundID, studentID)
	if err != nil || quota == nil {
		t.Fatalf("QueryStudentSelectionQuota() = %#v, %v", quota, err)
	}

	request := &enrollment.SelectionRequest{
		RequestID:       requestID,
		RoundID:         roundID,
		TermID:          termID,
		StudentID:       studentID,
		CourseID:        courseID,
		TeachingClassID: classID,
		Credits:         class.Credits,
		Source:          enrollment.ApplicationSourceWeb,
	}
	application, err := enrollment.NewSelectionApplication(applicationID, request, now)
	if err != nil {
		t.Fatalf("NewSelectionApplication() error = %v", err)
	}

	reservation, err := repo.ReserveSelection(context.Background(), application)
	if err != nil {
		t.Fatalf("ReserveSelection() error = %v", err)
	}
	if reservation.Status != enrollment.ReservationStatusAcquired ||
		reservation.Application.State != enrollment.ApplicationStateReserved {
		t.Fatalf("ReserveSelection() = %#v", reservation)
	}
	pendingRecord, err := repo.QuerySelectionByRequest(
		context.Background(),
		roundID,
		studentID,
		requestID,
	)
	if err != nil ||
		pendingRecord == nil ||
		pendingRecord.Application == nil ||
		pendingRecord.Application.ApplicationID != applicationID ||
		pendingRecord.Application.State != enrollment.ApplicationStateReserved ||
		pendingRecord.Publication != nil ||
		pendingRecord.MySQLPersisted {
		t.Fatalf("QuerySelectionByRequest(pending) = %#v, %v", pendingRecord, err)
	}

	assertRedisInt(t, fmt.Sprintf("courseforge:selection:quota:credit:%d:%d", roundID, studentID), 165)
	assertRedisInt(t, fmt.Sprintf("courseforge:selection:quota:course:%d:%d", roundID, studentID), 5)
	assertRedisInt(t, fmt.Sprintf("courseforge:selection:class:seat:%d", classID), 1)

	result, err := reservation.Application.CompleteSelected(now.Add(2 * time.Second))
	if err != nil {
		t.Fatalf("CompleteSelected() error = %v", err)
	}
	publication, err := repo.CompleteSelection(context.Background(), result)
	if err != nil {
		t.Fatalf("CompleteSelection() error = %v", err)
	}
	if publication.StreamID == "" || publication.BrokerConfirmed {
		t.Fatalf("CompleteSelection() publication = %#v", publication)
	}
	reusedPublication, err := repo.CompleteSelection(context.Background(), result)
	if err != nil ||
		reusedPublication.StreamID != publication.StreamID ||
		reusedPublication.Result.ApplicationID != applicationID {
		t.Fatalf("CompleteSelection(retry) = %#v, %v", reusedPublication, err)
	}
	completedRecord, err := repo.QuerySelectionByRequest(
		context.Background(),
		roundID,
		studentID,
		requestID,
	)
	if err != nil ||
		completedRecord == nil ||
		completedRecord.Publication == nil ||
		completedRecord.Application == nil ||
		completedRecord.Application.State != enrollment.ApplicationStateSelected ||
		completedRecord.MySQLPersisted {
		t.Fatalf("QuerySelectionByRequest(result) = %#v, %v", completedRecord, err)
	}
	if length := integrationRedisClient.XLen(
		context.Background(),
		"courseforge:selection:result:stream",
	).Val(); length != 1 {
		t.Fatalf("selection result stream length = %d, want 1", length)
	}

	// 使用独立 topic 跑真实 RabbitMQ，避免集成测试之间相互消费消息。
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	topic := "prizeforge.integration.selection-result." + xrand.RandomNumeric(12)
	trackIntegrationRabbitMQTopology(t, topic)
	connection, err := adapter.NewConnection(integrationRabbitMQConfig)
	if err != nil {
		t.Fatalf("connect selection-result RabbitMQ: %v", err)
	}
	consumer := listener.NewRabbitMQConsumer(connection)
	t.Cleanup(consumer.Shutdown)
	consumer.RegisterListener(topic, listener.NewSelectionResultListener(repo))
	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("start selection-result consumer: %v", err)
	}

	rabbitPublisher, err := adapter.NewRabbitMQPublisher(connection, 1)
	if err != nil {
		t.Fatalf("create selection-result publisher: %v", err)
	}
	publisherConfig := *integrationRabbitMQConfig
	publisherConfig.Topic.SelectionResult = topic
	selectionPublisher := job.NewSelectionResultPublisher(
		repo,
		adapter.NewPublisher(rabbitPublisher, &publisherConfig),
	)
	if err := selectionPublisher.Publish(ctx, publication); err != nil {
		t.Fatalf("publish selection result with confirm: %v", err)
	}
	if length := integrationRedisClient.XLen(
		context.Background(),
		"courseforge:selection:result:stream",
	).Val(); length != 0 {
		t.Fatalf("selection result stream length after confirm = %d, want 0", length)
	}

	// RabbitMQ Confirm 只代表 Broker 收到消息；MySQL 消费落库是异步的，因此轮询结果。
	waitForSelectionResultPersisted(t, ctx, applicationID)

	// 重复消费同一标准结果不应产生第二份申请单或选课记录。
	if err := repo.SaveSelectionResult(context.Background(), result); err != nil {
		t.Fatalf("idempotent SaveSelectionResult() error = %v", err)
	}

	var applicationCount, enrollmentCount int64
	integrationCourseforgeDB.Table("selection_application").
		Where("application_id = ?", applicationID).
		Count(&applicationCount)
	integrationCourseforgeDB.Table("student_course_enrollment").
		Where("application_id = ?", applicationID).
		Count(&enrollmentCount)
	if applicationCount != 1 || enrollmentCount != 1 {
		t.Fatalf(
			"persisted counts = application:%d enrollment:%d, want 1/1",
			applicationCount,
			enrollmentCount,
		)
	}
	var persistedQuota struct {
		SelectedCredits     string
		SelectedCourseCount uint16
	}
	if err := integrationCourseforgeDB.Table("student_selection_quota").
		Select(
			"CAST(selected_credits AS CHAR) AS selected_credits, selected_course_count",
		).
		Where("round_id = ? AND student_id = ?", roundID, studentID).
		Take(&persistedQuota).Error; err != nil {
		t.Fatalf("query persisted quota: %v", err)
	}
	if persistedQuota.SelectedCredits != "3.5" && persistedQuota.SelectedCredits != "3.50" {
		t.Fatalf("selected credits = %q, want 3.5", persistedQuota.SelectedCredits)
	}
	if persistedQuota.SelectedCourseCount != 1 {
		t.Fatalf("selected course count = %d, want 1", persistedQuota.SelectedCourseCount)
	}

	// 删除 Redis 的短期结果和 pending，验证幂等查询会回退 MySQL 唯一申请记录。
	if err := integrationRedisClient.Del(
		context.Background(),
		fmt.Sprintf(
			"courseforge:selection:result:%d:%d:%s",
			roundID,
			studentID,
			requestID,
		),
		fmt.Sprintf("courseforge:selection:pending:%d:%d", roundID, studentID),
	).Err(); err != nil {
		t.Fatalf("delete Redis selection lookup state: %v", err)
	}
	persistedRecord, err := repo.QuerySelectionByRequest(
		context.Background(),
		roundID,
		studentID,
		requestID,
	)
	if err != nil ||
		persistedRecord == nil ||
		persistedRecord.Application == nil ||
		persistedRecord.Application.ApplicationID != applicationID ||
		persistedRecord.Application.State != enrollment.ApplicationStateSelected ||
		persistedRecord.Publication != nil ||
		!persistedRecord.MySQLPersisted {
		t.Fatalf("QuerySelectionByRequest(MySQL) = %#v, %v", persistedRecord, err)
	}
}

// TestEnrollmentRepositoryConcurrentReservationDoesNotOversell 验证100个学生
// 并发抢10个名额时，Redis原子预占严格限制成功数且不会出现负库存。
func TestEnrollmentRepositoryConcurrentReservationDoesNotOversell(t *testing.T) {
	const (
		studentCount = 100
		capacity     = 10

		departmentID = uint64(991001)
		majorID      = uint64(991001)
		studentID    = uint64(991100)
		teacherID    = uint64(991001)
		termID       = uint64(991001)
		courseID     = uint64(991001)
		classID      = uint64(991001)
		roundID      = uint64(991001)
	)

	now := time.Now().Truncate(time.Millisecond)
	seedEnrollmentIntegrationData(
		t,
		now,
		departmentID,
		majorID,
		studentID,
		teacherID,
		termID,
		courseID,
		classID,
		roundID,
	)
	if err := integrationCourseforgeDB.Table("teaching_class").
		Where("id = ?", classID).
		Update("capacity", capacity).Error; err != nil {
		t.Fatalf("set concurrent test class capacity: %v", err)
	}

	t.Cleanup(func() {
		integrationCourseforgeDB.Exec(
			"DELETE FROM student_selection_quota WHERE student_id > ? AND student_id < ?",
			studentID,
			studentID+studentCount,
		)
		integrationCourseforgeDB.Exec(
			"DELETE FROM student WHERE id > ? AND id < ?",
			studentID,
			studentID+studentCount,
		)
	})
	for index := 1; index < studentCount; index++ {
		currentStudentID := studentID + uint64(index)
		if err := integrationCourseforgeDB.Table("student").Create(map[string]interface{}{
			"id":           currentStudentID,
			"student_no":   fmt.Sprintf("IT-CS-%03d", index),
			"student_name": fmt.Sprintf("并发测试学生%03d", index),
			"major_id":     majorID,
			"grade_year":   2026,
		}).Error; err != nil {
			t.Fatalf("seed concurrent student %d: %v", currentStudentID, err)
		}
		if err := integrationCourseforgeDB.Table("student_selection_quota").Create(
			map[string]interface{}{
				"round_id":              roundID,
				"term_id":               termID,
				"student_id":            currentStudentID,
				"credit_limit":          "20.0",
				"selected_credits":      "0.0",
				"course_limit":          6,
				"selected_course_count": 0,
			},
		).Error; err != nil {
			t.Fatalf("seed concurrent student quota %d: %v", currentStudentID, err)
		}
	}

	requestIDs := make([]string, studentCount)
	redisKeys := []string{
		fmt.Sprintf("courseforge:selection:class:seat:%d", classID),
		"courseforge:selection:result:stream",
	}
	for index := 0; index < studentCount; index++ {
		currentStudentID := studentID + uint64(index)
		requestIDs[index] = fmt.Sprintf("integration-concurrent-request-%03d", index)
		redisKeys = append(
			redisKeys,
			fmt.Sprintf("courseforge:selection:quota:credit:%d:%d", roundID, currentStudentID),
			fmt.Sprintf("courseforge:selection:quota:course:%d:%d", roundID, currentStudentID),
			fmt.Sprintf("courseforge:selection:pending:%d:%d", roundID, currentStudentID),
			fmt.Sprintf(
				"courseforge:selection:result:%d:%d:%s",
				roundID,
				currentStudentID,
				requestIDs[index],
			),
			fmt.Sprintf(
				"courseforge:selection:course:%d:%d:%d",
				termID,
				currentStudentID,
				courseID,
			),
		)
	}
	if err := integrationRedisClient.Del(context.Background(), redisKeys...).Err(); err != nil {
		t.Fatalf("cleanup concurrent selection Redis before test: %v", err)
	}
	t.Cleanup(func() {
		_ = integrationRedisClient.Del(context.Background(), redisKeys...).Err()
	})

	type reservationResult struct {
		reservation *enrollment.SelectionReservation
		err         error
	}
	repo := enrollmentrepo.NewRepository(integrationCourseforgeDB, integrationRedis)
	start := make(chan struct{})
	results := make(chan reservationResult, studentCount)
	var workers sync.WaitGroup
	workers.Add(studentCount)
	for index := 0; index < studentCount; index++ {
		index := index
		go func() {
			defer workers.Done()
			<-start

			application, err := enrollment.NewSelectionApplication(
				fmt.Sprintf("concurrent-app-%03d", index),
				&enrollment.SelectionRequest{
					RequestID:       requestIDs[index],
					RoundID:         roundID,
					TermID:          termID,
					StudentID:       studentID + uint64(index),
					CourseID:        courseID,
					TeachingClassID: classID,
					Credits:         enrollment.Credit(35),
					Source:          enrollment.ApplicationSourceWeb,
				},
				now,
			)
			if err != nil {
				results <- reservationResult{err: err}
				return
			}
			reservation, err := repo.ReserveSelection(context.Background(), application)
			results <- reservationResult{reservation: reservation, err: err}
		}()
	}
	close(start)
	workers.Wait()
	close(results)

	var acquired, full int
	for result := range results {
		switch {
		case result.err == nil:
			if result.reservation == nil ||
				result.reservation.Status != enrollment.ReservationStatusAcquired ||
				result.reservation.Application == nil ||
				result.reservation.Application.State != enrollment.ApplicationStateReserved {
				t.Fatalf("unexpected successful reservation: %#v", result.reservation)
			}
			acquired++
		case errors.Is(result.err, enrollment.ErrTeachingClassFull):
			full++
		default:
			t.Fatalf("unexpected concurrent reservation error: %v", result.err)
		}
	}
	if acquired != capacity || full != studentCount-capacity {
		t.Fatalf(
			"concurrent reservation results = acquired:%d full:%d, want %d/%d",
			acquired,
			full,
			capacity,
			studentCount-capacity,
		)
	}
	assertRedisInt(
		t,
		fmt.Sprintf("courseforge:selection:class:seat:%d", classID),
		0,
	)
}

func waitForSelectionResultPersisted(
	t *testing.T,
	ctx context.Context,
	applicationID string,
) {
	t.Helper()
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		var count int64
		err := integrationCourseforgeDB.Table("selection_application").
			Where("application_id = ?", applicationID).
			Count(&count).Error
		if err != nil {
			t.Fatalf("query asynchronous selection result: %v", err)
		}
		if count == 1 {
			return
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for asynchronous selection result: %v", ctx.Err())
		case <-ticker.C:
		}
	}
}

func seedEnrollmentIntegrationData(
	t *testing.T,
	now time.Time,
	departmentID, majorID, studentID, teacherID, termID, courseID, classID, roundID uint64,
) {
	t.Helper()
	db := integrationCourseforgeDB
	rows := []struct {
		table string
		value map[string]interface{}
	}{
		{"department", map[string]interface{}{
			"id": departmentID, "department_code": "IT-D", "department_name": "集成测试学院",
		}},
		{"major", map[string]interface{}{
			"id": majorID, "department_id": departmentID, "major_code": "IT-M",
			"major_name": "集成测试专业",
		}},
		{"student", map[string]interface{}{
			"id": studentID, "student_no": "IT-S-001", "student_name": "测试学生",
			"major_id": majorID, "grade_year": 2026,
		}},
		{"teacher", map[string]interface{}{
			"id": teacherID, "teacher_no": "IT-T-001", "teacher_name": "测试教师",
			"department_id": departmentID,
		}},
		{"academic_term", map[string]interface{}{
			"id": termID, "term_code": "IT-2026", "term_name": "集成测试学期",
			"start_date": now.AddDate(0, -1, 0), "end_date": now.AddDate(0, 1, 0),
			"state": "active",
		}},
		{"course", map[string]interface{}{
			"id": courseID, "course_code": "IT-C-001", "course_name": "分布式系统",
			"department_id": departmentID, "credits": "3.5", "course_type": "elective",
		}},
		{"teaching_class", map[string]interface{}{
			"id": classID, "class_code": "IT-CL-001", "term_id": termID,
			"course_id": courseID, "teacher_id": teacherID, "capacity": 2,
			"selected_count": 0, "state": "open",
		}},
		{"selection_round", map[string]interface{}{
			"id": roundID, "term_id": termID, "round_code": "IT-R-001",
			"round_name": "集成测试轮次", "start_time": now.Add(-time.Hour),
			"end_time": now.Add(time.Hour), "default_credit_limit": "20.0",
			"default_course_limit": 6, "state": "open",
		}},
		{"selection_round_class", map[string]interface{}{
			"round_id": roundID, "teaching_class_id": classID, "state": "open",
		}},
		{"student_selection_quota", map[string]interface{}{
			"round_id": roundID, "term_id": termID, "student_id": studentID,
			"credit_limit": "20.0", "selected_credits": "0.0",
			"course_limit": 6, "selected_course_count": 0,
		}},
	}
	for _, row := range rows {
		if err := db.Table(row.table).Create(row.value).Error; err != nil {
			t.Fatalf("seed %s: %v", row.table, err)
		}
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM selection_event WHERE student_id = ?", studentID)
		db.Exec("DELETE FROM student_course_enrollment WHERE student_id = ?", studentID)
		db.Exec("DELETE FROM selection_application WHERE student_id = ?", studentID)
		db.Exec("DELETE FROM student_selection_quota WHERE student_id = ?", studentID)
		db.Exec("DELETE FROM selection_round_class WHERE round_id = ?", roundID)
		db.Exec("DELETE FROM selection_round WHERE id = ?", roundID)
		db.Exec("DELETE FROM teaching_class WHERE id = ?", classID)
		db.Exec("DELETE FROM course WHERE id = ?", courseID)
		db.Exec("DELETE FROM academic_term WHERE id = ?", termID)
		db.Exec("DELETE FROM teacher WHERE id = ?", teacherID)
		db.Exec("DELETE FROM student WHERE id = ?", studentID)
		db.Exec("DELETE FROM major WHERE id = ?", majorID)
		db.Exec("DELETE FROM department WHERE id = ?", departmentID)
	})
}

func cleanupEnrollmentRedis(
	t *testing.T,
	roundID, termID, studentID, courseID, classID uint64,
	requestID string,
) {
	t.Helper()
	keys := []string{
		fmt.Sprintf("courseforge:selection:quota:credit:%d:%d", roundID, studentID),
		fmt.Sprintf("courseforge:selection:quota:course:%d:%d", roundID, studentID),
		fmt.Sprintf("courseforge:selection:class:seat:%d", classID),
		fmt.Sprintf("courseforge:selection:pending:%d:%d", roundID, studentID),
		fmt.Sprintf("courseforge:selection:result:%d:%d:%s", roundID, studentID, requestID),
		fmt.Sprintf("courseforge:selection:course:%d:%d:%d", termID, studentID, courseID),
		"courseforge:selection:result:stream",
	}
	if err := integrationRedisClient.Del(context.Background(), keys...).Err(); err != nil {
		t.Fatalf("cleanup enrollment Redis before test: %v", err)
	}
	t.Cleanup(func() {
		_ = integrationRedisClient.Del(context.Background(), keys...).Err()
	})
}

func assertRedisInt(t *testing.T, key string, want int64) {
	t.Helper()
	got, err := integrationRedisClient.Get(context.Background(), key).Int64()
	if err != nil {
		t.Fatalf("GET %s: %v", key, err)
	}
	if got != want {
		t.Fatalf("GET %s = %d, want %d", key, got, want)
	}
}
