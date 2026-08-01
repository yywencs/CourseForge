package enrollmentapp

import (
	"context"
	"time"

	"prizeforge/internal/enrollment/domain"
)

// SelectionAdmissionService loads admission facts and delegates business decisions
// to the pure enrollment domain model.
type SelectionAdmissionService struct {
	selections  SelectionAdmissionQuery
	eligibility EligibilityQuery
	policy      enrollment.EligibilityPolicy
}

func NewSelectionAdmissionService(
	selections SelectionAdmissionQuery,
	eligibility EligibilityQuery,
) *SelectionAdmissionService {
	return &SelectionAdmissionService{
		selections:  selections,
		eligibility: eligibility,
		policy:      enrollment.EligibilityPolicy{},
	}
}

func (s *SelectionAdmissionService) AdmitSelection(
	ctx context.Context,
	intent enrollment.SelectionIntent,
	now time.Time,
) (*enrollment.SelectionRequest, error) {
	request, class, err := s.prepare(ctx, intent, now)
	if err != nil {
		return nil, err
	}
	if err := class.ValidateForSelection(request); err != nil {
		return nil, err
	}
	return s.evaluateStudent(ctx, request)
}

func (s *SelectionAdmissionService) AdmitWaitlist(
	ctx context.Context,
	intent enrollment.SelectionIntent,
	now time.Time,
) (*enrollment.SelectionRequest, error) {
	request, class, err := s.prepare(ctx, intent, now)
	if err != nil {
		return nil, err
	}
	if err := class.ValidateForWaitlist(request); err != nil {
		return nil, err
	}
	return s.evaluateStudent(ctx, request)
}

func (s *SelectionAdmissionService) prepare(
	ctx context.Context,
	intent enrollment.SelectionIntent,
	now time.Time,
) (*enrollment.SelectionRequest, *enrollment.TeachingClass, error) {
	if s == nil || s.selections == nil || s.eligibility == nil ||
		!intent.Valid() || now.IsZero() {
		return nil, nil, enrollment.ErrInvalidParams
	}
	round, err := s.selections.QuerySelectionRound(ctx, intent.RoundID())
	if err != nil {
		return nil, nil, err
	}
	if round == nil {
		return nil, nil, enrollment.ErrRecordNotFound
	}
	if err := round.EnsureAcceptingAt(now); err != nil {
		return nil, nil, err
	}
	class, err := s.selections.QueryTeachingClass(ctx, round.ID, intent.TeachingClassID())
	if err != nil {
		return nil, nil, err
	}
	if class == nil {
		return nil, nil, enrollment.ErrRecordNotFound
	}
	request := &enrollment.SelectionRequest{
		RequestID:       intent.RequestID(),
		RoundID:         round.ID,
		TermID:          round.TermID,
		StudentID:       intent.StudentID(),
		CourseID:        class.CourseID,
		TeachingClassID: class.ID,
		Credits:         class.Credits,
		Source:          intent.Source(),
	}
	if err := request.Validate(); err != nil {
		return nil, nil, err
	}
	return request, class, nil
}

func (s *SelectionAdmissionService) evaluateStudent(
	ctx context.Context,
	request *enrollment.SelectionRequest,
) (*enrollment.SelectionRequest, error) {
	snapshot, err := s.eligibility.QueryEligibilitySnapshot(
		ctx,
		request.StudentID,
		request.TermID,
		request.CourseID,
		request.TeachingClassID,
	)
	if err != nil {
		return nil, err
	}
	if err := s.policy.Evaluate(snapshot); err != nil {
		return nil, err
	}
	exists, err := s.selections.HasExistingEnrollment(
		ctx,
		request.TermID,
		request.StudentID,
		request.CourseID,
	)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, enrollment.ErrDuplicateSelection
	}
	quota, err := s.selections.QueryStudentSelectionQuota(
		ctx,
		request.RoundID,
		request.StudentID,
	)
	if err != nil {
		return nil, err
	}
	if quota == nil {
		return nil, enrollment.ErrRecordNotFound
	}
	if err := quota.ValidateReservation(request); err != nil {
		return nil, err
	}
	return request, nil
}
