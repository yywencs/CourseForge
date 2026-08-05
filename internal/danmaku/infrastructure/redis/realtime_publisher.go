package danmakuredis

import (
	"context"

	danmakuapp "github.com/yywencs/courseforge/internal/danmaku/application"
	"github.com/yywencs/courseforge/internal/danmaku/domain"
	danmakurealtime "github.com/yywencs/courseforge/internal/danmaku/infrastructure/realtime"
	"github.com/yywencs/courseforge/internal/platform/observability/logger"
)

const (
	// RealtimePublishedChannel 是所有 API 实例共同使用的实时弹幕频道。
	RealtimePublishedChannel = "courseforge:danmaku:published:v1"
)

// MessagePublisher 定义 Redis 实时发布器所需的最小客户端能力。
type MessagePublisher interface {
	Publish(context.Context, string, []byte) error
}

// RealtimePublisher 将已持久化弹幕发布到 Redis Pub/Sub。
type RealtimePublisher struct {
	client MessagePublisher
}

var _ danmakuapp.RealtimePublisher = (*RealtimePublisher)(nil)

// NewRealtimePublisher 创建 Redis Pub/Sub 实时弹幕发布器。
func NewRealtimePublisher(client MessagePublisher) *RealtimePublisher {
	return &RealtimePublisher{client: client}
}

// Publish 编码弹幕并发布到所有实例共同订阅的 Redis 频道。
func (p *RealtimePublisher) Publish(ctx context.Context, item danmaku.Danmaku) {
	if p == nil || p.client == nil {
		return
	}
	payload, err := danmakurealtime.MarshalPublished(item)
	if err == nil {
		err = p.client.Publish(ctx, RealtimePublishedChannel, payload)
	}
	if err != nil {
		logRealtimePublishFailure(ctx, item, err)
	}
}

func logRealtimePublishFailure(ctx context.Context, item danmaku.Danmaku, err error) {
	if logger.Log == nil {
		return
	}
	logger.WarnContext(
		ctx,
		"publish realtime danmaku to redis failed",
		"danmaku_id", item.ID,
		"video_id", item.VideoID,
		"channel", RealtimePublishedChannel,
		"error", err,
	)
}
