package danmakuredis

import (
	"testing"
	"time"

	"github.com/yywencs/courseforge/internal/danmaku/domain"
)

func TestSegmentCacheKeyIncludesVideoAndSegment(t *testing.T) {
	segment, err := danmaku.NewHistorySegment(3)
	if err != nil {
		t.Fatal(err)
	}
	key := segmentKey(7, segment.Index())
	if key != "danmaku:segment:v1:7:3" {
		t.Fatalf("key = %q", key)
	}
}

func TestNewSegmentCacheAppliesDefaultTTL(t *testing.T) {
	segmentCache := NewSegmentCache(nil, 0)
	if segmentCache.ttl != DefaultSegmentTTL {
		t.Fatalf("ttl = %s", segmentCache.ttl)
	}
	custom := NewSegmentCache(nil, time.Minute)
	if custom.ttl != time.Minute {
		t.Fatalf("custom ttl = %s", custom.ttl)
	}
}
