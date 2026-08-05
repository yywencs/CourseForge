// Package realtime 提供实时弹幕在进程内和实例间传输时共用的消息编码。
package realtime

import (
	"errors"
	"fmt"

	danmakuv1 "github.com/yywencs/courseforge/gen/courseforge/danmaku/v1"
	"github.com/yywencs/courseforge/internal/danmaku/domain"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// MarshalPublished 将一条已持久化弹幕编码为 WebSocket ServerFrame。
func MarshalPublished(item danmaku.Danmaku) ([]byte, error) {
	if item.ID == 0 || item.VideoID == 0 || item.CreateTime.IsZero() {
		return nil, errors.New("persisted danmaku identity or create time is missing")
	}
	createdAt := timestamppb.New(item.CreateTime)
	if err := createdAt.CheckValid(); err != nil {
		return nil, err
	}
	return proto.Marshal(&danmakuv1.ServerFrame{
		Payload: &danmakuv1.ServerFrame_DanmakuPublished{
			DanmakuPublished: &danmakuv1.DanmakuPublished{
				Id: item.ID, VideoId: item.VideoID,
				ClientMessageId: item.ClientMessageID,
				VideoTimeMs:     item.VideoTimeMS,
				Content:         item.Content,
				CreateTime:      createdAt,
			},
		},
	})
}

// PublishedVideoID 校验实时发布帧并返回其目标视频 ID。
func PublishedVideoID(payload []byte) (uint64, error) {
	var frame danmakuv1.ServerFrame
	if err := proto.Unmarshal(payload, &frame); err != nil {
		return 0, fmt.Errorf("unmarshal realtime frame: %w", err)
	}
	published := frame.GetDanmakuPublished()
	if published == nil {
		return 0, errors.New("realtime frame is not a danmaku published event")
	}
	if published.GetId() == 0 || published.GetVideoId() == 0 ||
		published.GetCreateTime() == nil {
		return 0, errors.New("realtime danmaku identity or create time is missing")
	}
	if err := published.GetCreateTime().CheckValid(); err != nil {
		return 0, fmt.Errorf("invalid realtime danmaku create time: %w", err)
	}
	return published.GetVideoId(), nil
}
