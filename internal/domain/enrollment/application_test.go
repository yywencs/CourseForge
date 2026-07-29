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
// created → reserved → processing → selected 顺序迁移。
func TestSelectionApplicationSelectedFlow(t *testing.T) {
	application, appliedAt := newTestApplication(t)
	if application.State != ApplicationStateCreated {
		t.Fatalf("initial state = %q, want %q", application.State, ApplicationStateCreated)
	}

	if err := application.Reserve(); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	claimedAt := appliedAt.Add(time.Second)
	if err := application.Claim("owner-1", claimedAt); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if application.State != ApplicationStateProcessing ||
		application.Owner != "owner-1" ||
		application.ProcessingAt == nil ||
		!application.ProcessingAt.Equal(claimedAt) {
		t.Fatalf("claimed application = %#v", application)
	}

	completedAt := claimedAt.Add(time.Second)
	result, err := application.CompleteSelected("owner-1", completedAt)
	if err != nil {
		t.Fatalf("CompleteSelected() error = %v", err)
	}
	if application.State != ApplicationStateSelected ||
		application.Owner != "" ||
		application.ProcessingAt != nil ||
		application.CompletedAt == nil {
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

// TestSelectionApplicationRejectsInvalidTransitions 验证跳过预占、重复抢占和旧Owner提交都会被拒绝。
func TestSelectionApplicationRejectsInvalidTransitions(t *testing.T) {
	application, appliedAt := newTestApplication(t)

	if err := application.Claim("owner-1", appliedAt.Add(time.Second)); !errors.Is(err, ErrInvalidApplicationState) {
		t.Fatalf("claim before reserve error = %v, want %v", err, ErrInvalidApplicationState)
	}
	if err := application.Reserve(); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if err := application.Reserve(); !errors.Is(err, ErrInvalidApplicationState) {
		t.Fatalf("second Reserve() error = %v, want %v", err, ErrInvalidApplicationState)
	}
	if err := application.Claim("owner-1", appliedAt.Add(time.Second)); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}
	if err := application.Claim("owner-2", appliedAt.Add(2*time.Second)); !errors.Is(err, ErrApplicationInProgress) {
		t.Fatalf("second Claim() error = %v, want %v", err, ErrApplicationInProgress)
	}
	if _, err := application.CompleteSelected("owner-2", appliedAt.Add(3*time.Second)); !errors.Is(err, ErrClaimOwnerMismatch) {
		t.Fatalf("stale owner completion error = %v, want %v", err, ErrClaimOwnerMismatch)
	}
}

// TestSelectionApplicationReleaseClaim 验证只有当前Owner可以释放处理权并回到reserved状态。
func TestSelectionApplicationReleaseClaim(t *testing.T) {
	application, appliedAt := newTestApplication(t)
	if err := application.Reserve(); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if err := application.Claim("owner-1", appliedAt.Add(time.Second)); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}

	if err := application.ReleaseClaim("owner-2"); !errors.Is(err, ErrClaimOwnerMismatch) {
		t.Fatalf("ReleaseClaim(stale owner) error = %v, want %v", err, ErrClaimOwnerMismatch)
	}
	if err := application.ReleaseClaim("owner-1"); err != nil {
		t.Fatalf("ReleaseClaim() error = %v", err)
	}
	if application.State != ApplicationStateReserved ||
		application.Owner != "" ||
		application.ProcessingAt != nil {
		t.Fatalf("released application = %#v", application)
	}
}

// TestSelectionApplicationRejectedFlow 验证失败完成会保存稳定失败码并清理处理权。
func TestSelectionApplicationRejectedFlow(t *testing.T) {
	application, appliedAt := newTestApplication(t)
	if err := application.Reserve(); err != nil {
		t.Fatalf("Reserve() error = %v", err)
	}
	if err := application.Claim("owner-1", appliedAt.Add(time.Second)); err != nil {
		t.Fatalf("Claim() error = %v", err)
	}

	reason := FailureReason{
		Code:    FailureCodeScheduleConflict,
		Message: "与已选课程时间冲突",
	}
	result, err := application.CompleteRejected(
		"owner-1",
		reason,
		appliedAt.Add(2*time.Second),
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

// TestSelectionApplicationCancel 验证未处理申请可以取消，处理中申请不能被无Owner直接取消。
func TestSelectionApplicationCancel(t *testing.T) {
	reason := FailureReason{Code: FailureCodeInternal, Message: "管理员取消教学班"}

	t.Run("reserved application", func(t *testing.T) {
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
	})

	t.Run("processing application", func(t *testing.T) {
		application, appliedAt := newTestApplication(t)
		if err := application.Reserve(); err != nil {
			t.Fatalf("Reserve() error = %v", err)
		}
		if err := application.Claim("owner-1", appliedAt.Add(time.Second)); err != nil {
			t.Fatalf("Claim() error = %v", err)
		}
		if _, err := application.Cancel(reason, appliedAt.Add(2*time.Second)); !errors.Is(err, ErrApplicationInProgress) {
			t.Fatalf("Cancel(processing) error = %v, want %v", err, ErrApplicationInProgress)
		}
	})
}
