package enrollmentapp

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeEnrollmentCountProjectionStore struct {
	projectLimit  int
	processedAt   time.Time
	cleanupLimit  int
	cleanupBefore time.Time
	projectErr    error
	cleanupErr    error
}

func (f *fakeEnrollmentCountProjectionStore) ProjectPendingEnrollmentCounts(
	_ context.Context,
	limit int,
	processedAt time.Time,
) (int, error) {
	f.projectLimit = limit
	f.processedAt = processedAt
	return limit, f.projectErr
}

func (f *fakeEnrollmentCountProjectionStore) DeleteProcessedEnrollmentCountDeltas(
	_ context.Context,
	before time.Time,
	limit int,
) (int64, error) {
	f.cleanupBefore = before
	f.cleanupLimit = limit
	return int64(limit), f.cleanupErr
}

func TestEnrollmentCountProjectionProjectsAndCleansByRetention(t *testing.T) {
	store := &fakeEnrollmentCountProjectionStore{}
	now := time.Date(2026, time.August, 7, 12, 0, 0, 0, time.Local)
	usecase := NewEnrollmentCountProjectionUsecase(store, 7*24*time.Hour)
	usecase.now = func() time.Time { return now }

	if err := usecase.ProjectBatch(context.Background(), 500); err != nil {
		t.Fatalf("ProjectBatch() error = %v", err)
	}
	if store.projectLimit != 500 || !store.processedAt.Equal(now) {
		t.Fatalf("project call = limit:%d at:%v", store.projectLimit, store.processedAt)
	}
	if err := usecase.Cleanup(context.Background(), 1000); err != nil {
		t.Fatalf("Cleanup() error = %v", err)
	}
	wantBefore := now.Add(-7 * 24 * time.Hour)
	if store.cleanupLimit != 1000 || !store.cleanupBefore.Equal(wantBefore) {
		t.Fatalf("cleanup call = limit:%d before:%v", store.cleanupLimit, store.cleanupBefore)
	}
}

func TestEnrollmentCountProjectionPropagatesStoreErrors(t *testing.T) {
	projectErr := errors.New("project failed")
	store := &fakeEnrollmentCountProjectionStore{projectErr: projectErr}
	usecase := NewEnrollmentCountProjectionUsecase(store, 7*24*time.Hour)
	if err := usecase.ProjectBatch(context.Background(), 500); !errors.Is(err, projectErr) {
		t.Fatalf("ProjectBatch() error = %v, want %v", err, projectErr)
	}

	cleanupErr := errors.New("cleanup failed")
	store.projectErr = nil
	store.cleanupErr = cleanupErr
	if err := usecase.Cleanup(context.Background(), 1000); !errors.Is(err, cleanupErr) {
		t.Fatalf("Cleanup() error = %v, want %v", err, cleanupErr)
	}
}
