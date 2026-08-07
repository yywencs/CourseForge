package enrollmentapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
)

func TestBindRoundClassUsesDomainPolicyBeforeInsert(t *testing.T) {
	repository := &roundRepositoryStub{
		round: &enrollment.SelectionRound{
			ID: 3, TermID: 202601, State: enrollment.SelectionRoundStatePlanned,
		},
		candidate: enrollment.RoundClassCandidate{
			TermID: 202602, State: enrollment.BindingTeachingClassStatePlanned,
		},
	}
	service := NewRoundManagementService(repository)

	err := service.BindRoundClass(context.Background(), 3, 2)
	if !errors.Is(err, enrollment.ErrTermMismatch) {
		t.Fatalf("BindRoundClass() error = %v, want %v", err, enrollment.ErrTermMismatch)
	}
	if repository.insertBindingCalled {
		t.Fatal("InsertRoundClass() called after domain rejected binding")
	}
}

func TestDeleteRoundUsesUsageFactsBeforeRemove(t *testing.T) {
	repository := &roundRepositoryStub{
		round: &enrollment.SelectionRound{
			ID: 3, TermID: 202601, State: enrollment.SelectionRoundStatePlanned,
		},
		roundUsage: enrollment.SelectionRoundUsage{QuotaCount: 1},
	}
	service := NewRoundManagementService(repository)

	err := service.DeleteRound(context.Background(), 3)
	if !errors.Is(err, enrollment.ErrRoundInUse) {
		t.Fatalf("DeleteRound() error = %v, want %v", err, enrollment.ErrRoundInUse)
	}
	if repository.removeRoundCalled {
		t.Fatal("RemoveRound() called after domain rejected deletion")
	}
}

type roundWarmupStatusStub struct{ status *RoundWarmupStatus }

func (s roundWarmupStatusStub) Status(context.Context, uint64) (*RoundWarmupStatus, error) {
	return s.status, nil
}
func (roundWarmupStatusStub) MarkQueued(context.Context, RoundWarmupStatus, time.Duration) error {
	return nil
}
func (roundWarmupStatusStub) MarkFailed(context.Context, RoundWarmupStatus, time.Duration) error {
	return nil
}
func (roundWarmupStatusStub) MarkOpen(context.Context, uint64, string) error { return nil }

func TestOpenRoundRequiresReadyWarmup(t *testing.T) {
	repository := &roundRepositoryStub{
		round:      &enrollment.SelectionRound{ID: 3, State: enrollment.SelectionRoundStatePlanned},
		roundUsage: enrollment.SelectionRoundUsage{ClassBindingCount: 1, QuotaCount: 1},
	}
	service := NewRoundManagementService(repository)
	service.ConfigureWarmup(nil, roundWarmupStatusStub{})
	if _, err := service.OpenRound(context.Background(), 3); !errors.Is(err, enrollment.ErrRoundNotReady) {
		t.Fatalf("OpenRound() error = %v, want %v", err, enrollment.ErrRoundNotReady)
	}

	service.ConfigureWarmup(nil, roundWarmupStatusStub{status: &RoundWarmupStatus{
		RoundID: 3, Version: "v1", State: RoundWarmupStateReady,
	}})
	if _, err := service.OpenRound(context.Background(), 3); err != nil {
		t.Fatalf("OpenRound() error = %v", err)
	}
	if repository.round.State != enrollment.SelectionRoundStateOpen {
		t.Fatalf("round state = %s", repository.round.State)
	}
}

type recordingWarmupControl struct{ status *RoundWarmupStatus }

func (s *recordingWarmupControl) Status(context.Context, uint64) (*RoundWarmupStatus, error) {
	return s.status, nil
}
func (s *recordingWarmupControl) MarkQueued(_ context.Context, status RoundWarmupStatus, _ time.Duration) error {
	s.status = &status
	return nil
}
func (s *recordingWarmupControl) MarkFailed(_ context.Context, status RoundWarmupStatus, _ time.Duration) error {
	s.status = &status
	return nil
}
func (*recordingWarmupControl) MarkOpen(context.Context, uint64, string) error { return nil }

type recordingWarmupEnqueuer struct{ roundID uint64 }

func (e *recordingWarmupEnqueuer) Enqueue(_ context.Context, roundID uint64) error {
	e.roundID = roundID
	return nil
}

func TestRequestWarmupFreezesConfigurationBeforeEnqueue(t *testing.T) {
	repository := &roundRepositoryStub{
		round:      &enrollment.SelectionRound{ID: 3, State: enrollment.SelectionRoundStatePlanned},
		roundUsage: enrollment.SelectionRoundUsage{ClassBindingCount: 1, QuotaCount: 1},
	}
	control := &recordingWarmupControl{}
	enqueuer := &recordingWarmupEnqueuer{}
	service := NewRoundManagementService(repository)
	service.ConfigureWarmup(enqueuer, control)

	if err := service.RequestWarmup(context.Background(), 3); err != nil {
		t.Fatalf("RequestWarmup() error = %v", err)
	}
	if control.status == nil || control.status.State != RoundWarmupStateQueued || enqueuer.roundID != 3 {
		t.Fatalf("status = %+v, enqueued round = %d", control.status, enqueuer.roundID)
	}
	if err := service.RequestWarmup(context.Background(), 3); !errors.Is(err, ErrRoundWarmupRunning) {
		t.Fatalf("second RequestWarmup() error = %v, want %v", err, ErrRoundWarmupRunning)
	}
	if err := service.BindRoundClass(context.Background(), 3, 5); !errors.Is(err, ErrRoundWarmupRunning) {
		t.Fatalf("BindRoundClass() error = %v, want %v", err, ErrRoundWarmupRunning)
	}
}

type roundRepositoryStub struct {
	round               *enrollment.SelectionRound
	roundUsage          enrollment.SelectionRoundUsage
	candidate           enrollment.RoundClassCandidate
	insertBindingCalled bool
	removeRoundCalled   bool
}

func (r *roundRepositoryStub) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (r *roundRepositoryStub) ListRounds(context.Context, uint64) ([]SelectionRoundView, error) {
	return nil, nil
}
func (r *roundRepositoryStub) GetRoundForUpdate(context.Context, uint64) (*enrollment.SelectionRound, error) {
	if r.round == nil {
		return nil, enrollment.ErrNotFound
	}
	return r.round, nil
}
func (r *roundRepositoryStub) InsertRound(context.Context, *enrollment.SelectionRound) error {
	return nil
}
func (r *roundRepositoryStub) SaveRound(context.Context, *enrollment.SelectionRound) error {
	return nil
}
func (r *roundRepositoryStub) RemoveRound(context.Context, uint64) error {
	r.removeRoundCalled = true
	return nil
}
func (r *roundRepositoryStub) InspectRoundUsage(context.Context, uint64) (enrollment.SelectionRoundUsage, error) {
	return r.roundUsage, nil
}
func (r *roundRepositoryStub) GetRoundClassCandidateForUpdate(context.Context, uint64) (enrollment.RoundClassCandidate, error) {
	return r.candidate, nil
}
func (r *roundRepositoryStub) ListRoundClasses(context.Context, uint64) ([]RoundClassBindingView, error) {
	return nil, nil
}
func (r *roundRepositoryStub) InsertRoundClass(context.Context, uint64, uint64) error {
	r.insertBindingCalled = true
	return nil
}
func (r *roundRepositoryStub) RemoveRoundClass(context.Context, uint64, uint64) error {
	return nil
}
func (r *roundRepositoryStub) OpenRound(context.Context, *enrollment.SelectionRound) error {
	return nil
}
