package enrollmentapp

import (
	"context"
	"errors"
	"testing"

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
