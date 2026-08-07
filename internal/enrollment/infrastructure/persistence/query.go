package enrollmentrepo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	applicationapi "github.com/yywencs/courseforge/internal/enrollment/application"
	"github.com/yywencs/courseforge/internal/enrollment/domain"

	"gorm.io/gorm"
)

type selectionApplicationRow struct {
	ApplicationID   string
	RequestID       string
	RoundID         uint64
	TermID          uint64
	StudentID       uint64
	CourseID        uint64
	TeachingClassID uint64
	Credits         string
	State           string
	FailureCode     string
	FailureMessage  string
	AppliedAt       timeValue
	CompletedAt     *time.Time
	Source          string
}

func (r *QueryStore) QuerySelectionByRequest(
	ctx context.Context,
	roundID uint64,
	studentID uint64,
	requestID string,
) (*applicationapi.SelectionRequestRecord, error) {
	if roundID == 0 || studentID == 0 || strings.TrimSpace(requestID) == "" {
		return nil, enrollment.ErrInvalidParams
	}
	record, err := r.querySelectionByRequestFromRedis(ctx, roundID, studentID, requestID)
	if err != nil {
		return nil, err
	}
	return record, nil
}

func (r *QueryStore) QuerySelectionApplication(
	ctx context.Context,
	applicationID string,
	studentID uint64,
) (*applicationapi.SelectionApplicationRecord, error) {
	if strings.TrimSpace(applicationID) == "" || studentID == 0 {
		return nil, enrollment.ErrInvalidParams
	}

	var row selectionApplicationRow
	err := r.db.WithContext(ctx).
		Table("selection_application").
		Select(`
			application_id,
			request_id,
			round_id,
			term_id,
			student_id,
			course_id,
			teaching_class_id,
			CAST(credits AS CHAR) AS credits,
			state,
			failure_code,
			failure_message,
			applied_at,
			completed_at,
			source
		`).
		Where("application_id = ? AND student_id = ?", applicationID, studentID).
		Take(&row).Error
	if err == nil {
		application, convertErr := row.toEntity()
		if convertErr != nil {
			return nil, convertErr
		}
		return &applicationapi.SelectionApplicationRecord{
			Application:       application,
			DeliveryConfirmed: true,
			DurablyPersisted:  true,
		}, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return r.querySelectionApplicationFromRedis(ctx, applicationID, studentID)
}

func (r *QueryStore) ListStudentEnrollments(
	ctx context.Context,
	studentID uint64,
	termID uint64,
	limit int,
	offset int,
) (*enrollment.EnrollmentPage, error) {
	if studentID == 0 || termID == 0 || limit <= 0 || limit > 100 || offset < 0 {
		return nil, enrollment.ErrInvalidParams
	}

	query := r.db.WithContext(ctx).
		Table("student_course_enrollment AS sce").
		Joins("JOIN selection_application sa ON sa.application_id = sce.application_id").
		Where("sce.student_id = ? AND sce.term_id = ?", studentID, termID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}

	var rows []struct {
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
	if err := query.
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
		Order("sce.enrolled_at DESC, sce.id DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}

	items := make([]*enrollment.StudentEnrollment, 0, len(rows))
	for _, row := range rows {
		credits, err := creditFromDecimal(row.Credits)
		if err != nil {
			return nil, fmt.Errorf("解析正式选课记录学分: %w", err)
		}
		item := &enrollment.StudentEnrollment{
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
		if err := item.Validate(); err != nil {
			return nil, fmt.Errorf("正式选课记录非法: %w", err)
		}
		items = append(items, item)
	}
	return &enrollment.EnrollmentPage{
		Items:  items,
		Limit:  limit,
		Offset: offset,
		Total:  total,
	}, nil
}

func (row *selectionApplicationRow) toEntity() (*enrollment.SelectionApplication, error) {
	if row == nil {
		return nil, enrollment.ErrRecordNotFound
	}
	credits, err := creditFromDecimal(row.Credits)
	if err != nil {
		return nil, fmt.Errorf("解析已有选课申请学分: %w", err)
	}
	var failure *enrollment.FailureReason
	if row.FailureCode != "" || row.FailureMessage != "" {
		reason := enrollment.FailureReason{
			Code:    enrollment.FailureCode(row.FailureCode),
			Message: row.FailureMessage,
		}
		if !reason.Valid() {
			return nil, errors.New("MySQL选课申请失败原因非法")
		}
		failure = &reason
	}
	return &enrollment.SelectionApplication{
		ApplicationID:   row.ApplicationID,
		RequestID:       row.RequestID,
		RoundID:         row.RoundID,
		TermID:          row.TermID,
		StudentID:       row.StudentID,
		CourseID:        row.CourseID,
		TeachingClassID: row.TeachingClassID,
		Credits:         credits,
		Source:          enrollment.ApplicationSource(row.Source),
		State:           enrollment.ApplicationState(row.State),
		Failure:         failure,
		AppliedAt:       row.AppliedAt.Time,
		CompletedAt:     row.CompletedAt,
	}, nil
}

func creditFromDecimal(raw string) (enrollment.Credit, error) {
	credit, err := creditFromDecimalAllowZero(raw)
	if err != nil {
		return 0, err
	}
	if !credit.Valid() {
		return 0, errors.New("学分必须大于0")
	}
	return credit, nil
}

func creditFromDecimalAllowZero(raw string) (enrollment.Credit, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, errors.New("学分为空")
	}
	parts := strings.Split(raw, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("非法学分 %q", raw)
	}
	integer, err := strconv.ParseInt(parts[0], 10, 32)
	if err != nil || integer < 0 {
		return 0, fmt.Errorf("非法学分 %q", raw)
	}
	fraction := int64(0)
	if len(parts) == 2 && parts[1] != "" {
		if len(parts[1]) > 1 {
			return 0, fmt.Errorf("学分精度超过一位小数 %q", raw)
		}
		fraction, err = strconv.ParseInt(parts[1], 10, 8)
		if err != nil {
			return 0, fmt.Errorf("非法学分 %q", raw)
		}
	}
	return enrollment.Credit(integer*10 + fraction), nil
}

func creditToDecimal(credit enrollment.Credit) string {
	return credit.String()
}
