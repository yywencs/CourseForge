package enrollmentrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"prizeforge/internal/domain/enrollment"

	"gorm.io/gorm"
)

type persistedApplication struct {
	ApplicationID   string
	RequestID       string
	RoundID         uint64
	TermID          uint64
	StudentID       uint64
	CourseID        uint64
	TeachingClassID uint64
	State           string
}

// SaveSelectionResult 将 Redis 已完成的选课结果幂等写入 MySQL。
// Redis 已经完成热路径预占，MySQL 仍通过条件更新再次守住额度和容量下限。
func (r *Repository) SaveSelectionResult(
	ctx context.Context,
	result *enrollment.SelectionResult,
) error {
	if err := result.Validate(); err != nil {
		return err
	}

	txnErr := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing persistedApplication
		err := tx.Table("selection_application").
			Select(`
				application_id,
				request_id,
				round_id,
				term_id,
				student_id,
				course_id,
				teaching_class_id,
				state
			`).
			Where("application_id = ?", result.ApplicationID).
			Take(&existing).Error
		if err == nil {
			return validatePersistedApplication(&existing, result)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		if result.State == enrollment.ApplicationStateSelected {
			if err := persistSelectedResources(tx, result); err != nil {
				return err
			}
		}
		if err := createSelectionApplication(tx, result); err != nil {
			return err
		}
		if result.State == enrollment.ApplicationStateSelected {
			enrollmentID, err := r.newID()
			if err != nil {
				return fmt.Errorf("生成正式选课记录ID: %w", err)
			}
			if err := tx.Table("student_course_enrollment").Create(map[string]interface{}{
				"enrollment_id":     enrollmentID,
				"application_id":    result.ApplicationID,
				"term_id":           result.TermID,
				"student_id":        result.StudentID,
				"course_id":         result.CourseID,
				"teaching_class_id": result.TeachingClassID,
				"credits":           creditToDecimal(result.Credits),
				"state":             "enrolled",
				"active_key": fmt.Sprintf(
					"%d:%d:%d",
					result.TermID,
					result.StudentID,
					result.CourseID,
				),
				"enrolled_at": result.CompletedAt,
				"create_time": time.Now(),
				"update_time": time.Now(),
			}).Error; err != nil {
				return err
			}
		}

		eventPayload, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("序列化选课审计事件: %w", err)
		}
		eventID := fmt.Sprintf("selection:%d:%s", result.StudentID, result.ApplicationID)
		if err := tx.Table("selection_event").Create(map[string]interface{}{
			"event_id":       eventID,
			"application_id": result.ApplicationID,
			"student_id":     result.StudentID,
			"event_type":     string(result.State),
			"event_payload":  string(eventPayload),
			"occurred_at":    result.CompletedAt,
			"create_time":    time.Now(),
		}).Error; err != nil {
			return err
		}
		return nil
	})
	if txnErr != nil {
		return txnErr
	}

	// MySQL 已提交后再清理 pending。清理失败时让 RabbitMQ 重投，
	// 下次消费会命中上面的幂等分支，不会重复扣减。
	if err := r.clearPersistedSelection(ctx, result); err != nil {
		return fmt.Errorf("清理Redis选课pending: %w", err)
	}
	return nil
}

func persistSelectedResources(tx *gorm.DB, result *enrollment.SelectionResult) error {
	credit := creditToDecimal(result.Credits)
	quotaUpdate := tx.Table("student_selection_quota").
		Where(
			`round_id = ? AND student_id = ?
			 AND selected_credits + ? <= credit_limit
			 AND selected_course_count < course_limit`,
			result.RoundID,
			result.StudentID,
			credit,
		).
		Updates(map[string]interface{}{
			"selected_credits":      gorm.Expr("selected_credits + ?", credit),
			"selected_course_count": gorm.Expr("selected_course_count + 1"),
			"update_time":           time.Now(),
		})
	if quotaUpdate.Error != nil {
		return quotaUpdate.Error
	}
	if quotaUpdate.RowsAffected != 1 {
		return enrollment.ErrCreditQuotaExceeded
	}

	classUpdate := tx.Table("teaching_class").
		Where(
			"id = ? AND term_id = ? AND course_id = ? AND selected_count < capacity",
			result.TeachingClassID,
			result.TermID,
			result.CourseID,
		).
		Updates(map[string]interface{}{
			"selected_count": gorm.Expr("selected_count + 1"),
			"update_time":    time.Now(),
		})
	if classUpdate.Error != nil {
		return classUpdate.Error
	}
	if classUpdate.RowsAffected != 1 {
		return enrollment.ErrTeachingClassFull
	}
	return nil
}

func createSelectionApplication(tx *gorm.DB, result *enrollment.SelectionResult) error {
	failureCode := ""
	failureMessage := ""
	if result.Failure != nil {
		failureCode = string(result.Failure.Code)
		failureMessage = result.Failure.Message
	}
	return tx.Table("selection_application").Create(map[string]interface{}{
		"application_id":    result.ApplicationID,
		"request_id":        result.RequestID,
		"round_id":          result.RoundID,
		"term_id":           result.TermID,
		"student_id":        result.StudentID,
		"course_id":         result.CourseID,
		"teaching_class_id": result.TeachingClassID,
		"credits":           creditToDecimal(result.Credits),
		"state":             string(result.State),
		"failure_code":      failureCode,
		"failure_message":   failureMessage,
		"applied_at":        result.AppliedAt,
		"completed_at":      result.CompletedAt,
		"source":            string(result.Source),
		"create_time":       time.Now(),
		"update_time":       time.Now(),
	}).Error
}

func validatePersistedApplication(
	existing *persistedApplication,
	result *enrollment.SelectionResult,
) error {
	if existing == nil {
		return enrollment.ErrRecordNotFound
	}
	if existing.ApplicationID != result.ApplicationID ||
		existing.RequestID != result.RequestID ||
		existing.RoundID != result.RoundID ||
		existing.TermID != result.TermID ||
		existing.StudentID != result.StudentID ||
		existing.CourseID != result.CourseID ||
		existing.TeachingClassID != result.TeachingClassID ||
		existing.State != string(result.State) {
		return fmt.Errorf(
			"选课结果幂等冲突: application_id=%s",
			result.ApplicationID,
		)
	}
	return nil
}
