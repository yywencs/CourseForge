package enrollmentrepo

import (
	"errors"
	"testing"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
)

func validBatchSelectionResult() *enrollment.SelectionResult {
	now := time.Date(2026, time.September, 1, 8, 0, 0, 0, time.UTC)
	return &enrollment.SelectionResult{
		ApplicationID:   "application-001",
		RequestID:       "request-001",
		RoundID:         101,
		TermID:          202601,
		StudentID:       10001,
		CourseID:        20001,
		TeachingClassID: 30001,
		Credits:         enrollment.Credit(35),
		Source:          enrollment.ApplicationSourceWeb,
		State:           enrollment.ApplicationStateSelected,
		AppliedAt:       now,
		CompletedAt:     now.Add(time.Second),
	}
}

func TestNormalizeSelectionResultsDeduplicatesIdenticalEvent(t *testing.T) {
	result := validBatchSelectionResult()
	duplicate := *result
	got, err := normalizeSelectionResults(
		[]*enrollment.SelectionResult{result, &duplicate},
	)
	if err != nil {
		t.Fatalf("normalizeSelectionResults() error = %v", err)
	}
	if len(got) != 1 || got[0] != result {
		t.Fatalf("normalized results = %#v, want first result only", got)
	}
}

func TestNormalizeSelectionResultsRejectsConflictingEvent(t *testing.T) {
	result := validBatchSelectionResult()
	conflict := *result
	conflict.RequestID = "different-request"
	_, err := normalizeSelectionResults(
		[]*enrollment.SelectionResult{result, &conflict},
	)
	if !errors.Is(err, enrollment.ErrIdempotencyConflict) {
		t.Fatalf("normalizeSelectionResults() error = %v, want idempotency conflict", err)
	}
}

func TestNormalizeSelectionResultsRejectsNil(t *testing.T) {
	_, err := normalizeSelectionResults([]*enrollment.SelectionResult{nil})
	if !errors.Is(err, enrollment.ErrInvalidParams) {
		t.Fatalf("normalizeSelectionResults(nil) error = %v, want invalid params", err)
	}
}
