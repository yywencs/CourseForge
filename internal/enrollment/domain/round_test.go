package enrollment

import (
	"errors"
	"testing"
	"time"
)

func TestSelectionRoundOpenRequiresClassesAndQuotas(t *testing.T) {
	round := &SelectionRound{ID: 1, State: SelectionRoundStatePlanned}
	if err := round.Open(SelectionRoundUsage{ClassBindingCount: 1}, true); !errors.Is(err, ErrRoundConfigurationEmpty) {
		t.Fatalf("Open() error = %v, want %v", err, ErrRoundConfigurationEmpty)
	}
	if err := round.Open(SelectionRoundUsage{ClassBindingCount: 1, QuotaCount: 1}, false); !errors.Is(err, ErrRoundNotReady) {
		t.Fatalf("Open() error = %v, want %v", err, ErrRoundNotReady)
	}
	if err := round.Open(SelectionRoundUsage{ClassBindingCount: 1, QuotaCount: 1}, true); err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	if round.State != SelectionRoundStateOpen {
		t.Fatalf("round state = %s", round.State)
	}
}

func TestSelectionRoundOwnsChangeDeleteAndBindingRules(t *testing.T) {
	round := validManagedRound()
	changed := validRoundPlan()
	changed.TermID = 202602
	if err := round.ChangePlan(changed, SelectionRoundUsage{ClassBindingCount: 1}); !errors.Is(err, ErrRoundTermLocked) {
		t.Fatalf("ChangePlan() error = %v, want %v", err, ErrRoundTermLocked)
	}
	if err := round.EnsureDeletable(SelectionRoundUsage{QuotaCount: 1}); !errors.Is(err, ErrRoundInUse) {
		t.Fatalf("EnsureDeletable() error = %v, want %v", err, ErrRoundInUse)
	}
	candidate := RoundClassCandidate{TermID: 202602, State: BindingTeachingClassStatePlanned}
	if err := round.EnsureCanBind(candidate); !errors.Is(err, ErrTermMismatch) {
		t.Fatalf("EnsureCanBind() error = %v, want %v", err, ErrTermMismatch)
	}
	round.State = SelectionRoundStateOpen
	if err := round.EnsureBindingsMutable(); !errors.Is(err, ErrRoundNotEditable) {
		t.Fatalf("EnsureBindingsMutable() error = %v, want %v", err, ErrRoundNotEditable)
	}
}

func TestSelectionRoundRejectsReversedTimeRange(t *testing.T) {
	plan := validRoundPlan()
	plan.EndTime = plan.StartTime.Add(-time.Minute)
	if _, err := NewSelectionRound(plan); !errors.Is(err, ErrInvalidTimeRange) {
		t.Fatalf("NewSelectionRound() error = %v, want %v", err, ErrInvalidTimeRange)
	}
}

func validRoundPlan() SelectionRoundPlan {
	start := time.Date(2026, 9, 1, 8, 0, 0, 0, time.Local)
	return SelectionRoundPlan{
		TermID: 202601, RoundCode: "ROUND-1", RoundName: "第一轮",
		StartTime: start, EndTime: start.Add(24 * time.Hour),
	}
}

func validManagedRound() SelectionRound {
	round, err := NewSelectionRound(validRoundPlan())
	if err != nil {
		panic(err)
	}
	round.ID = 1
	return *round
}
