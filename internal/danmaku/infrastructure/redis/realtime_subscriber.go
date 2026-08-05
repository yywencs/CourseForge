package danmakuredis

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"time"

	danmakurealtime "github.com/yywencs/courseforge/internal/danmaku/infrastructure/realtime"
	platformcache "github.com/yywencs/courseforge/internal/platform/cache"
	"github.com/yywencs/courseforge/internal/platform/observability/logger"
	"github.com/yywencs/courseforge/internal/platform/observability/metrics"
)

var errRealtimeSubscriberNotConfigured = errors.New("realtime subscriber is not configured")

const (
	defaultSubscriberRetryInitialDelay = time.Second
	defaultSubscriberRetryMaxDelay     = 30 * time.Second
)

// MessageSubscriber 定义 Redis 实时订阅器所需的最小客户端能力。
type MessageSubscriber interface {
	Subscribe(context.Context, ...string) (platformcache.PubSubSubscription, error)
}

// RealtimeBroadcaster 将已经校验的实时帧提交给当前实例。
type RealtimeBroadcaster interface {
	Broadcast(videoID uint64, payload []byte) error
}

// RealtimeSubscriber 将 Redis 中的实时弹幕转发给当前实例的连接。
type RealtimeSubscriber struct {
	client            MessageSubscriber
	broadcaster       RealtimeBroadcaster
	retryWait         func(context.Context, time.Duration) bool
	retryJitter       func(time.Duration) time.Duration
	retryInitialDelay time.Duration
	retryMaxDelay     time.Duration

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewRealtimeSubscriber 创建 Redis Pub/Sub 实时弹幕订阅器。
func NewRealtimeSubscriber(
	client MessageSubscriber,
	broadcaster RealtimeBroadcaster,
) *RealtimeSubscriber {
	return &RealtimeSubscriber{
		client:            client,
		broadcaster:       broadcaster,
		retryWait:         waitRealtimeSubscriberRetry,
		retryJitter:       equalJitterRealtimeSubscriberRetry,
		retryInitialDelay: defaultSubscriberRetryInitialDelay,
		retryMaxDelay:     defaultSubscriberRetryMaxDelay,
	}
}

// Start 使用 ctx 完成订阅握手后启动后台消费；消费生命周期由 Stop 管理。
// 重复启动不会创建额外订阅。
func (s *RealtimeSubscriber) Start(ctx context.Context) error {
	if s == nil || s.client == nil || s.broadcaster == nil {
		return errRealtimeSubscriberNotConfigured
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cancel != nil {
		return nil
	}
	subscription, err := s.client.Subscribe(ctx, RealtimePublishedChannel)
	if err != nil {
		return err
	}
	// 服务退出时需要先停止 HTTP 接流量，再停止订阅，不能让启动 ctx 的取消
	// 提前打乱显式的优雅关闭顺序。
	runCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	done := make(chan struct{})
	s.cancel = cancel
	s.done = done
	go s.run(runCtx, subscription, done)
	return nil
}

// Stop 停止消费、关闭独占的 Pub/Sub 连接，并等待后台协程退出。
func (s *RealtimeSubscriber) Stop(ctx context.Context) error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	cancel, done := s.cancel, s.done
	s.mu.Unlock()
	if cancel == nil || done == nil {
		return nil
	}
	cancel()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// run 管理订阅连接的完整生命周期。消息 Channel 异常关闭后，它按照指数退避
// 重新订阅；一次订阅握手成功后重置退避，避免 Redis 恢复后仍等待过长时间。
func (s *RealtimeSubscriber) run(
	ctx context.Context,
	subscription platformcache.PubSubSubscription,
	done chan<- struct{},
) {
	defer close(done)
	retryDelay := s.retryInitialDelayValue()

	for {
		s.consume(ctx, subscription)
		s.closeSubscription(subscription)
		if ctx.Err() != nil {
			return
		}

		warnRealtimeSubscriber("realtime danmaku subscription interrupted")
		for {
			waitDelay := s.jitterRetryDelay(retryDelay)
			if !s.waitRetry(ctx, waitDelay) {
				return
			}
			metrics.IncDanmakuSubscriberReconnect("attempt")
			next, err := s.client.Subscribe(ctx, RealtimePublishedChannel)
			if err != nil {
				metrics.IncDanmakuSubscriberReconnect("failure")
				warnRealtimeSubscriber(
					"reconnect realtime danmaku subscription failed",
					"retry_backoff", retryDelay,
					"retry_wait", waitDelay,
					"error", err,
				)
				retryDelay = nextRealtimeSubscriberRetryDelay(
					retryDelay,
					s.retryMaxDelayValue(),
				)
				continue
			}

			subscription = next
			metrics.IncDanmakuSubscriberReconnect("success")
			retryDelay = s.retryInitialDelayValue()
			infoRealtimeSubscriber("realtime danmaku subscription reconnected")
			break
		}
	}
}

// consume 持续消费单次 Redis Pub/Sub 订阅，并把合法消息转发给本机 Hub。
// 当前订阅关闭或 Subscriber 开始退出时返回，由 run 决定是否重连。
func (s *RealtimeSubscriber) consume(
	ctx context.Context,
	subscription platformcache.PubSubSubscription,
) {
	messages := subscription.Channel()
	for {
		select {
		case <-ctx.Done():
			// Stop 取消消费上下文后，从这里退出并执行上面的订阅清理逻辑。
			return
		case message, ok := <-messages:
			if !ok {
				// go-redis 关闭消息 channel 表示该订阅已经无法继续消费。
				return
			}
			if message == nil || message.Channel != RealtimePublishedChannel {
				// 只接受约定频道，避免未来复用订阅连接时发生错误路由。
				continue
			}
			payload := []byte(message.Payload)
			videoID, err := danmakurealtime.PublishedVideoID(payload)
			if err != nil {
				warnRealtimeSubscriber("discard invalid realtime danmaku message", "error", err)
				continue
			}
			if err := s.broadcaster.Broadcast(videoID, payload); err != nil {
				warnRealtimeSubscriber(
					"broadcast subscribed realtime danmaku failed",
					"video_id", videoID,
					"error", err,
				)
			}
		}
	}
}

func (s *RealtimeSubscriber) closeSubscription(subscription platformcache.PubSubSubscription) {
	if subscription == nil {
		return
	}
	if err := subscription.Close(); err != nil {
		warnRealtimeSubscriber("close realtime danmaku subscription failed", "error", err)
	}
}

func (s *RealtimeSubscriber) retryInitialDelayValue() time.Duration {
	if s.retryInitialDelay > 0 {
		return s.retryInitialDelay
	}
	return defaultSubscriberRetryInitialDelay
}

func (s *RealtimeSubscriber) retryMaxDelayValue() time.Duration {
	maximum := s.retryMaxDelay
	if maximum <= 0 {
		maximum = defaultSubscriberRetryMaxDelay
	}
	if initial := s.retryInitialDelayValue(); maximum < initial {
		return initial
	}
	return maximum
}

func (s *RealtimeSubscriber) waitRetry(ctx context.Context, delay time.Duration) bool {
	if s.retryWait != nil {
		return s.retryWait(ctx, delay)
	}
	return waitRealtimeSubscriberRetry(ctx, delay)
}

func (s *RealtimeSubscriber) jitterRetryDelay(delay time.Duration) time.Duration {
	if s.retryJitter != nil {
		return s.retryJitter(delay)
	}
	return equalJitterRealtimeSubscriberRetry(delay)
}

func waitRealtimeSubscriberRetry(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}

// equalJitterRealtimeSubscriberRetry 将实际等待时间随机分布在基础退避的 50%～100%。
// 保留下限可以避免抽到接近零的等待，同时打散多个实例的重连时刻。
func equalJitterRealtimeSubscriberRetry(delay time.Duration) time.Duration {
	if delay <= 0 {
		return 0
	}
	minimum := delay / 2
	if minimum == 0 {
		return delay
	}
	spread := delay - minimum
	return minimum + time.Duration(rand.Int64N(int64(spread)+1))
}

func nextRealtimeSubscriberRetryDelay(current, maximum time.Duration) time.Duration {
	if current <= 0 {
		current = defaultSubscriberRetryInitialDelay
	}
	if maximum < current {
		return current
	}
	if current >= maximum/2 {
		return maximum
	}
	return current * 2
}

func warnRealtimeSubscriber(message string, fields ...interface{}) {
	if logger.Log != nil {
		logger.Warn(message, fields...)
	}
}

func infoRealtimeSubscriber(message string, fields ...interface{}) {
	if logger.Log != nil {
		logger.Info(message, fields...)
	}
}
