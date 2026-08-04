package danmakuapp

import (
	"context"
	"errors"

	"github.com/yywencs/courseforge/internal/danmaku/domain"
)

// Repository 定义弹幕发布与历史查询用例需要的持久化能力。
type Repository interface {
	Insert(context.Context, *danmaku.Danmaku) error
	GetByClientMessage(context.Context, uint64, uint64, string) (*danmaku.Danmaku, error)
	ListVisibleSegment(context.Context, uint64, danmaku.HistorySegment) ([]danmaku.Danmaku, error)
}

// VideoReader 定义课程视频状态和时长的只读查询能力。
type VideoReader interface {
	GetVideo(context.Context, uint64) (*danmaku.VideoTarget, error)
}

// SegmentLoader 从权威数据源读取一个历史弹幕分段。
type SegmentLoader func(context.Context) ([]danmaku.Danmaku, error)

// SegmentCache 定义历史弹幕分段的读穿缓存和失效能力。
type SegmentCache interface {
	GetOrLoad(context.Context, uint64, danmaku.HistorySegment, SegmentLoader) ([]danmaku.Danmaku, error)
	Invalidate(context.Context, uint64, danmaku.HistorySegment) error
}

// PublishCommand 封装一次已认证学生的弹幕发布请求。
type PublishCommand struct {
	VideoID         uint64
	StudentID       uint64
	ClientMessageID string
	VideoTimeMS     uint64
	Content         string
}

// HistoryQuery 指定一个视频的固定历史弹幕分段。
type HistoryQuery struct {
	VideoID      uint64
	SegmentIndex uint64
}

// HistoryPage 返回一个完整的固定时间分段及其可见弹幕。
type HistoryPage struct {
	Segment danmaku.HistorySegment
	Items   []danmaku.Danmaku
}

// Service 编排视频校验、弹幕持久化、历史查询和幂等冲突处理。
type Service struct {
	repository Repository
	videos     VideoReader
	cache      SegmentCache
}

// NewService 创建弹幕应用服务。
func NewService(repository Repository, videos VideoReader, options ...ServiceOption) *Service {
	service := &Service{repository: repository, videos: videos}
	for _, option := range options {
		option(service)
	}
	return service
}

// ServiceOption 配置弹幕应用服务的可选依赖。
type ServiceOption func(*Service)

// WithSegmentCache 为历史弹幕查询配置分段缓存。
func WithSegmentCache(cache SegmentCache) ServiceOption {
	return func(service *Service) { service.cache = cache }
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
		s.invalidateSegment(ctx, *item)
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
	s.invalidateSegment(ctx, *existing)
	return existing, nil
}

func (s *Service) invalidateSegment(ctx context.Context, item danmaku.Danmaku) {
	if s.cache == nil {
		return
	}
	segment, err := danmaku.HistorySegmentAt(item.VideoTimeMS)
	if err != nil {
		return
	}
	// 缓存只优化读取；失效失败时由短 TTL 保证最终回源，不能反向影响已提交的发布。
	_ = s.cache.Invalidate(ctx, item.VideoID, segment)
}

// ListHistory 查询一个60秒分段内按播放位置稳定排序的可见弹幕。
func (s *Service) ListHistory(ctx context.Context, query HistoryQuery) (*HistoryPage, error) {
	if s == nil || s.repository == nil || s.videos == nil {
		return nil, errors.New("danmaku service is not configured")
	}
	historyQuery, err := danmaku.NewHistoryQuery(query.VideoID, query.SegmentIndex)
	if err != nil {
		return nil, err
	}
	video, err := s.videos.GetVideo(ctx, historyQuery.VideoID())
	if err != nil {
		return nil, err
	}
	if video == nil {
		return nil, danmaku.ErrVideoNotFound
	}
	if err := video.EnsureReadableHistory(historyQuery.VideoID(), historyQuery.Segment()); err != nil {
		return nil, err
	}
	if s.cache != nil {
		items, cacheErr := s.cache.GetOrLoad(
			ctx,
			historyQuery.VideoID(),
			historyQuery.Segment(),
			func(loadCtx context.Context) ([]danmaku.Danmaku, error) {
				return s.repository.ListVisibleSegment(
					loadCtx, historyQuery.VideoID(), historyQuery.Segment(),
				)
			},
		)
		if cacheErr == nil {
			return &HistoryPage{Segment: historyQuery.Segment(), Items: items}, nil
		}
		// 缓存适配器异常时直接回源，避免缓存成为历史查询的可用性依赖。
	}
	items, err := s.repository.ListVisibleSegment(
		ctx, historyQuery.VideoID(), historyQuery.Segment(),
	)
	if err != nil {
		return nil, err
	}
	return &HistoryPage{Segment: historyQuery.Segment(), Items: items}, nil
}
