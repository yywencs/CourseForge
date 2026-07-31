package enrollment

import (
	"errors"
	"testing"
	"time"
)

func newTestWaitlistEntry(t *testing.T) (*WaitlistEntry, time.Time) {
	t.Helper()
	now := time.Date(2026, time.September, 1, 8, 0, 0, 0, time.Local)
	entry, err := NewWaitlistEntry("waitlist-1", &SelectionRequest{
		RequestID:       "request-1",
		RoundID:         101,
		TermID:          202601,
		StudentID:       10001,
		CourseID:        20001,
		TeachingClassID: 30001,
		Credits:         Credit(35),
		Source:          ApplicationSourceWeb,
	}, now)
	if err != nil {
		t.Fatalf("NewWaitlistEntry() error = %v", err)
	}
	return entry, now
}

func TestWaitlistEntryPromotionStateTransition(t *testing.T) {
	entry, now := newTestWaitlistEntry(t)
	if err := entry.MarkPromoted(now.Add(time.Minute)); !errors.Is(err, ErrInvalidWaitlistState) {
		t.Fatalf("waiting 直接晋级 error = %v, want %v", err, ErrInvalidWaitlistState)
	}
	entry.State = WaitlistStatePromoting
	if err := entry.MarkPromoted(now.Add(time.Minute)); err != nil {
		t.Fatalf("MarkPromoted() error = %v", err)
	}
	if entry.State != WaitlistStatePromoted || entry.PromotedAt == nil {
		t.Fatalf("晋级结果 = %#v", entry)
	}
}

func TestWaitlistEntryCancelIsIdempotent(t *testing.T) {
	entry, now := newTestWaitlistEntry(t)
	reason := FailureReason{Code: FailureCodeCancelled, Message: "学生主动取消候补"}
	if err := entry.Cancel(reason, now.Add(time.Minute)); err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if err := entry.Cancel(reason, now.Add(2*time.Minute)); err != nil {
		t.Fatalf("重复 Cancel() error = %v", err)
	}
	if entry.State != WaitlistStateCancelled || entry.CancelledAt == nil {
		t.Fatalf("取消结果 = %#v", entry)
	}
}

func TestDecidePromotionFailure(t *testing.T) {
	retry := DecidePromotionFailure(ErrTeachingClassFull)
	if retry.Action != PromotionFailureActionRetry {
		t.Fatalf("full class decision = %#v", retry)
	}

	cancel := DecidePromotionFailure(ErrPrerequisiteNotMet)
	if cancel.Action != PromotionFailureActionCancel ||
		cancel.Reason.Code != FailureCodePrerequisite ||
		cancel.Reason.Message == "" {
		t.Fatalf("prerequisite decision = %#v", cancel)
	}
}
