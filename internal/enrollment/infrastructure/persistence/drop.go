package enrollmentrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type enrollmentRow struct {
	EnrollmentID    string
	ApplicationID   string
	RoundID         uint64
	TermID          uint64
	StudentID       uint64
	CourseID        uint64
	TeachingClassID uint64
	Credits         string
	State           string
	EnrolledAt      timeValue
	DroppedAt       *time.Time
}

func (r *EnrollmentStore) QueryStudentEnrollment(
	ctx context.Context,
	enrollmentID string,
	studentID uint64,
) (*enrollment.StudentEnrollment, error) {
	if enrollmentID == "" || studentID == 0 {
		return nil, enrollment.ErrInvalidParams
	}
	var row enrollmentRow
	err := enrollmentRowQuery(r.db.WithContext(ctx)).
		Where("sce.enrollment_id = ? AND sce.student_id = ?", enrollmentID, studentID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row.toEntity()
}

func (r *EnrollmentStore) DropEnrollment(
	ctx context.Context,
	target *enrollment.StudentEnrollment,
) (bool, error) {
	if target == nil || target.State != enrollment.EnrollmentStateDropped ||
		target.DroppedAt == nil {
		return false, enrollment.ErrInvalidParams
	}
	applied := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var current enrollmentRow
		err := enrollmentRowQuery(tx).
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Where(
				"sce.enrollment_id = ? AND sce.student_id = ?",
				target.EnrollmentID,
				target.StudentID,
			).
			Take(&current).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return enrollment.ErrRecordNotFound
		}
		if err != nil {
			return err
		}
		if current.State == string(enrollment.EnrollmentStateDropped) {
			return nil
		}
		if current.State != string(enrollment.EnrollmentStateEnrolled) {
			return enrollment.ErrInvalidEnrollmentState
		}
		currentEntity, err := current.toEntity()
		if err != nil {
			return err
		}
		if !sameEnrollmentIdentity(currentEntity, target) {
			return enrollment.ErrIdempotencyConflict
		}

		now := *target.DroppedAt
		update := tx.Table("student_course_enrollment").
			Where("enrollment_id = ? AND student_id = ? AND state = ?",
				target.EnrollmentID,
				target.StudentID,
				string(enrollment.EnrollmentStateEnrolled),
			).
			Updates(map[string]interface{}{
				"state":       string(enrollment.EnrollmentStateDropped),
				"active_key":  nil,
				"dropped_at":  now,
				"update_time": now,
			})
		if update.Error != nil {
			return update.Error
		}
		if update.RowsAffected != 1 {
			return enrollment.ErrInvalidEnrollmentState
		}
		credit := creditToDecimal(target.Credits)
		quota := tx.Table("student_selection_quota").
			Where(
				`round_id = ? AND student_id = ?
				 AND selected_credits >= ? AND selected_course_count > 0`,
				target.RoundID,
				target.StudentID,
				credit,
			).
			Updates(map[string]interface{}{
				"selected_credits":      gorm.Expr("selected_credits - ?", credit),
				"selected_course_count": gorm.Expr("selected_course_count - 1"),
				"update_time":           now,
			})
		if quota.Error != nil {
			return quota.Error
		}
		if quota.RowsAffected != 1 {
			return enrollment.ErrInvalidEnrollmentState
		}
		if err := appendEnrollmentCountDelta(
			tx,
			"drop:"+target.EnrollmentID,
			target.TeachingClassID,
			-1,
			now,
		); err != nil {
			return err
		}
		eventPayload, err := json.Marshal(newDroppedEnrollmentEventPayload(target))
		if err != nil {
			return fmt.Errorf("序列化退课事件: %w", err)
		}
		if err := tx.Table("selection_event").Create(map[string]interface{}{
			"event_id":       "drop:" + target.EnrollmentID,
			"application_id": target.ApplicationID,
			"student_id":     target.StudentID,
			"event_type":     "dropped",
			"event_payload":  string(eventPayload),
			"occurred_at":    now,
			"create_time":    now,
		}).Error; err != nil {
			return err
		}
		if err := tx.Table("enrollment_projection_repair").Create(map[string]interface{}{
			"repair_id":     "drop:" + target.EnrollmentID,
			"enrollment_id": target.EnrollmentID,
			"operation":     "release_dropped",
			"state":         "pending",
			"next_retry_at": now,
			"create_time":   now,
			"update_time":   now,
		}).Error; err != nil {
			return err
		}
		applied = true
		return nil
	})
	return applied, err
}

func enrollmentRowQuery(db *gorm.DB) *gorm.DB {
	return db.Table("student_course_enrollment AS sce").
		Select(`
			sce.enrollment_id,
			sce.application_id,
			sa.round_id,
			sce.term_id,
			sce.student_id,
			sce.course_id,
			sce.teaching_class_id,
			CAST(sce.credits AS CHAR) AS credits,
			sce.state,
			sce.enrolled_at,
			sce.dropped_at
		`).
		Joins("JOIN selection_application sa ON sa.application_id = sce.application_id")
}

func (row *enrollmentRow) toEntity() (*enrollment.StudentEnrollment, error) {
	if row == nil {
		return nil, enrollment.ErrRecordNotFound
	}
	credits, err := creditFromDecimal(row.Credits)
	if err != nil {
		return nil, err
	}
	entity := &enrollment.StudentEnrollment{
		EnrollmentID:    row.EnrollmentID,
		ApplicationID:   row.ApplicationID,
		RoundID:         row.RoundID,
		TermID:          row.TermID,
		StudentID:       row.StudentID,
		CourseID:        row.CourseID,
		TeachingClassID: row.TeachingClassID,
		Credits:         credits,
		State:           enrollment.EnrollmentState(row.State),
		EnrolledAt:      row.EnrolledAt.Time,
		DroppedAt:       row.DroppedAt,
	}
	if err := entity.Validate(); err != nil {
		return nil, err
	}
	return entity, nil
}

func sameEnrollmentIdentity(left, right *enrollment.StudentEnrollment) bool {
	return left != nil && right != nil &&
		left.EnrollmentID == right.EnrollmentID &&
		left.ApplicationID == right.ApplicationID &&
		left.RoundID == right.RoundID &&
		left.TermID == right.TermID &&
		left.StudentID == right.StudentID &&
		left.CourseID == right.CourseID &&
		left.TeachingClassID == right.TeachingClassID &&
		left.Credits == right.Credits
}
