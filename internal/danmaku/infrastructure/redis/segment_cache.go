package danmakucache

import (
	"context"
	"errors"
	"fmt"
	"time"

	danmakuapp "github.com/yywencs/courseforge/internal/danmaku/application"
	"github.com/yywencs/courseforge/internal/danmaku/domain"
	platformcache "github.com/yywencs/courseforge/internal/platform/cache"
)

const (
	// DefaultSegmentTTL 是历史弹幕分段的默认缓存时间。
	DefaultSegmentTTL = 5 * time.Minute
	keyVersion        = "v1"
)

var _ danmakuapp.SegmentCache = (*SegmentCache)(nil)

// SegmentCache 使用项目通用缓存封装保存历史弹幕分段。
type SegmentCache struct {
	cache *platformcache.Cache
	ttl   time.Duration
}

// NewSegmentCache 创建历史弹幕 Redis 分段缓存。
func NewSegmentCache(cache *platformcache.Cache, ttl time.Duration) *SegmentCache {
	if ttl <= 0 {
		ttl = DefaultSegmentTTL
	}
	return &SegmentCache{cache: cache, ttl: ttl}
}

// GetOrLoad 优先读取缓存；未命中时合并同一进程内的并发请求并从数据源加载。
func (c *SegmentCache) GetOrLoad(
	ctx context.Context,
	videoID uint64,
	segment danmaku.HistorySegment,
	loader danmakuapp.SegmentLoader,
) ([]danmaku.Danmaku, error) {
	if c == nil || c.cache == nil || loader == nil {
		return nil, errors.New("danmaku segment cache is not configured")
	}

	items := make([]danmaku.Danmaku, 0)
	loaded := false
	var loadErr error
	err := c.cache.Once(&platformcache.Item{
		Ctx: ctx, Key: segmentKey(videoID, segment.Index()), Value: &items, TTL: c.ttl,
		Do: func(*platformcache.Item) (interface{}, error) {
			items, loadErr = loader(ctx)
			loaded = loadErr == nil
			return items, loadErr
		},
	})
	if err == nil {
		return items, nil
	}
	// 数据源读取已经成功时，即使序列化或写缓存失败也返回权威数据。
	if loaded {
		return items, nil
	}
	if loadErr != nil {
		return nil, loadErr
	}
	return nil, err
}

// Invalidate 删除指定分段；重复删除不存在的缓存同样成功。
func (c *SegmentCache) Invalidate(
	ctx context.Context,
	videoID uint64,
	segment danmaku.HistorySegment,
) error {
	if c == nil || c.cache == nil {
		return errors.New("danmaku segment cache is not configured")
	}
	return c.cache.Delete(ctx, segmentKey(videoID, segment.Index()))
}

func segmentKey(videoID, segmentIndex uint64) string {
	return fmt.Sprintf("danmaku:segment:%s:%d:%d", keyVersion, videoID, segmentIndex)
}
