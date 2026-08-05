package danmakuredis

import (
	"context"
	"errors"
	"sync"

	danmakurealtime "github.com/yywencs/courseforge/internal/danmaku/infrastructure/realtime"
	platformcache "github.com/yywencs/courseforge/internal/platform/cache"
	"github.com/yywencs/courseforge/internal/platform/observability/logger"
)

var errRealtimeSubscriberNotConfigured = errors.New("realtime subscriber is not configured")

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
	client      MessageSubscriber
	broadcaster RealtimeBroadcaster

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewRealtimeSubscriber 创建 Redis Pub/Sub 实时弹幕订阅器。
func NewRealtimeSubscriber(
	client MessageSubscriber,
	broadcaster RealtimeBroadcaster,
) *RealtimeSubscriber {
	return &RealtimeSubscriber{client: client, broadcaster: broadcaster}
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
	go s.consume(runCtx, subscription, done)
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

// consume 持续消费已经建立的 Redis Pub/Sub 订阅，并把合法消息转发给本机 Hub。
func (s *RealtimeSubscriber) consume(
	ctx context.Context,
	subscription platformcache.PubSubSubscription,
	done chan<- struct{},
) {
	defer close(done)
	defer func() {
		if err := subscription.Close(); err != nil {
			warnRealtimeSubscriber("close realtime danmaku subscription failed", "error", err)
		}
	}()
	messages := subscription.Channel()
	for {
		select {
		case <-ctx.Done():
			// Stop 取消消费上下文后，从这里退出并执行上面的订阅清理逻辑。
			return
		case message, ok := <-messages:
			if !ok {
				// go-redis 关闭消息 channel 表示该订阅已经无法继续消费。
				warnRealtimeSubscriber("realtime danmaku subscription channel closed")
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

func warnRealtimeSubscriber(message string, fields ...interface{}) {
	if logger.Log != nil {
		logger.Warn(message, fields...)
	}
}
