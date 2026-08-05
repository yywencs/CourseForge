package realtime

import (
	"testing"
	"time"

	"github.com/yywencs/courseforge/internal/danmaku/domain"
)

func TestPublishedVideoIDValidatesPublishedFrame(t *testing.T) {
	payload, err := MarshalPublished(danmaku.Danmaku{
		ID: 88, VideoID: 7, CreateTime: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	videoID, err := PublishedVideoID(payload)
	if err != nil {
		t.Fatal(err)
	}
	if videoID != 7 {
		t.Fatalf("video id = %d, want 7", videoID)
	}
}

func TestPublishedVideoIDRejectsMalformedFrame(t *testing.T) {
	if _, err := PublishedVideoID([]byte("not-protobuf")); err == nil {
		t.Fatal("PublishedVideoID() error = nil, want malformed frame error")
	}
}
