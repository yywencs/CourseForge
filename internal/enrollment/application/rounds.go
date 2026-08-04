package enrollmentapp

import (
	"context"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
)

type SelectionRoundView struct {
	ID         uint64
	TermID     uint64
	RoundCode  string
	RoundName  string
	StartTime  time.Time
	EndTime    time.Time
	State      enrollment.SelectionRoundState
	ClassCount int64
	CreateTime time.Time
	UpdateTime time.Time
}

type RoundClassBindingView struct {
	ID              uint64
	RoundID         uint64
	TeachingClassID uint64
	ClassCode       string
	CourseName      string
	State           string
	CreateTime      time.Time
}

type RoundManagementRepository interface {
	WithinTransaction(context.Context, func(context.Context) error) error
	ListRounds(context.Context, uint64) ([]SelectionRoundView, error)
	GetRoundForUpdate(context.Context, uint64) (*enrollment.SelectionRound, error)
	InsertRound(context.Context, *enrollment.SelectionRound) error
	SaveRound(context.Context, *enrollment.SelectionRound) error
	RemoveRound(context.Context, uint64) error
	InspectRoundUsage(context.Context, uint64) (enrollment.SelectionRoundUsage, error)
	GetRoundClassCandidateForUpdate(context.Context, uint64) (enrollment.RoundClassCandidate, error)
	ListRoundClasses(context.Context, uint64) ([]RoundClassBindingView, error)
	InsertRoundClass(context.Context, uint64, uint64) error
	RemoveRoundClass(context.Context, uint64, uint64) error
}

type RoundManagementService struct {
	repository RoundManagementRepository
}

func NewRoundManagementService(repository RoundManagementRepository) *RoundManagementService {
	return &RoundManagementService{repository: repository}
}

func (s *RoundManagementService) ListRounds(ctx context.Context, termID uint64) ([]SelectionRoundView, error) {
	return s.repository.ListRounds(ctx, termID)
}

func (s *RoundManagementService) CreateRound(
	ctx context.Context,
	input SelectionRoundCommand,
) (*enrollment.SelectionRound, error) {
	round, err := enrollment.NewSelectionRound(input.plan())
	if err != nil {
		return nil, err
	}
	if err := s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		return s.repository.InsertRound(txCtx, round)
	}); err != nil {
		return nil, err
	}
	return round, nil
}

func (s *RoundManagementService) UpdateRound(
	ctx context.Context,
	id uint64,
	input SelectionRoundCommand,
) (*enrollment.SelectionRound, error) {
	var round *enrollment.SelectionRound
	err := s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		current, err := s.repository.GetRoundForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		usage, err := s.repository.InspectRoundUsage(txCtx, id)
		if err != nil {
			return err
		}
		if err := current.ChangePlan(input.plan(), usage); err != nil {
			return err
		}
		if err := s.repository.SaveRound(txCtx, current); err != nil {
			return err
		}
		round = current
		return nil
	})
	return round, err
}

func (s *RoundManagementService) DeleteRound(ctx context.Context, id uint64) error {
	return s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		round, err := s.repository.GetRoundForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		usage, err := s.repository.InspectRoundUsage(txCtx, id)
		if err != nil {
			return err
		}
		if err := round.EnsureDeletable(usage); err != nil {
			return err
		}
		return s.repository.RemoveRound(txCtx, id)
	})
}

func (s *RoundManagementService) ListRoundClasses(
	ctx context.Context,
	roundID uint64,
) ([]RoundClassBindingView, error) {
	return s.repository.ListRoundClasses(ctx, roundID)
}

func (s *RoundManagementService) BindRoundClass(
	ctx context.Context,
	roundID uint64,
	teachingClassID uint64,
) error {
	return s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		round, err := s.repository.GetRoundForUpdate(txCtx, roundID)
		if err != nil {
			return err
		}
		candidate, err := s.repository.GetRoundClassCandidateForUpdate(txCtx, teachingClassID)
		if err != nil {
			return err
		}
		if err := round.EnsureCanBind(candidate); err != nil {
			return err
		}
		return s.repository.InsertRoundClass(txCtx, roundID, teachingClassID)
	})
}

func (s *RoundManagementService) UnbindRoundClass(
	ctx context.Context,
	roundID uint64,
	teachingClassID uint64,
) error {
	return s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		round, err := s.repository.GetRoundForUpdate(txCtx, roundID)
		if err != nil {
			return err
		}
		if err := round.EnsureBindingsMutable(); err != nil {
			return err
		}
		return s.repository.RemoveRoundClass(txCtx, roundID, teachingClassID)
	})
}
