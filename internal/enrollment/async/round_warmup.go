package enrollmentasync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	enrollmentapp "github.com/yywencs/courseforge/internal/enrollment/application"
	domain "github.com/yywencs/courseforge/internal/enrollment/domain"

	"github.com/hibiken/asynq"
)

const TaskTypeRoundWarmup = "enrollment:round_warmup"

type roundWarmupPayload struct {
	RoundID uint64 `json:"round_id"`
}

type asynqEnqueuer interface {
	EnqueueContext(context.Context, *asynq.Task, ...asynq.Option) (*asynq.TaskInfo, error)
}

// RoundWarmupEnqueuer 将管理员的预热请求可靠地交给 Asynq worker。
type RoundWarmupEnqueuer struct {
	client asynqEnqueuer
}

func NewRoundWarmupEnqueuer(client asynqEnqueuer) *RoundWarmupEnqueuer {
	return &RoundWarmupEnqueuer{client: client}
}

func (e *RoundWarmupEnqueuer) Enqueue(ctx context.Context, roundID uint64) error {
	if e == nil || e.client == nil || roundID == 0 {
		return domain.ErrInvalidParams
	}
	payload, err := json.Marshal(roundWarmupPayload{RoundID: roundID})
	if err != nil {
		return err
	}
	_, err = e.client.EnqueueContext(
		ctx,
		asynq.NewTask(TaskTypeRoundWarmup, payload),
		asynq.Queue("default"),
		asynq.MaxRetry(10),
		asynq.Timeout(30*time.Minute),
		asynq.Unique(10*time.Minute),
	)
	if errors.Is(err, asynq.ErrDuplicateTask) {
		return enrollmentapp.ErrRoundWarmupRunning
	}
	return err
}

// RoundWarmupJob 消费单个轮次预热任务。
type RoundWarmupJob struct {
	service *enrollmentapp.RoundWarmupService
}

func NewRoundWarmupJob(service *enrollmentapp.RoundWarmupService) *RoundWarmupJob {
	return &RoundWarmupJob{service: service}
}

func (j *RoundWarmupJob) ProcessTask(ctx context.Context, task *asynq.Task) error {
	if j == nil || j.service == nil || task == nil {
		return domain.ErrInvalidParams
	}
	var payload roundWarmupPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil || payload.RoundID == 0 {
		return fmt.Errorf("解析轮次预热任务: %v: %w", domain.ErrInvalidParams, asynq.SkipRetry)
	}
	_, err := j.service.Warmup(ctx, payload.RoundID)
	if errors.Is(err, domain.ErrRoundAlreadyWarmed) {
		return nil
	}
	// 进程中断后任务可能先于旧 Redis 锁过期被重新投递。锁冲突必须继续重试，
	// 否则任务会被确认成功，而状态永久停留在 running。
	return err
}
