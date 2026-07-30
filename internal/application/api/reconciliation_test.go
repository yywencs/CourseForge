package api

import (
	"context"
	"errors"
	"testing"
	"time"

	"prizeforge/internal/domain/enrollment"
)

type reconciliationRepairRepository struct {
	repairs   []*enrollment.ProjectionRepair
	completed []string
	failed    []string
}

func (f *reconciliationRepairRepository) QueryPendingProjectionRepairs(
	context.Context, time.Time, int,
) ([]*enrollment.ProjectionRepair, error) {
	return f.repairs, nil
}

func (f *reconciliationRepairRepository) MarkProjectionRepairCompleted(
	_ context.Context, repairID string, _ time.Time,
) error {
	f.completed = append(f.completed, repairID)
	return nil
}

func (f *reconciliationRepairRepository) MarkProjectionRepairFailed(
	_ context.Context, repairID string, _ time.Time, _ string,
) error {
	f.failed = append(f.failed, repairID)
	return nil
}

func (f *reconciliationRepairRepository) CountPendingProjectionRepairs(context.Context) (int64, error) {
	return int64(len(f.repairs) - len(f.completed)), nil
}

func TestProjectionReconciliationRetriesFailedRepair(t *testing.T) {
	repairs := &reconciliationRepairRepository{repairs: []*enrollment.ProjectionRepair{{
		RepairID:   "drop:enrollment-1",
		Enrollment: testEnrolledRecord(),
	}}}
	repairs.repairs[0].Enrollment.State = enrollment.EnrollmentStateDropped
	droppedAt := time.Now()
	repairs.repairs[0].Enrollment.DroppedAt = &droppedAt
	projection := &fakeProjectionRepository{err: errors.New("redis unavailable")}
	usecase := NewProjectionReconciliationUsecase(repairs, projection)

	if err := usecase.RepairBatch(context.Background(), 10); err != nil {
		t.Fatalf("RepairBatch() error = %v", err)
	}
	if len(repairs.failed) != 1 || repairs.failed[0] != "drop:enrollment-1" {
		t.Fatalf("failed repairs = %v", repairs.failed)
	}
}
