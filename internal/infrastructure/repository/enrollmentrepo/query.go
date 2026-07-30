package enrollmentrepo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"prizeforge/internal/domain/enrollment"

	"gorm.io/gorm"
)

func (r *Repository) QuerySelectionByRequest(
	ctx context.Context,
	roundID uint64,
	studentID uint64,
	requestID string,
) (*enrollment.SelectionRequestRecord, error) {
	if roundID == 0 || studentID == 0 || strings.TrimSpace(requestID) == "" {
		return nil, enrollment.ErrInvalidParams
	}
	record, err := r.querySelectionByRequestFromRedis(ctx, roundID, studentID, requestID)
	if err != nil {
		return nil, err
	}
	if record != nil {
		return record, nil
	}
	return r.querySelectionByRequestFromMySQL(ctx, roundID, studentID, requestID)
}

func (r *Repository) querySelectionByRequestFromMySQL(
	ctx context.Context,
	roundID uint64,
	studentID uint64,
	requestID string,
) (*enrollment.SelectionRequestRecord, error) {
	var row struct {
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
		Where(
			"round_id = ? AND student_id = ? AND request_id = ?",
			roundID,
			studentID,
			requestID,
		).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
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
	application := &enrollment.SelectionApplication{
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
	}
	if application.ApplicationID == "" ||
		!application.Source.Valid() ||
		!application.State.Terminal() ||
		application.CompletedAt == nil {
		return nil, errors.New("MySQL选课申请状态非法")
	}
	return &enrollment.SelectionRequestRecord{
		Application:    application,
		MySQLPersisted: true,
	}, nil
}

func (r *Repository) QuerySelectionRound(
	ctx context.Context,
	roundID uint64,
) (*enrollment.SelectionRound, error) {
	if roundID == 0 {
		return nil, enrollment.ErrInvalidParams
	}
	var row struct {
		ID        uint64
		TermID    uint64
		StartTime timeValue
		EndTime   timeValue
		State     string
	}
	err := r.db.WithContext(ctx).
		Table("selection_round").
		Select("id, term_id, start_time, end_time, state").
		Where("id = ?", roundID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &enrollment.SelectionRound{
		ID:        row.ID,
		TermID:    row.TermID,
		StartTime: row.StartTime.Time,
		EndTime:   row.EndTime.Time,
		State:     enrollment.SelectionRoundState(row.State),
	}, nil
}

func (r *Repository) QueryTeachingClass(
	ctx context.Context,
	roundID uint64,
	teachingClassID uint64,
) (*enrollment.TeachingClass, error) {
	if roundID == 0 || teachingClassID == 0 {
		return nil, enrollment.ErrInvalidParams
	}
	var row struct {
		ID            uint64
		TermID        uint64
		CourseID      uint64
		Credits       string
		Capacity      uint32
		SelectedCount uint32
		State         string
		RoundState    string
	}
	err := r.db.WithContext(ctx).
		Table("teaching_class AS tc").
		Select(`
			tc.id,
			tc.term_id,
			tc.course_id,
			CAST(c.credits AS CHAR) AS credits,
			tc.capacity,
			tc.selected_count,
			tc.state,
			src.state AS round_state
		`).
		Joins("JOIN course c ON c.id = tc.course_id").
		Joins("JOIN selection_round_class src ON src.teaching_class_id = tc.id AND src.round_id = ?", roundID).
		Where("tc.id = ?", teachingClassID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	credits, err := creditFromDecimal(row.Credits)
	if err != nil {
		return nil, fmt.Errorf("解析教学班学分: %w", err)
	}
	state := enrollment.TeachingClassState(row.State)
	if row.RoundState != "open" && state == enrollment.TeachingClassStateOpen {
		state = enrollment.TeachingClassStateClosed
	}
	return &enrollment.TeachingClass{
		ID:            row.ID,
		TermID:        row.TermID,
		CourseID:      row.CourseID,
		Credits:       credits,
		Capacity:      row.Capacity,
		SelectedCount: row.SelectedCount,
		State:         state,
	}, nil
}

func (r *Repository) QueryStudentSelectionQuota(
	ctx context.Context,
	roundID uint64,
	studentID uint64,
) (*enrollment.StudentSelectionQuota, error) {
	if roundID == 0 || studentID == 0 {
		return nil, enrollment.ErrInvalidParams
	}
	var row struct {
		RoundID             uint64
		TermID              uint64
		StudentID           uint64
		CreditLimit         string
		SelectedCredits     string
		CourseLimit         uint16
		SelectedCourseCount uint16
	}
	err := r.db.WithContext(ctx).
		Table("student_selection_quota").
		Select(`
			round_id,
			term_id,
			student_id,
			CAST(credit_limit AS CHAR) AS credit_limit,
			CAST(selected_credits AS CHAR) AS selected_credits,
			course_limit,
			selected_course_count
		`).
		Where("round_id = ? AND student_id = ?", roundID, studentID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	creditLimit, err := creditFromDecimal(row.CreditLimit)
	if err != nil {
		return nil, fmt.Errorf("解析学分上限: %w", err)
	}
	selectedCredits, err := creditFromDecimalAllowZero(row.SelectedCredits)
	if err != nil {
		return nil, fmt.Errorf("解析已选学分: %w", err)
	}
	return &enrollment.StudentSelectionQuota{
		RoundID:             row.RoundID,
		TermID:              row.TermID,
		StudentID:           row.StudentID,
		CreditLimit:         creditLimit,
		SelectedCredits:     selectedCredits,
		CourseLimit:         row.CourseLimit,
		SelectedCourseCount: row.SelectedCourseCount,
	}, nil
}

func (r *Repository) IsStudentActive(ctx context.Context, studentID uint64) (bool, error) {
	if studentID == 0 {
		return false, enrollment.ErrInvalidParams
	}
	var count int64
	err := r.db.WithContext(ctx).
		Table("student").
		Where("id = ? AND state = ?", studentID, "active").
		Count(&count).Error
	return count > 0, err
}

func (r *Repository) HasExistingEnrollment(
	ctx context.Context,
	termID uint64,
	studentID uint64,
	courseID uint64,
) (bool, error) {
	if termID == 0 || studentID == 0 || courseID == 0 {
		return false, enrollment.ErrInvalidParams
	}
	var count int64
	err := r.db.WithContext(ctx).
		Table("student_course_enrollment").
		Where(
			"term_id = ? AND student_id = ? AND course_id = ? AND state IN ?",
			termID,
			studentID,
			courseID,
			[]string{"enrolled", "completed"},
		).
		Count(&count).Error
	return count > 0, err
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
