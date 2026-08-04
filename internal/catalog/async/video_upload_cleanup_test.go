package catalogasync

import (
	"context"
	"testing"
	"time"
)

type videoUploadCleanerStub struct {
	cleanupBefore time.Time
	batchSize     int
}

func (s *videoUploadCleanerStub) CleanupExpiredCourseVideoUploads(_ context.Context, cleanupBefore time.Time, batchSize int) error {
	s.cleanupBefore = cleanupBefore
	s.batchSize = batchSize
	return nil
}

func TestVideoUploadCleanupJobUsesGraceAndBatchSize(t *testing.T) {
	cleaner := &videoUploadCleanerStub{}
	job := NewVideoUploadCleanupJob(cleaner, 25, 90*time.Minute)
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	job.now = func() time.Time { return now }

	if err := job.ProcessTask(context.Background(), nil); err != nil {
		t.Fatalf("ProcessTask() error = %v", err)
	}
	if cleaner.batchSize != 25 || !cleaner.cleanupBefore.Equal(now.Add(-90*time.Minute)) {
		t.Fatalf("batch size = %d, cleanup before = %v", cleaner.batchSize, cleaner.cleanupBefore)
	}
}
