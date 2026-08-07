package enrollmentapp

import (
	"context"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
)

type SelectionRoundView struct {
	ID            uint64
	TermID        uint64
	RoundCode     string
	RoundName     string
	StartTime     time.Time
	EndTime       time.Time
	State         enrollment.SelectionRoundState
	ClassCount    int64
	CreateTime    time.Time
	UpdateTime    time.Time
	WarmupState   RoundWarmupState
	WarmupVersion string
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
	OpenRound(context.Context, *enrollment.SelectionRound) error
}

type RoundWarmupEnqueuer interface {
	Enqueue(context.Context, uint64) error
}

type RoundWarmupStatusStore interface {
	Status(context.Context, uint64) (*RoundWarmupStatus, error)
	MarkQueued(context.Context, RoundWarmupStatus, time.Duration) error
	MarkFailed(context.Context, RoundWarmupStatus, time.Duration) error
	MarkOpen(context.Context, uint64, string) error
}

type RoundManagementService struct {
	repository     RoundManagementRepository
	warmupEnqueuer RoundWarmupEnqueuer
	warmupStatus   RoundWarmupStatusStore
}

// ConfigureWarmup 为管理服务装配异步预热命令与状态查询。
func (s *RoundManagementService) ConfigureWarmup(
	enqueuer RoundWarmupEnqueuer,
	status RoundWarmupStatusStore,
) {
	s.warmupEnqueuer = enqueuer
	s.warmupStatus = status
}

func (s *RoundManagementService) RequestWarmup(ctx context.Context, roundID uint64) error {
	if s == nil || s.warmupEnqueuer == nil || s.warmupStatus == nil || roundID == 0 {
		return enrollment.ErrInvalidParams
	}
	status, err := s.warmupStatus.Status(ctx, roundID)
	if err != nil {
		return err
	}
	if status != nil {
		if status.State == RoundWarmupStateReady {
			return enrollment.ErrRoundAlreadyWarmed
		}
		if status.State == RoundWarmupStateQueued || status.State == RoundWarmupStateRunning {
			return ErrRoundWarmupRunning
		}
	}
	const statusTTL = 30 * 24 * time.Hour
	status = &RoundWarmupStatus{
		RoundID: roundID, State: RoundWarmupStateQueued, StartedAt: time.Now(),
	}
	markedQueued := false
	if err := s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		round, err := s.repository.GetRoundForUpdate(txCtx, roundID)
		if err != nil {
			return err
		}
		if err := s.ensureConfigurationMutable(txCtx, roundID); err != nil {
			return err
		}
		usage, err := s.repository.InspectRoundUsage(txCtx, roundID)
		if err != nil {
			return err
		}
		if err := round.EnsureWarmupConfig(usage); err != nil {
			return err
		}
		if err := s.warmupStatus.MarkQueued(txCtx, *status, statusTTL); err != nil {
			return err
		}
		markedQueued = true
		return nil
	}); err != nil {
		if markedQueued {
			finishedAt := time.Now()
			status.State = RoundWarmupStateFailed
			status.FinishedAt = &finishedAt
			status.ErrorMessage = err.Error()
			_ = s.warmupStatus.MarkFailed(context.Background(), *status, statusTTL)
		}
		return err
	}
	if err := s.warmupEnqueuer.Enqueue(ctx, roundID); err != nil {
		finishedAt := time.Now()
		status.State = RoundWarmupStateFailed
		status.FinishedAt = &finishedAt
		status.ErrorMessage = err.Error()
		_ = s.warmupStatus.MarkFailed(context.Background(), *status, statusTTL)
		return err
	}
	return nil
}

func (s *RoundManagementService) WarmupStatus(
	ctx context.Context, roundID uint64,
) (*RoundWarmupStatus, error) {
	if s == nil || s.warmupStatus == nil || roundID == 0 {
		return nil, enrollment.ErrInvalidParams
	}
	return s.warmupStatus.Status(ctx, roundID)
}

// OpenRound 只允许已经完成当前静态资格预热的轮次开放。
func (s *RoundManagementService) OpenRound(ctx context.Context, roundID uint64) (*enrollment.SelectionRound, error) {
	status, err := s.WarmupStatus(ctx, roundID)
	if err != nil {
		return nil, err
	}
	ready := status != nil && status.State == RoundWarmupStateReady && status.Version != ""
	if !ready {
		return nil, enrollment.ErrRoundNotReady
	}
	var opened *enrollment.SelectionRound
	err = s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		round, err := s.repository.GetRoundForUpdate(txCtx, roundID)
		if err != nil {
			return err
		}
		if round.State == enrollment.SelectionRoundStateOpen {
			opened = round
			return nil
		}
		usage, err := s.repository.InspectRoundUsage(txCtx, roundID)
		if err != nil {
			return err
		}
		if err := round.Open(usage, ready); err != nil {
			return err
		}
		if err := s.repository.OpenRound(txCtx, round); err != nil {
			return err
		}
		opened = round
		return nil
	})
	if err != nil {
		return nil, err
	}
	if err := s.warmupStatus.MarkOpen(ctx, roundID, status.Version); err != nil {
		return nil, err
	}
	return opened, nil
}

func (s *RoundManagementService) ensureConfigurationMutable(ctx context.Context, roundID uint64) error {
	if s.warmupStatus == nil {
		return nil
	}
	status, err := s.warmupStatus.Status(ctx, roundID)
	if err != nil {
		return err
	}
	if status != nil {
		if status.State == RoundWarmupStateReady {
			return enrollment.ErrRoundAlreadyWarmed
		}
		if status.State == RoundWarmupStateQueued || status.State == RoundWarmupStateRunning {
			return ErrRoundWarmupRunning
		}
	}
	return nil
}

func NewRoundManagementService(repository RoundManagementRepository) *RoundManagementService {
	return &RoundManagementService{repository: repository}
}

func (s *RoundManagementService) ListRounds(ctx context.Context, termID uint64) ([]SelectionRoundView, error) {
	items, err := s.repository.ListRounds(ctx, termID)
	if err != nil || s.warmupStatus == nil {
		return items, err
	}
	for i := range items {
		status, statusErr := s.warmupStatus.Status(ctx, items[i].ID)
		if statusErr != nil {
			return nil, statusErr
		}
		if status != nil {
			items[i].WarmupState = status.State
			items[i].WarmupVersion = status.Version
		}
	}
	return items, nil
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
	if err := s.ensureConfigurationMutable(ctx, id); err != nil {
		return nil, err
	}
	var round *enrollment.SelectionRound
	err := s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		current, err := s.repository.GetRoundForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if err := s.ensureConfigurationMutable(txCtx, id); err != nil {
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
	if err := s.ensureConfigurationMutable(ctx, id); err != nil {
		return err
	}
	return s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		round, err := s.repository.GetRoundForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		if err := s.ensureConfigurationMutable(txCtx, id); err != nil {
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
	if err := s.ensureConfigurationMutable(ctx, roundID); err != nil {
		return err
	}
	return s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		round, err := s.repository.GetRoundForUpdate(txCtx, roundID)
		if err != nil {
			return err
		}
		if err := s.ensureConfigurationMutable(txCtx, roundID); err != nil {
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
	if err := s.ensureConfigurationMutable(ctx, roundID); err != nil {
		return err
	}
	return s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		round, err := s.repository.GetRoundForUpdate(txCtx, roundID)
		if err != nil {
			return err
		}
		if err := s.ensureConfigurationMutable(txCtx, roundID); err != nil {
			return err
		}
		if err := round.EnsureBindingsMutable(); err != nil {
			return err
		}
		return s.repository.RemoveRoundClass(txCtx, roundID, teachingClassID)
	})
}
