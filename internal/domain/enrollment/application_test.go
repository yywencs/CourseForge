package enrollment

import (
	"errors"
	"testing"
	"time"
)

func newTestApplication(t *testing.T) (*SelectionApplication, time.Time) {
	t.Helper()
	appliedAt := time.Date(2026, time.September, 1, 8, 0, 0, 0, time.Local)
	application, err := NewSelectionApplication(
		"application-001",
		validSelectionRequest(),
		appliedAt,
	)
	if err != nil {
		t.Fatalf("NewSelectionApplication() error = %v", err)
	}
	return application, appliedAt
}

// TestSelectionApplicationSelectedFlow 验证成功链路只能按
// created → reserved → selected 顺序迁移。
func TestSelectionApplicationSelectedFlow(t *testing.T) {
	application, appliedAt := newTestApplication(t)
	if application.State != ApplicationStateCreated {
		t.Fatalf("initial state = %q, want %q", application.State, ApplicationStateCreated)
	}

	if err := application.Reserve(); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if application.State != ApplicationStateReserved {
		t.Fatalf("reserved application = %#v", application)
	}

	completedAt := appliedAt.Add(time.Second)
	result, err := application.CompleteSelected(completedAt)
	if err != nil {
		t.Fatalf("CompleteSelected() error = %v", err)
	}
	if application.State != ApplicationStateSelected || application.CompletedAt == nil {
		t.Fatalf("completed application = %#v", application)
	}
	if result.State != ApplicationStateSelected ||
		result.ApplicationID != application.ApplicationID ||
		result.Failure != nil ||
		!result.CompletedAt.Equal(completedAt) {
		t.Fatalf("selection result = %#v", result)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result.Validate() error = %v", err)
	}
}

// TestSelectionApplicationRejectsInvalidTransitions 验证跳过预占和重复完成都会被拒绝。
func TestSelectionApplicationRejectsInvalidTransitions(t *testing.T) {
	application, appliedAt := newTestApplication(t)

	if _, err := application.CompleteSelected(appliedAt.Add(time.Second)); !errors.Is(err, ErrInvalidApplicationState) {
		t.Fatalf("complete before reserve error = %v, want %v", err, ErrInvalidApplicationState)
	}
	if err := application.Reserve(); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if err := application.Reserve(); !errors.Is(err, ErrInvalidApplicationState) {
		t.Fatalf("second Reserve() error = %v, want %v", err, ErrInvalidApplicationState)
	}
	if _, err := application.CompleteSelected(appliedAt.Add(time.Second)); err != nil {
		t.Fatalf("CompleteSelected() error = %v", err)
	}
	if _, err := application.CompleteSelected(appliedAt.Add(2 * time.Second)); !errors.Is(err, ErrInvalidApplicationState) {
		t.Fatalf("second completion error = %v, want %v", err, ErrInvalidApplicationState)
	}
}

// TestSelectionApplicationRejectedFlow 验证失败完成会保存稳定失败码。
func TestSelectionApplicationRejectedFlow(t *testing.T) {
	application, appliedAt := newTestApplication(t)
	if err := application.Reserve(); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	reason := FailureReason{
		Code:    FailureCodeScheduleConflict,
		Message: "与已选课程时间冲突",
	}
	result, err := application.CompleteRejected(
		reason,
		appliedAt.Add(time.Second),
	)
	if err != nil {
		t.Fatalf("CompleteRejected() error = %v", err)
	}
	if result.State != ApplicationStateRejected ||
		result.Failure == nil ||
		result.Failure.Code != FailureCodeScheduleConflict {
		t.Fatalf("rejected result = %#v", result)
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("result.Validate() error = %v", err)
	}
}

// TestSelectionApplicationCancel 验证预占申请可以取消。
func TestSelectionApplicationCancel(t *testing.T) {
	reason := FailureReason{Code: FailureCodeInternal, Message: "管理员取消教学班"}

	application, appliedAt := newTestApplication(t)
	if err := application.Reserve(); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	result, err := application.Cancel(reason, appliedAt.Add(time.Second))
	if err != nil {
		t.Fatalf("Cancel() error = %v", err)
	}
	if result.State != ApplicationStateCancelled || !result.State.Terminal() {
		t.Fatalf("cancelled result state = %q", result.State)
	}
}
