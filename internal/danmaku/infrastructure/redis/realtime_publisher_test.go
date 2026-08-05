package danmakuredis

import (
	"context"
	"errors"
	"testing"
	"time"

	danmakuv1 "github.com/yywencs/courseforge/gen/courseforge/danmaku/v1"
	"github.com/yywencs/courseforge/internal/danmaku/domain"

	"google.golang.org/protobuf/proto"
)

type messagePublisherStub struct {
	channel string
	payload []byte
	err     error
	calls   int
}

func (p *messagePublisherStub) Publish(_ context.Context, channel string, payload []byte) error {
	p.calls++
	p.channel = channel
	p.payload = append([]byte(nil), payload...)
	return p.err
}

func TestRealtimePublisherPublishesPersistedDanmakuFrame(t *testing.T) {
	client := &messagePublisherStub{}
	createdAt := time.Date(2026, 8, 5, 9, 30, 0, 123000000, time.UTC)
	item := danmaku.Danmaku{
		ID: 88, VideoID: 7, StudentID: 1001,
		ClientMessageID: "ec40a0ec-572c-4af5-9067-65f702fa666c",
		VideoTimeMS:     12_300, Content: "关键内容",
		Status: danmaku.StatusVisible, CreateTime: createdAt,
	}

	NewRealtimePublisher(client).Publish(context.Background(), item)

	if client.calls != 1 || client.channel != RealtimePublishedChannel {
		t.Fatalf("publish calls = %d, channel = %q", client.calls, client.channel)
	}
	var frame danmakuv1.ServerFrame
	if err := proto.Unmarshal(client.payload, &frame); err != nil {
		t.Fatal(err)
	}
	published := frame.GetDanmakuPublished()
	if published == nil || published.GetId() != item.ID ||
		published.GetVideoId() != item.VideoID ||
		published.GetContent() != item.Content ||
		!published.GetCreateTime().AsTime().Equal(createdAt) {
		t.Fatalf("published frame = %v", published)
	}
}

func TestRealtimePublisherSkipsInvalidDanmaku(t *testing.T) {
	client := &messagePublisherStub{}
	NewRealtimePublisher(client).Publish(context.Background(), danmaku.Danmaku{ID: 88, VideoID: 7})
	if client.calls != 0 {
		t.Fatalf("publish calls = %d, want 0", client.calls)
	}
}

func TestRealtimePublisherContainsRedisFailure(t *testing.T) {
	client := &messagePublisherStub{err: errors.New("redis unavailable")}
	NewRealtimePublisher(client).Publish(context.Background(), danmaku.Danmaku{
		ID: 88, VideoID: 7, CreateTime: time.Now(),
	})
	if client.calls != 1 {
		t.Fatalf("publish calls = %d, want 1", client.calls)
	}
}
