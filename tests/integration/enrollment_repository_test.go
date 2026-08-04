//go:build integration

package integration

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	application "github.com/yywencs/courseforge/internal/enrollment/application"
	enrollmentasync "github.com/yywencs/courseforge/internal/enrollment/async"
	"github.com/yywencs/courseforge/internal/enrollment/domain"
	"github.com/yywencs/courseforge/internal/platform/rabbitmq"
	"github.com/yywencs/courseforge/pkg/xrand"
)

// TestEnrollmentRepositoryMinimalMainChain 使用真实 MySQL 和 Redis 验证：
// 原子预占 → 完成结果/Stream → RabbitMQ Confirm → MySQL幂等落库。
func TestEnrollmentRepositoryMinimalMainChain(t *testing.T) {
	const (
		majorID       = uint64(990001)
		studentID     = uint64(990001)
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
		majorID,
		studentID,
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

	repo := newEnrollmentRepositoryFixture(integrationCourseForgeDB, integrationRedis)
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
	selectionApplication, err := enrollment.NewSelectionApplication(applicationID, request, now)
	if err != nil {
		t.Fatalf("NewSelectionApplication() error = %v", err)
	}

	reservation, err := repo.ReserveSelection(context.Background(), selectionApplication)
	if err != nil {
		t.Fatalf("ReserveSelection() error = %v", err)
	}
	if reservation.Status != application.ReservationStatusAcquired ||
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
		pendingRecord.DurablyPersisted {
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
	if publication.DeliveryCursor == "" || publication.DeliveryConfirmed {
		t.Fatalf("CompleteSelection() publication = %#v", publication)
	}
	reusedPublication, err := repo.CompleteSelection(context.Background(), result)
	if err != nil ||
		reusedPublication.DeliveryCursor != publication.DeliveryCursor ||
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
		completedRecord.DurablyPersisted {
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
	topic := "courseforge.integration.selection-result." + xrand.RandomNumeric(12)
	trackIntegrationRabbitMQTopology(t, topic)
	connection, err := rabbitmq.NewConnection(integrationRabbitMQConfig)
	if err != nil {
		t.Fatalf("connect selection-result RabbitMQ: %v", err)
	}
	consumer := rabbitmq.NewRabbitMQConsumer(connection)
	t.Cleanup(consumer.Shutdown)
	consumer.RegisterListener(topic, enrollmentasync.NewSelectionResultListener(repo))
	if err := consumer.Start(ctx); err != nil {
		t.Fatalf("start selection-result consumer: %v", err)
	}

	rabbitPublisher, err := rabbitmq.NewRabbitMQPublisher(connection, 1)
	if err != nil {
		t.Fatalf("create selection-result publisher: %v", err)
	}
	publisherConfig := *integrationRabbitMQConfig
	publisherConfig.Topic.SelectionResult = topic
	selectionPublisher := enrollmentasync.NewSelectionResultPublisher(
		repo,
		rabbitmq.NewPublisher(rabbitPublisher, &publisherConfig),
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
	integrationCourseForgeDB.Table("selection_application").
		Where("application_id = ?", applicationID).
		Count(&applicationCount)
	integrationCourseForgeDB.Table("student_course_enrollment").
		Where("application_id = ?", applicationID).
		Count(&enrollmentCount)
	if applicationCount != 1 || enrollmentCount != 1 {
		t.Fatalf(
			"persisted counts = application:%d enrollment:%d, want 1/1",
			applicationCount,
			enrollmentCount,
		)
	}
	applicationRecord, err := repo.QuerySelectionApplication(
		context.Background(), applicationID, studentID,
	)
	if err != nil || applicationRecord == nil ||
		!applicationRecord.DurablyPersisted ||
		applicationRecord.Application.ApplicationID != applicationID {
		t.Fatalf("QuerySelectionApplication() = %#v, %v", applicationRecord, err)
	}
	enrollmentPage, err := repo.ListStudentEnrollments(
		context.Background(), studentID, termID, 20, 0,
	)
	if err != nil || enrollmentPage.Total != 1 || len(enrollmentPage.Items) != 1 ||
		enrollmentPage.Items[0].RoundID != roundID {
		t.Fatalf("ListStudentEnrollments() = %#v, %v", enrollmentPage, err)
	}
	var persistedQuota struct {
		SelectedCredits     string
		SelectedCourseCount uint16
	}
	if err := integrationCourseForgeDB.Table("student_selection_quota").
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
		!persistedRecord.DurablyPersisted {
		t.Fatalf("QuerySelectionByRequest(MySQL) = %#v, %v", persistedRecord, err)
	}
}

// TestEnrollmentRepositoryConcurrentReservationDoesNotOversell 验证100个学生
// 并发抢10个名额时，Redis原子预占严格限制成功数且不会出现负库存。
func TestEnrollmentRepositoryConcurrentReservationDoesNotOversell(t *testing.T) {
	const (
		studentCount = 100
		capacity     = 10

		majorID   = uint64(991001)
		studentID = uint64(991100)
		termID    = uint64(991001)
		courseID  = uint64(991001)
		classID   = uint64(991001)
		roundID   = uint64(991001)
	)

	now := time.Now().Truncate(time.Millisecond)
	seedEnrollmentIntegrationData(
		t,
		now,
		majorID,
		studentID,
		termID,
		courseID,
		classID,
		roundID,
	)
	if err := integrationCourseForgeDB.Table("teaching_class").
		Where("id = ?", classID).
		Update("capacity", capacity).Error; err != nil {
		t.Fatalf("set concurrent test class capacity: %v", err)
	}

	t.Cleanup(func() {
		integrationCourseForgeDB.Exec(
			"DELETE FROM student_selection_quota WHERE student_id > ? AND student_id < ?",
			studentID,
			studentID+studentCount,
		)
		integrationCourseForgeDB.Exec(
			"DELETE FROM student WHERE id > ? AND id < ?",
			studentID,
			studentID+studentCount,
		)
	})
	for index := 1; index < studentCount; index++ {
		currentStudentID := studentID + uint64(index)
		if err := integrationCourseForgeDB.Table("student").Create(map[string]interface{}{
			"id":         currentStudentID,
			"major_id":   majorID,
			"grade_year": 2026,
		}).Error; err != nil {
			t.Fatalf("seed concurrent student %d: %v", currentStudentID, err)
		}
		if err := integrationCourseForgeDB.Table("student_selection_quota").Create(
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
		reservation *application.SelectionReservation
		err         error
	}
	repo := newEnrollmentRepositoryFixture(integrationCourseForgeDB, integrationRedis)
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
				result.reservation.Status != application.ReservationStatusAcquired ||
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

// TestEnrollmentRepositoryDropAndProjectionRepair 使用真实 MySQL/Redis 验证：
// 退课事务写入修复任务，Redis Lua 幂等返还额度与名额，任务最终标记完成。
func TestEnrollmentRepositoryDropAndProjectionRepair(t *testing.T) {
	const (
		majorID       = uint64(992001)
		studentID     = uint64(992001)
		termID        = uint64(992001)
		courseID      = uint64(992001)
		classID       = uint64(992001)
		roundID       = uint64(992001)
		applicationID = "019d0000000000000000000000000201"
		enrollmentID  = "019d0000000000000000000000000202"
	)
	now := time.Now().Truncate(time.Millisecond)
	seedEnrollmentIntegrationData(
		t, now, majorID, studentID, termID, courseID, classID, roundID,
	)
	db := integrationCourseForgeDB
	if err := db.Table("student_selection_quota").
		Where("round_id = ? AND student_id = ?", roundID, studentID).
		Updates(map[string]interface{}{
			"selected_credits":      "3.5",
			"selected_course_count": 1,
		}).Error; err != nil {
		t.Fatalf("seed selected quota: %v", err)
	}
	if err := db.Table("teaching_class").Where("id = ?", classID).
		Update("selected_count", 1).Error; err != nil {
		t.Fatalf("seed selected class count: %v", err)
	}
	if err := db.Table("selection_application").Create(map[string]interface{}{
		"application_id": applicationID, "request_id": "drop-request-1",
		"round_id": roundID, "term_id": termID, "student_id": studentID,
		"course_id": courseID, "teaching_class_id": classID, "credits": "3.5",
		"state": "selected", "source": "web", "applied_at": now,
		"completed_at": now,
	}).Error; err != nil {
		t.Fatalf("seed selection application: %v", err)
	}
	if err := db.Table("student_course_enrollment").Create(map[string]interface{}{
		"enrollment_id": enrollmentID, "application_id": applicationID,
		"term_id": termID, "student_id": studentID, "course_id": courseID,
		"teaching_class_id": classID, "credits": "3.5", "state": "enrolled",
		"active_key":  fmt.Sprintf("%d:%d:%d", termID, studentID, courseID),
		"enrolled_at": now,
	}).Error; err != nil {
		t.Fatalf("seed enrollment: %v", err)
	}

	redisKeys := []string{
		fmt.Sprintf("courseforge:selection:quota:credit:%d:%d", roundID, studentID),
		fmt.Sprintf("courseforge:selection:quota:course:%d:%d", roundID, studentID),
		fmt.Sprintf("courseforge:selection:class:seat:%d", classID),
		fmt.Sprintf("courseforge:selection:course:%d:%d:%d", termID, studentID, courseID),
		fmt.Sprintf("courseforge:selection:dropped:%s", enrollmentID),
	}
	if err := integrationRedisClient.Del(context.Background(), redisKeys...).Err(); err != nil {
		t.Fatalf("cleanup drop Redis: %v", err)
	}
	t.Cleanup(func() { _ = integrationRedisClient.Del(context.Background(), redisKeys...).Err() })
	if err := integrationRedisClient.MSet(context.Background(),
		redisKeys[0], 165,
		redisKeys[1], 5,
		redisKeys[2], 1,
		redisKeys[3], applicationID,
	).Err(); err != nil {
		t.Fatalf("seed drop Redis: %v", err)
	}

	repo := newEnrollmentRepositoryFixture(db, integrationRedis)
	target, err := repo.QueryStudentEnrollment(context.Background(), enrollmentID, studentID)
	if err != nil || target == nil {
		t.Fatalf("QueryStudentEnrollment() = %#v, %v", target, err)
	}
	if _, err := target.Drop(now.Add(time.Minute)); err != nil {
		t.Fatalf("domain Drop() error = %v", err)
	}
	if applied, err := repo.DropEnrollment(context.Background(), target); err != nil || !applied {
		t.Fatalf("DropEnrollment() = %v, %v", applied, err)
	}
	repairs, err := repo.QueryPendingProjectionRepairs(
		context.Background(),
		now.Add(2*time.Minute),
		10,
	)
	if err != nil || len(repairs) != 1 || repairs[0].RepairID != "drop:"+enrollmentID {
		t.Fatalf("QueryPendingProjectionRepairs() = %#v, %v", repairs, err)
	}
	if err := repo.ReleaseDroppedEnrollment(context.Background(), target); err != nil {
		t.Fatalf("ReleaseDroppedEnrollment() error = %v", err)
	}
	// Lua 必须幂等，重复修复不能重复返还资源。
	if err := repo.ReleaseDroppedEnrollment(context.Background(), target); err != nil {
		t.Fatalf("ReleaseDroppedEnrollment(retry) error = %v", err)
	}
	if err := repo.MarkProjectionRepairCompleted(
		context.Background(), repairs[0].RepairID, time.Now(),
	); err != nil {
		t.Fatalf("MarkProjectionRepairCompleted() error = %v", err)
	}
	assertRedisInt(t, redisKeys[0], 200)
	assertRedisInt(t, redisKeys[1], 6)
	assertRedisInt(t, redisKeys[2], 2)
	if exists := integrationRedisClient.Exists(context.Background(), redisKeys[3]).Val(); exists != 0 {
		t.Fatalf("course guard still exists after drop")
	}
}

// TestEnrollmentRepositoryWaitlistLifecycle 验证候补加入、查询、原子抢占和晋级状态迁移。
func TestEnrollmentRepositoryWaitlistLifecycle(t *testing.T) {
	const (
		majorID   = uint64(993001)
		studentID = uint64(993001)
		termID    = uint64(993001)
		courseID  = uint64(993001)
		classID   = uint64(993001)
		roundID   = uint64(993001)
	)
	now := time.Now().Truncate(time.Millisecond)
	seedEnrollmentIntegrationData(
		t, now, majorID, studentID, termID, courseID, classID, roundID,
	)
	if err := integrationCourseForgeDB.Table("teaching_class").
		Where("id = ?", classID).Update("selected_count", 2).Error; err != nil {
		t.Fatalf("fill teaching class: %v", err)
	}
	request := &enrollment.SelectionRequest{
		RequestID: "waitlist-request-1", RoundID: roundID, TermID: termID,
		StudentID: studentID, CourseID: courseID, TeachingClassID: classID,
		Credits: enrollment.Credit(35), Source: enrollment.ApplicationSourceWeb,
	}
	entry, err := enrollment.NewWaitlistEntry(
		"019d0000000000000000000000000301", request, now,
	)
	if err != nil {
		t.Fatalf("NewWaitlistEntry() error = %v", err)
	}
	repo := newEnrollmentRepositoryFixture(integrationCourseForgeDB, integrationRedis)
	joined, err := repo.JoinWaitlist(context.Background(), entry)
	if err != nil || joined.Position == 0 {
		t.Fatalf("JoinWaitlist() = %#v, %v", joined, err)
	}
	reused, err := repo.JoinWaitlist(context.Background(), entry)
	if err != nil || reused.WaitlistID != entry.WaitlistID {
		t.Fatalf("JoinWaitlist(retry) = %#v, %v", reused, err)
	}
	page, err := repo.ListStudentWaitlist(context.Background(), studentID, termID, 20, 0)
	if err != nil || page.Total != 1 || len(page.Items) != 1 {
		t.Fatalf("ListStudentWaitlist() = %#v, %v", page, err)
	}
	// 释放一个 MySQL 名额后，队首候补才可被原子抢占。
	if err := integrationCourseForgeDB.Table("teaching_class").
		Where("id = ?", classID).Update("selected_count", 1).Error; err != nil {
		t.Fatalf("release class seat: %v", err)
	}
	claimed, err := repo.ClaimPromotableEntries(context.Background(), time.Now(), 10)
	if err != nil || len(claimed) != 1 || claimed[0].State != enrollment.WaitlistStatePromoting {
		t.Fatalf("ClaimPromotableEntries() = %#v, %v", claimed, err)
	}
	if err := claimed[0].MarkPromoted(time.Now()); err != nil {
		t.Fatalf("MarkPromoted() domain error = %v", err)
	}
	if err := repo.MarkWaitlistPromoted(context.Background(), claimed[0]); err != nil {
		t.Fatalf("MarkWaitlistPromoted() error = %v", err)
	}
	persisted, err := repo.QueryWaitlist(context.Background(), entry.WaitlistID, studentID)
	if err != nil || persisted.State != enrollment.WaitlistStatePromoted {
		t.Fatalf("QueryWaitlist() = %#v, %v", persisted, err)
	}
}

// TestEnrollmentRepositoryLoadsEligibilitySnapshot 验证基础设施层能一次性装配
// 学生状态、年级、专业、先修成绩和课表，并交给纯领域策略判断。
func TestEnrollmentRepositoryLoadsEligibilitySnapshot(t *testing.T) {
	const (
		majorID              = uint64(994001)
		studentID            = uint64(994001)
		termID               = uint64(994001)
		courseID             = uint64(994001)
		prerequisiteCourseID = uint64(994002)
		classID              = uint64(994001)
		roundID              = uint64(994001)
	)
	now := time.Now().Truncate(time.Millisecond)
	seedEnrollmentIntegrationData(
		t, now, majorID, studentID, termID, courseID, classID, roundID,
	)
	db := integrationCourseForgeDB
	t.Cleanup(func() {
		db.Exec("DELETE FROM student_course_history WHERE student_id = ?", studentID)
		db.Exec("DELETE FROM course_prerequisite WHERE course_id = ?", courseID)
		db.Exec("DELETE FROM teaching_class_schedule WHERE teaching_class_id = ?", classID)
		db.Exec("DELETE FROM teaching_class_major_scope WHERE teaching_class_id = ?", classID)
		db.Exec("DELETE FROM course WHERE id = ?", prerequisiteCourseID)
	})
	if err := db.Table("course").Create(map[string]interface{}{
		"id": prerequisiteCourseID, "course_code": "IT-C-PRE",
		"course_name": "先修课程", "credits": "2.0",
	}).Error; err != nil {
		t.Fatalf("seed prerequisite course: %v", err)
	}
	if err := db.Table("course_prerequisite").Create(map[string]interface{}{
		"course_id": courseID, "prerequisite_course_id": prerequisiteCourseID,
		"minimum_score": "60.0",
	}).Error; err != nil {
		t.Fatalf("seed prerequisite: %v", err)
	}
	if err := db.Table("student_course_history").Create(map[string]interface{}{
		"student_id": studentID, "course_id": prerequisiteCourseID,
		"term_id": termID, "score": "85.0", "result": "passed", "completed_at": now,
	}).Error; err != nil {
		t.Fatalf("seed course history: %v", err)
	}
	if err := db.Table("teaching_class_major_scope").Create(map[string]interface{}{
		"teaching_class_id": classID, "major_id": majorID, "scope_type": "allow",
	}).Error; err != nil {
		t.Fatalf("seed major scope: %v", err)
	}
	if err := db.Table("teaching_class_schedule").Create(map[string]interface{}{
		"teaching_class_id": classID, "day_of_week": 1,
		"start_week": 1, "end_week": 16, "start_section": 1, "end_section": 2,
	}).Error; err != nil {
		t.Fatalf("seed schedule: %v", err)
	}
	minimumGrade := uint16(2025)
	maximumGrade := uint16(2027)
	if err := db.Table("teaching_class").Where("id = ?", classID).Updates(map[string]interface{}{
		"minimum_grade_year": minimumGrade,
		"maximum_grade_year": maximumGrade,
	}).Error; err != nil {
		t.Fatalf("seed grade scope: %v", err)
	}

	repo := newEnrollmentRepositoryFixture(db, integrationRedis)
	snapshot, err := repo.QueryEligibilitySnapshot(
		context.Background(), studentID, termID, courseID, classID,
	)
	if err != nil {
		t.Fatalf("QueryEligibilitySnapshot() error = %v", err)
	}
	if len(snapshot.Prerequisites) != 1 || len(snapshot.Achievements) != 1 ||
		len(snapshot.MajorScopes) != 1 || len(snapshot.TargetSchedules) != 1 {
		t.Fatalf("eligibility snapshot = %#v", snapshot)
	}
	if err := (enrollment.EligibilityPolicy{}).Evaluate(snapshot); err != nil {
		t.Fatalf("EligibilityPolicy.Evaluate() error = %v", err)
	}
	snapshot.Achievements = nil
	if err := (enrollment.EligibilityPolicy{}).Evaluate(snapshot); !errors.Is(err, enrollment.ErrPrerequisiteNotMet) {
		t.Fatalf("missing prerequisite error = %v", err)
	}
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
		err := integrationCourseForgeDB.Table("selection_application").
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
	majorID, studentID, termID, courseID, classID, roundID uint64,
) {
	t.Helper()
	db := integrationCourseForgeDB
	rows := []struct {
		table string
		value map[string]interface{}
	}{
		{"student", map[string]interface{}{
			"id": studentID, "major_id": majorID, "grade_year": 2026,
		}},
		{"course", map[string]interface{}{
			"id": courseID, "course_code": "IT-C-001", "course_name": "分布式系统",
			"credits": "3.5",
		}},
		{"teaching_class", map[string]interface{}{
			"id": classID, "class_code": "IT-CL-001", "term_id": termID,
			"course_id": courseID, "capacity": 2,
			"selected_count": 0, "state": "open",
		}},
		{"selection_round", map[string]interface{}{
			"id": roundID, "term_id": termID, "round_code": "IT-R-001",
			"round_name": "集成测试轮次", "start_time": now.Add(-time.Hour),
			"end_time": now.Add(time.Hour), "state": "open",
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
		db.Exec(
			`DELETE epr FROM enrollment_projection_repair epr
			 JOIN student_course_enrollment sce ON sce.enrollment_id = epr.enrollment_id
			 WHERE sce.student_id = ?`,
			studentID,
		)
		db.Exec("DELETE FROM selection_waitlist WHERE student_id = ?", studentID)
		db.Exec("DELETE FROM student_course_history WHERE student_id = ?", studentID)
		db.Exec("DELETE FROM teaching_class_schedule WHERE teaching_class_id = ?", classID)
		db.Exec("DELETE FROM teaching_class_major_scope WHERE teaching_class_id = ?", classID)
		db.Exec("DELETE FROM course_prerequisite WHERE course_id = ?", courseID)
		db.Exec("DELETE FROM selection_event WHERE student_id = ?", studentID)
		db.Exec("DELETE FROM student_course_enrollment WHERE student_id = ?", studentID)
		db.Exec("DELETE FROM selection_application WHERE student_id = ?", studentID)
		db.Exec("DELETE FROM student_selection_quota WHERE student_id = ?", studentID)
		db.Exec("DELETE FROM selection_round_class WHERE round_id = ?", roundID)
		db.Exec("DELETE FROM selection_round WHERE id = ?", roundID)
		db.Exec("DELETE FROM teaching_class WHERE id = ?", classID)
		db.Exec("DELETE FROM course WHERE id = ?", courseID)
		db.Exec("DELETE FROM student WHERE id = ?", studentID)
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
