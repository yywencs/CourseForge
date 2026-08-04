package enrollmentasync

import (
	"context"

	api "github.com/yywencs/courseforge/internal/enrollment/application"

	"github.com/hibiken/asynq"
)

const TaskTypeWaitlistPromotion = "enrollment:waitlist_promotion"

// WaitlistPromotionJob 触发一批候补晋级；业务编排仍由 application 层负责。
type WaitlistPromotionJob struct {
	usecase   *api.WaitlistUsecase
	batchSize int
}

func NewWaitlistPromotionJob(usecase *api.WaitlistUsecase, batchSize int) *WaitlistPromotionJob {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &WaitlistPromotionJob{usecase: usecase, batchSize: batchSize}
}

func (j *WaitlistPromotionJob) ProcessTask(ctx context.Context, _ *asynq.Task) error {
	return j.usecase.PromoteBatch(ctx, j.batchSize)
}
