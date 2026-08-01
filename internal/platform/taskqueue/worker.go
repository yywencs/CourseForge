package taskqueue

import (
	"context"
	"errors"
	"fmt"
	"time"

	"prizeforge/internal/platform/config"
	"prizeforge/internal/platform/observability/logger"
	"prizeforge/internal/platform/observability/metrics"

	"github.com/hibiken/asynq"
)

// AsynqWorker 封装 Asynq 任务队列服务端。
type AsynqWorker struct {
	server    *asynq.Server
	mux       *asynq.ServeMux
	scheduler *asynq.Scheduler
	inspector *asynq.Inspector
	queues    []string
}

// ScheduledHandler binds a task type to its handler and optional cron schedule.
// It keeps the generic task runtime independent from bounded-context packages.
type ScheduledHandler struct {
	taskType string
	schedule string
	handler  func(context.Context, *asynq.Task) error
}

func NewScheduledHandler(
	taskType string,
	schedule string,
	handler func(context.Context, *asynq.Task) error,
) ScheduledHandler {
	return ScheduledHandler{taskType: taskType, schedule: schedule, handler: handler}
}

// NewAsynqWorker 创建一个 Asynq worker 服务端，并注册所有任务处理器。
func NewAsynqWorker(
	cfg *config.AsynqConfig,
	handlers ...ScheduledHandler,
) *AsynqWorker {
	redisOpt := asynq.RedisClientOpt{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	}

	server := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: cfg.Concurrency,
			Queues: map[string]int{
				"default": 3,
				"low":     1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				result := asynqTaskResult(err)
				metrics.IncAsynqTask(task.Type(), result)
				logger.Error("process task failed", "type", task.Type(), "payload", string(task.Payload()), "err", err)
			}),
		},
	)

	mux := asynq.NewServeMux()
	scheduler := asynq.NewScheduler(redisOpt, &asynq.SchedulerOpts{})
	inspector := asynq.NewInspector(redisOpt)
	queues := []string{
		"default",
		"low",
	}

	for _, registration := range handlers {
		if registration.taskType == "" || registration.handler == nil {
			continue
		}
		mux.HandleFunc(
			registration.taskType,
			wrapAsynqHandler(registration.taskType, registration.handler),
		)
		if registration.schedule == "" {
			continue
		}
		if _, err := scheduler.Register(
			registration.schedule,
			asynq.NewTask(registration.taskType, nil),
		); err != nil {
			logger.Error(
				"register task scheduler failed",
				"type", registration.taskType,
				"schedule", registration.schedule,
				"err", err,
			)
		}
	}

	return &AsynqWorker{
		server:    server,
		mux:       mux,
		scheduler: scheduler,
		inspector: inspector,
		queues:    queues,
	}
}

// Start 启动 Asynq worker。
func (w *AsynqWorker) Start(ctx context.Context) error {
	logger.Info("Asynq Worker starting...")
	w.startQueueMetricsCollector(ctx)

	if err := w.scheduler.Start(); err != nil {
		return fmt.Errorf("scheduler start failed: %w", err)
	}

	if err := w.server.Start(w.mux); err != nil {
		return fmt.Errorf("asynq server start failed: %w", err)
	}
	return nil
}

// Shutdown 优雅停止 Asynq worker。
func (w *AsynqWorker) Shutdown() {
	logger.Info("Asynq Worker stopping...")
	w.scheduler.Shutdown()
	w.server.Shutdown()
	if w.inspector != nil {
		_ = w.inspector.Close()
	}
}

// wrapAsynqHandler 包装任务处理函数，添加耗时和成功指标。
func wrapAsynqHandler(taskType string, handler func(context.Context, *asynq.Task) error) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, task *asynq.Task) error {
		start := time.Now()
		err := handler(ctx, task)
		metrics.ObserveAsynqTaskDuration(taskType, time.Since(start))
		if err == nil {
			metrics.IncAsynqTask(taskType, "success")
		}
		return err
	}
}

// startQueueMetricsCollector 启动一个后台 goroutine，定期采集队列指标。
func (w *AsynqWorker) startQueueMetricsCollector(ctx context.Context) {
	go func() {
		w.collectQueueMetrics()

		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				w.collectQueueMetrics()
			case <-ctx.Done():
				return
			}
		}
	}()
}

// collectQueueMetrics 采集各队列的任务积压、重试、调度数量并上报到 Prometheus。
func (w *AsynqWorker) collectQueueMetrics() {
	if w.inspector == nil {
		return
	}

	queueSet := make(map[string]struct{}, len(w.queues))
	for _, queue := range w.queues {
		queueSet[queue] = struct{}{}
	}

	queues, err := w.inspector.Queues()
	if err == nil {
		for _, queue := range queues {
			queueSet[queue] = struct{}{}
		}
	}

	for queue := range queueSet {
		info, err := w.inspector.GetQueueInfo(queue)
		if err != nil {
			if errors.Is(err, asynq.ErrQueueNotFound) {
				metrics.SetAsynqQueueStats(queue, 0, 0, 0)
			}
			continue
		}
		metrics.SetAsynqQueueStats(queue, info.Size, info.Retry, info.Scheduled)
	}
}

// asynqTaskResult 根据任务处理错误返回结果标签（success/skip_retry/error），用于指标统计。
func asynqTaskResult(err error) string {
	if errors.Is(err, asynq.SkipRetry) {
		return "skip_retry"
	}
	return "error"
}
