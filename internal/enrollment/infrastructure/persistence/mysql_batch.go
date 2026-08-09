package enrollmentrepo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
	enrollmentintegration "github.com/yywencs/courseforge/internal/enrollment/integration"
	"github.com/yywencs/courseforge/internal/platform/outbox"
	outboxrepo "github.com/yywencs/courseforge/internal/platform/outbox/mysql"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type selectionEventBatchRow struct {
	EventID        string    `gorm:"column:event_id"`
	ApplicationID  string    `gorm:"column:application_id"`
	StudentID      uint64    `gorm:"column:student_id"`
	EventType      string    `gorm:"column:event_type"`
	EventPayload   string    `gorm:"column:event_payload"`
	OccurredAt     time.Time `gorm:"column:occurred_at"`
	ConsumeBatchID string    `gorm:"column:consume_batch_id"`
	CreateTime     time.Time `gorm:"column:create_time"`
}

type selectionApplicationBatchRow struct {
	ApplicationID   string    `gorm:"column:application_id"`
	RequestID       string    `gorm:"column:request_id"`
	RoundID         uint64    `gorm:"column:round_id"`
	TermID          uint64    `gorm:"column:term_id"`
	StudentID       uint64    `gorm:"column:student_id"`
	CourseID        uint64    `gorm:"column:course_id"`
	TeachingClassID uint64    `gorm:"column:teaching_class_id"`
	Credits         string    `gorm:"column:credits"`
	State           string    `gorm:"column:state"`
	FailureCode     string    `gorm:"column:failure_code"`
	FailureMessage  string    `gorm:"column:failure_message"`
	AppliedAt       time.Time `gorm:"column:applied_at"`
	CompletedAt     time.Time `gorm:"column:completed_at"`
	Source          string    `gorm:"column:source"`
	CreateTime      time.Time `gorm:"column:create_time"`
	UpdateTime      time.Time `gorm:"column:update_time"`
}

type enrollmentBatchRow struct {
	EnrollmentID    string    `gorm:"column:enrollment_id"`
	ApplicationID   string    `gorm:"column:application_id"`
	TermID          uint64    `gorm:"column:term_id"`
	StudentID       uint64    `gorm:"column:student_id"`
	CourseID        uint64    `gorm:"column:course_id"`
	TeachingClassID uint64    `gorm:"column:teaching_class_id"`
	Credits         string    `gorm:"column:credits"`
	State           string    `gorm:"column:state"`
	ActiveKey       string    `gorm:"column:active_key"`
	EnrolledAt      time.Time `gorm:"column:enrolled_at"`
	CreateTime      time.Time `gorm:"column:create_time"`
	UpdateTime      time.Time `gorm:"column:update_time"`
}

type enrollmentCountDeltaBatchRow struct {
	EventID         string    `gorm:"column:event_id"`
	TeachingClassID uint64    `gorm:"column:teaching_class_id"`
	Delta           int8      `gorm:"column:delta"`
	CreateTime      time.Time `gorm:"column:create_time"`
}

// SaveSelectionResults 将一批 Redis 已完成结果在同一事务中幂等写入 MySQL。
// selection_event.consume_batch_id 是事务内的原子消费凭据：只有本批首次插入的
// event_id 才会继续修改额度及业务表；重投消息仅校验既有事件并清理 Redis。
func (r *ResultStore) SaveSelectionResults(
	ctx context.Context,
	results []*enrollment.SelectionResult,
) error {
	uniqueResults, err := normalizeSelectionResults(results)
	if err != nil {
		return err
	}
	if len(uniqueResults) == 0 {
		return nil
	}
	batchID, err := r.ids.NewID()
	if err != nil {
		return fmt.Errorf("生成选课消费批次ID: %w", err)
	}

	txnErr := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		claimed, claimErr := claimSelectionEvents(tx, batchID, uniqueResults)
		if claimErr != nil {
			return claimErr
		}
		if err := validateUnclaimedSelectionEvents(tx, uniqueResults, claimed); err != nil {
			return err
		}
		if len(claimed) == 0 {
			return nil
		}
		return r.persistClaimedSelectionResults(tx, claimed)
	})
	if txnErr != nil {
		return txnErr
	}

	// MySQL 已提交后再批量清理 pending。若清理失败，Stream 重投会命中
	// selection_event 的幂等占位，不会再次更新额度或插入选课事实。
	if err := r.clearPersistedSelections(ctx, results); err != nil {
		return fmt.Errorf("批量清理Redis选课pending: %w", err)
	}
	return nil
}

func normalizeSelectionResults(
	results []*enrollment.SelectionResult,
) ([]*enrollment.SelectionResult, error) {
	unique := make([]*enrollment.SelectionResult, 0, len(results))
	seen := make(map[string]*enrollment.SelectionResult, len(results))
	for _, result := range results {
		if result == nil {
			return nil, enrollment.ErrInvalidParams
		}
		if err := result.Validate(); err != nil {
			return nil, err
		}
		eventID := selectionResultEventID(result)
		if existing := seen[eventID]; existing != nil {
			if !sameCompleteSelectionResult(existing, result) {
				return nil, fmt.Errorf(
					"%w: event_id=%s",
					enrollment.ErrIdempotencyConflict,
					eventID,
				)
			}
			continue
		}
		seen[eventID] = result
		unique = append(unique, result)
	}
	return unique, nil
}

func claimSelectionEvents(
	tx *gorm.DB,
	batchID string,
	results []*enrollment.SelectionResult,
) ([]*enrollment.SelectionResult, error) {
	now := time.Now()
	rows := make([]selectionEventBatchRow, 0, len(results))
	byEventID := make(map[string]*enrollment.SelectionResult, len(results))
	for _, result := range results {
		payload, err := json.Marshal(newSelectionResultPayload(result))
		if err != nil {
			return nil, fmt.Errorf("序列化选课审计事件: %w", err)
		}
		eventID := selectionResultEventID(result)
		byEventID[eventID] = result
		rows = append(rows, selectionEventBatchRow{
			EventID:        eventID,
			ApplicationID:  result.ApplicationID,
			StudentID:      result.StudentID,
			EventType:      string(result.State),
			EventPayload:   string(payload),
			OccurredAt:     result.CompletedAt,
			ConsumeBatchID: batchID,
			CreateTime:     now,
		})
	}
	if err := tx.Table("selection_event").
		Clauses(clause.OnConflict{DoNothing: true}).
		CreateInBatches(rows, len(rows)).Error; err != nil {
		return nil, err
	}

	var eventIDs []string
	if err := tx.Table("selection_event").
		Where("consume_batch_id = ?", batchID).
		Pluck("event_id", &eventIDs).Error; err != nil {
		return nil, err
	}
	claimed := make([]*enrollment.SelectionResult, 0, len(eventIDs))
	for _, eventID := range eventIDs {
		if result := byEventID[eventID]; result != nil {
			claimed = append(claimed, result)
		}
	}
	return claimed, nil
}

func validateUnclaimedSelectionEvents(
	tx *gorm.DB,
	all []*enrollment.SelectionResult,
	claimed []*enrollment.SelectionResult,
) error {
	claimedIDs := make(map[string]struct{}, len(claimed))
	for _, result := range claimed {
		claimedIDs[selectionResultEventID(result)] = struct{}{}
	}
	unclaimed := make([]*enrollment.SelectionResult, 0, len(all)-len(claimed))
	for _, result := range all {
		if _, ok := claimedIDs[selectionResultEventID(result)]; !ok {
			unclaimed = append(unclaimed, result)
		}
	}
	return validateExistingSelectionEvents(tx, unclaimed)
}

func validateExistingSelectionEvents(
	tx *gorm.DB,
	results []*enrollment.SelectionResult,
) error {
	if len(results) == 0 {
		return nil
	}
	expected := make(map[string]*enrollment.SelectionResult, len(results))
	eventIDs := make([]string, 0, len(results))
	for _, result := range results {
		eventID := selectionResultEventID(result)
		expected[eventID] = result
		eventIDs = append(eventIDs, eventID)
	}
	var rows []selectionEventBatchRow
	if err := tx.Table("selection_event").
		Select("event_id", "event_payload").
		Where("event_id IN ?", eventIDs).
		Find(&rows).Error; err != nil {
		return err
	}
	if len(rows) != len(expected) {
		return enrollment.ErrRecordNotFound
	}
	for _, row := range rows {
		var payload selectionResultPayload
		if err := json.Unmarshal([]byte(row.EventPayload), &payload); err != nil {
			return fmt.Errorf("解析已落库选课事件: %w", err)
		}
		if !sameCompleteSelectionResult(payload.toDomain(), expected[row.EventID]) {
			return fmt.Errorf(
				"%w: event_id=%s",
				enrollment.ErrIdempotencyConflict,
				row.EventID,
			)
		}
	}
	return nil
}

func (r *ResultStore) persistClaimedSelectionResults(
	tx *gorm.DB,
	results []*enrollment.SelectionResult,
) error {
	selected := make([]*enrollment.SelectionResult, 0, len(results))
	for _, result := range results {
		if result.State == enrollment.ApplicationStateSelected {
			selected = append(selected, result)
		}
	}
	if err := persistSelectedQuotasBatch(tx, selected); err != nil {
		return err
	}

	now := time.Now()
	applications := make([]selectionApplicationBatchRow, 0, len(results))
	enrollments := make([]enrollmentBatchRow, 0, len(selected))
	deltas := make([]enrollmentCountDeltaBatchRow, 0, len(selected))
	for _, result := range results {
		failureCode, failureMessage := "", ""
		if result.Failure != nil {
			failureCode = string(result.Failure.Code)
			failureMessage = result.Failure.Message
		}
		applications = append(applications, selectionApplicationBatchRow{
			ApplicationID:   result.ApplicationID,
			RequestID:       result.RequestID,
			RoundID:         result.RoundID,
			TermID:          result.TermID,
			StudentID:       result.StudentID,
			CourseID:        result.CourseID,
			TeachingClassID: result.TeachingClassID,
			Credits:         creditToDecimal(result.Credits),
			State:           string(result.State),
			FailureCode:     failureCode,
			FailureMessage:  failureMessage,
			AppliedAt:       result.AppliedAt,
			CompletedAt:     result.CompletedAt,
			Source:          string(result.Source),
			CreateTime:      now,
			UpdateTime:      now,
		})
		if result.State != enrollment.ApplicationStateSelected {
			continue
		}

		enrollmentID, err := r.ids.NewID()
		if err != nil {
			return fmt.Errorf("生成正式选课记录ID: %w", err)
		}
		eventID := selectionResultEventID(result)
		enrollments = append(enrollments, enrollmentBatchRow{
			EnrollmentID:    enrollmentID,
			ApplicationID:   result.ApplicationID,
			TermID:          result.TermID,
			StudentID:       result.StudentID,
			CourseID:        result.CourseID,
			TeachingClassID: result.TeachingClassID,
			Credits:         creditToDecimal(result.Credits),
			State:           "enrolled",
			ActiveKey: fmt.Sprintf(
				"%d:%d:%d",
				result.TermID,
				result.StudentID,
				result.CourseID,
			),
			EnrolledAt: result.CompletedAt,
			CreateTime: now,
			UpdateTime: now,
		})
		deltas = append(deltas, enrollmentCountDeltaBatchRow{
			EventID:         eventID,
			TeachingClassID: result.TeachingClassID,
			Delta:           1,
			CreateTime:      result.CompletedAt,
		})
	}

	if err := tx.Table("selection_application").
		CreateInBatches(applications, len(applications)).Error; err != nil {
		return err
	}
	if len(enrollments) > 0 {
		if err := tx.Table("student_course_enrollment").
			CreateInBatches(enrollments, len(enrollments)).Error; err != nil {
			return err
		}
		if err := tx.Table("enrollment_count_delta").
			CreateInBatches(deltas, len(deltas)).Error; err != nil {
			return err
		}
	}
	events, err := newSelectionNotificationEvents(results)
	if err != nil {
		return err
	}
	return outboxrepo.NewRepository(tx).AppendBatch(tx.Statement.Context, events)
}

func newSelectionNotificationEvents(
	results []*enrollment.SelectionResult,
) ([]*outbox.NewEvent, error) {
	events := make([]*outbox.NewEvent, 0, len(results))
	for _, result := range results {
		payload := enrollmentintegration.NewSelectionNotification(result)
		if err := payload.Validate(); err != nil {
			return nil, err
		}
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("序列化选课通知事件: %w", err)
		}
		events = append(events, &outbox.NewEvent{
			EventID:       selectionResultEventID(result),
			AggregateType: enrollmentintegration.SelectionNotificationAggregate,
			AggregateID:   result.ApplicationID,
			Topic:         enrollmentintegration.SelectionNotificationTopic,
			EventType:     enrollmentintegration.SelectionResultPersisted,
			Payload:       encoded,
		})
	}
	return events, nil
}

type quotaBatchKey struct {
	RoundID   uint64
	StudentID uint64
}

type quotaBatchDelta struct {
	Credits enrollment.Credit
	Courses uint16
}

func persistSelectedQuotasBatch(tx *gorm.DB, results []*enrollment.SelectionResult) error {
	if len(results) == 0 {
		return nil
	}
	deltas := make(map[quotaBatchKey]quotaBatchDelta, len(results))
	for _, result := range results {
		key := quotaBatchKey{RoundID: result.RoundID, StudentID: result.StudentID}
		delta := deltas[key]
		delta.Credits += result.Credits
		delta.Courses++
		deltas[key] = delta
	}

	var query strings.Builder
	args := make([]interface{}, 0, len(deltas)*4)
	query.WriteString("UPDATE student_selection_quota AS quota JOIN (")
	i := 0
	for key, delta := range deltas {
		if i > 0 {
			query.WriteString(" UNION ALL ")
		}
		query.WriteString(
			"SELECT ? AS round_id, ? AS student_id, " +
				"CAST(? AS DECIMAL(5,1)) AS credits, ? AS courses",
		)
		args = append(
			args,
			key.RoundID,
			key.StudentID,
			creditToDecimal(delta.Credits),
			delta.Courses,
		)
		i++
	}
	query.WriteString(`) AS input
		ON quota.round_id = input.round_id AND quota.student_id = input.student_id
		SET quota.selected_credits = quota.selected_credits + input.credits,
			quota.selected_course_count = quota.selected_course_count + input.courses,
			quota.update_time = NOW(3)
		WHERE quota.selected_credits + input.credits <= quota.credit_limit
			AND quota.selected_course_count + input.courses <= quota.course_limit`)
	updated := tx.Exec(query.String(), args...)
	if updated.Error != nil {
		return updated.Error
	}
	if updated.RowsAffected != int64(len(deltas)) {
		return enrollment.ErrCreditQuotaExceeded
	}
	return nil
}

func selectionResultEventID(result *enrollment.SelectionResult) string {
	return fmt.Sprintf("selection:%d:%s", result.StudentID, result.ApplicationID)
}

func sameCompleteSelectionResult(actual, expected *enrollment.SelectionResult) bool {
	if actual == nil || expected == nil || actual.ApplicationID != expected.ApplicationID ||
		actual.State != expected.State || !actual.AppliedAt.Equal(expected.AppliedAt) ||
		!actual.CompletedAt.Equal(expected.CompletedAt) ||
		!sameSelectionFingerprint(actual, expected) {
		return false
	}
	if actual.Failure == nil || expected.Failure == nil {
		return actual.Failure == nil && expected.Failure == nil
	}
	return actual.Failure.Code == expected.Failure.Code &&
		actual.Failure.Message == expected.Failure.Message
}
