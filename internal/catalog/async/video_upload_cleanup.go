package catalogasync

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
)

type videoUploadCleaner interface {
	CleanupExpiredCourseVideoUploads(context.Context, time.Time, int) error
}

// VideoUploadCleanupJob 周期扫描已过签名有效期和安全缓冲期的上传对象。
type VideoUploadCleanupJob struct {
	service   videoUploadCleaner
	batchSize int
	grace     time.Duration
	now       func() time.Time
}

func NewVideoUploadCleanupJob(service videoUploadCleaner, batchSize int, grace time.Duration) *VideoUploadCleanupJob {
	if batchSize <= 0 {
		batchSize = 100
	}
	if grace <= 0 {
		grace = time.Hour
	}
	return &VideoUploadCleanupJob{
		service: service, batchSize: batchSize, grace: grace, now: time.Now,
	}
}

func (j *VideoUploadCleanupJob) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	if j == nil || j.service == nil {
		return nil
	}
	cleanupBefore := j.now().Add(-j.grace)
	return j.service.CleanupExpiredCourseVideoUploads(ctx, cleanupBefore, j.batchSize)
}
