package danmakuapp

import (
	"context"
	"errors"

	"prizeforge/internal/danmaku/domain"
)

// Repository 定义弹幕发布用例需要的持久化能力。
type Repository interface {
	Insert(context.Context, *danmaku.Danmaku) error
	GetByClientMessage(context.Context, uint64, uint64, string) (*danmaku.Danmaku, error)
}

// VideoReader 定义课程视频状态和时长的只读查询能力。
type VideoReader interface {
	GetVideo(context.Context, uint64) (*danmaku.VideoTarget, error)
}

// PublishCommand 封装一次已认证学生的弹幕发布请求。
type PublishCommand struct {
	VideoID         uint64
	StudentID       uint64
	ClientMessageID string
	VideoTimeMS     uint64
	Content         string
}

// Service 编排视频校验、弹幕持久化和幂等冲突处理。
type Service struct {
	repository Repository
	videos     VideoReader
}

// NewService 创建弹幕应用服务。
func NewService(repository Repository, videos VideoReader) *Service {
	return &Service{repository: repository, videos: videos}
}

// Publish 同步持久化一条弹幕。
// 相同幂等键和相同载荷的重试返回原记录，载荷不一致时返回 ErrIdempotencyConflict。
func (s *Service) Publish(ctx context.Context, command PublishCommand) (*danmaku.Danmaku, error) {
	item, err := danmaku.New(
		command.VideoID,
		command.StudentID,
		command.ClientMessageID,
		command.VideoTimeMS,
		command.Content,
	)
	if err != nil {
		return nil, err
	}
	if s == nil || s.repository == nil || s.videos == nil {
		return nil, errors.New("danmaku service is not configured")
	}

	video, err := s.videos.GetVideo(ctx, item.VideoID)
	if err != nil {
		return nil, err
	}
	if err := video.EnsureAccepts(*item); err != nil {
		return nil, err
	}

	if err := s.repository.Insert(ctx, item); err == nil {
		return item, nil
	} else if !errors.Is(err, danmaku.ErrClientMessageExists) {
		return nil, err
	}

	existing, err := s.repository.GetByClientMessage(
		ctx,
		item.VideoID,
		item.StudentID,
		item.ClientMessageID,
	)
	if err != nil {
		return nil, err
	}
	if !existing.SameRequest(*item) {
		return nil, danmaku.ErrIdempotencyConflict
	}
	return existing, nil
}
