package enrollmentrepo

import (
	"context"
	"errors"
	"time"

	"prizeforge/internal/domain/enrollment"

	"gorm.io/gorm"
)

type projectionRepairRow struct {
	RepairID        string
	RetryCount      uint32
	NextRetryAt     timeValue
	LastError       string
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

// QueryPendingProjectionRepairs 从 MySQL 可靠任务表读取到期修复任务。
func (r *Repository) QueryPendingProjectionRepairs(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]*enrollment.ProjectionRepair, error) {
	if now.IsZero() || limit <= 0 || limit > 500 {
		return nil, enrollment.ErrInvalidParams
	}
	var rows []projectionRepairRow
	err := r.db.WithContext(ctx).
		Table("enrollment_projection_repair AS epr").
		Select(`
			epr.repair_id,
			epr.retry_count,
			epr.next_retry_at,
			epr.last_error,
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
		Joins("JOIN student_course_enrollment sce ON sce.enrollment_id = epr.enrollment_id").
		Joins("JOIN selection_application sa ON sa.application_id = sce.application_id").
		Where("epr.state = 'pending' AND epr.next_retry_at <= ?", now).
		Order("epr.id ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	repairs := make([]*enrollment.ProjectionRepair, 0, len(rows))
	for index := range rows {
		target, err := (&enrollmentRow{
			EnrollmentID:    rows[index].EnrollmentID,
			ApplicationID:   rows[index].ApplicationID,
			RoundID:         rows[index].RoundID,
			TermID:          rows[index].TermID,
			StudentID:       rows[index].StudentID,
			CourseID:        rows[index].CourseID,
			TeachingClassID: rows[index].TeachingClassID,
			Credits:         rows[index].Credits,
			State:           rows[index].State,
			EnrolledAt:      rows[index].EnrolledAt,
			DroppedAt:       rows[index].DroppedAt,
		}).toEntity()
		if err != nil {
			return nil, err
		}
		repairs = append(repairs, &enrollment.ProjectionRepair{
			RepairID:    rows[index].RepairID,
			Enrollment:  target,
			RetryCount:  rows[index].RetryCount,
			NextRetryAt: rows[index].NextRetryAt.Time,
			LastError:   rows[index].LastError,
		})
	}
	return repairs, nil
}

func (r *Repository) MarkProjectionRepairCompleted(
	ctx context.Context,
	repairID string,
	completedAt time.Time,
) error {
	if repairID == "" || completedAt.IsZero() {
		return enrollment.ErrInvalidParams
	}
	return r.db.WithContext(ctx).Table("enrollment_projection_repair").
		Where("repair_id = ? AND state = 'pending'", repairID).
		Updates(map[string]interface{}{
			"state":        "completed",
			"completed_at": completedAt,
			"last_error":   "",
			"update_time":  completedAt,
		}).Error
}

func (r *Repository) MarkProjectionRepairFailed(
	ctx context.Context,
	repairID string,
	retryAt time.Time,
	lastError string,
) error {
	if repairID == "" || retryAt.IsZero() {
		return enrollment.ErrInvalidParams
	}
	if len(lastError) > 512 {
		lastError = lastError[:512]
	}
	return r.db.WithContext(ctx).Table("enrollment_projection_repair").
		Where("repair_id = ? AND state = 'pending'", repairID).
		Updates(map[string]interface{}{
			"retry_count":   gorm.Expr("retry_count + 1"),
			"next_retry_at": retryAt,
			"last_error":    lastError,
			"update_time":   time.Now(),
		}).Error
}

func (r *Repository) CountPendingProjectionRepairs(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("enrollment_projection_repair").
		Where("state = 'pending'").
		Count(&count).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	return count, err
}
