package api

import (
	"context"
	"testing"
	"time"

	"prizeforge/internal/domain/enrollment"
)

type fakeWaitlistRepository struct {
	joined *enrollment.WaitlistEntry
}

func (f *fakeWaitlistRepository) JoinWaitlist(
	_ context.Context, entry *enrollment.WaitlistEntry,
) (*enrollment.WaitlistEntry, error) {
	entry.Position = 1
	f.joined = entry
	return entry, nil
}

func (f *fakeWaitlistRepository) QueryWaitlist(
	context.Context, string, uint64,
) (*enrollment.WaitlistEntry, error) {
	return f.joined, nil
}

func (f *fakeWaitlistRepository) ListStudentWaitlist(
	context.Context, uint64, uint64, int, int,
) (*enrollment.WaitlistPage, error) {
	return &enrollment.WaitlistPage{Items: []*enrollment.WaitlistEntry{f.joined}}, nil
}

func (f *fakeWaitlistRepository) CancelWaitlist(context.Context, *enrollment.WaitlistEntry) error {
	return nil
}

func (f *fakeWaitlistRepository) ClaimPromotableEntries(
	context.Context, time.Time, int,
) ([]*enrollment.WaitlistEntry, error) {
	return nil, nil
}

func (f *fakeWaitlistRepository) ClaimExpiredEntries(
	context.Context, time.Time, int,
) ([]*enrollment.WaitlistEntry, error) {
	return nil, nil
}

func (f *fakeWaitlistRepository) MarkWaitlistPromoted(
	context.Context, *enrollment.WaitlistEntry,
) error {
	return nil
}

func (f *fakeWaitlistRepository) ReturnWaitlistToQueue(
	context.Context, *enrollment.WaitlistEntry,
) error {
	return nil
}

func TestWaitlistUsecaseJoinsFullClass(t *testing.T) {
	selector, queryRepo, _, now := newSuccessfulEnrollmentUsecase(t)
	queryRepo.class.SelectedCount = queryRepo.class.Capacity
	waitlistRepo := &fakeWaitlistRepository{}
	usecase := NewWaitlistUsecase(waitlistRepo, selector, selector.admission)
	usecase.now = func() time.Time { return now }
	usecase.newID = func() (string, error) { return "waitlist-1", nil }

	entry, err := usecase.Join(context.Background(), &JoinWaitlistCommand{
		RequestID:       "wait-request-1",
		RoundID:         101,
		StudentID:       10001,
		TeachingClassID: 30001,
	})
	if err != nil {
		t.Fatalf("Join() error = %v", err)
	}
	if entry.State != enrollment.WaitlistStateWaiting || entry.Position != 1 {
		t.Fatalf("entry = %#v", entry)
	}
}
