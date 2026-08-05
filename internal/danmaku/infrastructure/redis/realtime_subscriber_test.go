package danmakuredis

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/yywencs/courseforge/internal/danmaku/domain"
	danmakurealtime "github.com/yywencs/courseforge/internal/danmaku/infrastructure/realtime"
	platformcache "github.com/yywencs/courseforge/internal/platform/cache"
)

type subscriptionStub struct {
	messages  chan *redis.Message
	closed    chan struct{}
	closeOnce sync.Once
}

func newSubscriptionStub() *subscriptionStub {
	return &subscriptionStub{
		messages: make(chan *redis.Message, 4),
		closed:   make(chan struct{}),
	}
}

func (s *subscriptionStub) Channel(...redis.ChannelOption) <-chan *redis.Message {
	return s.messages
}

func (s *subscriptionStub) Close() error {
	s.closeOnce.Do(func() { close(s.closed) })
	return nil
}

type messageSubscriberStub struct {
	subscription platformcache.PubSubSubscription
	err          error
	channels     []string
}

func (s *messageSubscriberStub) Subscribe(
	_ context.Context,
	channels ...string,
) (platformcache.PubSubSubscription, error) {
	s.channels = append([]string(nil), channels...)
	return s.subscription, s.err
}

type broadcastRecord struct {
	videoID uint64
	payload []byte
}

type broadcasterStub struct {
	records chan broadcastRecord
	err     error
}

func (b *broadcasterStub) Broadcast(videoID uint64, payload []byte) error {
	b.records <- broadcastRecord{videoID: videoID, payload: append([]byte(nil), payload...)}
	return b.err
}

func TestRealtimeSubscriberBroadcastsValidPublishedFrame(t *testing.T) {
	subscription := newSubscriptionStub()
	client := &messageSubscriberStub{subscription: subscription}
	broadcaster := &broadcasterStub{records: make(chan broadcastRecord, 1)}
	subscriber := NewRealtimeSubscriber(client, broadcaster)
	if err := subscriber.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	stopSubscriber(t, subscriber)

	payload, err := danmakurealtime.MarshalPublished(danmaku.Danmaku{
		ID: 88, VideoID: 7, CreateTime: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	subscription.messages <- &redis.Message{
		Channel: RealtimePublishedChannel,
		Payload: string(payload),
	}

	select {
	case record := <-broadcaster.records:
		if record.videoID != 7 || string(record.payload) != string(payload) {
			t.Fatalf("broadcast record = %#v", record)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for realtime broadcast")
	}
	if len(client.channels) != 1 || client.channels[0] != RealtimePublishedChannel {
		t.Fatalf("subscribed channels = %#v", client.channels)
	}
}

func TestRealtimeSubscriberDiscardsMalformedMessageAndContinues(t *testing.T) {
	subscription := newSubscriptionStub()
	broadcaster := &broadcasterStub{records: make(chan broadcastRecord, 1)}
	subscriber := NewRealtimeSubscriber(
		&messageSubscriberStub{subscription: subscription},
		broadcaster,
	)
	if err := subscriber.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	stopSubscriber(t, subscriber)

	subscription.messages <- &redis.Message{
		Channel: RealtimePublishedChannel,
		Payload: "malformed",
	}
	payload, err := danmakurealtime.MarshalPublished(danmaku.Danmaku{
		ID: 89, VideoID: 8, CreateTime: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	subscription.messages <- &redis.Message{
		Channel: RealtimePublishedChannel,
		Payload: string(payload),
	}

	select {
	case record := <-broadcaster.records:
		if record.videoID != 8 {
			t.Fatalf("video id = %d, want 8", record.videoID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for valid realtime broadcast")
	}
}

func TestRealtimeSubscriberReturnsSubscribeFailure(t *testing.T) {
	want := errors.New("redis unavailable")
	subscriber := NewRealtimeSubscriber(
		&messageSubscriberStub{err: want},
		&broadcasterStub{records: make(chan broadcastRecord, 1)},
	)
	if err := subscriber.Start(context.Background()); !errors.Is(err, want) {
		t.Fatalf("Start() error = %v, want %v", err, want)
	}
}

func TestRealtimeSubscriberStopClosesSubscription(t *testing.T) {
	subscription := newSubscriptionStub()
	subscriber := NewRealtimeSubscriber(
		&messageSubscriberStub{subscription: subscription},
		&broadcasterStub{records: make(chan broadcastRecord, 1)},
	)
	if err := subscriber.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := subscriber.Stop(ctx); err != nil {
		t.Fatal(err)
	}
	select {
	case <-subscription.closed:
	default:
		t.Fatal("subscription was not closed")
	}
	if err := subscriber.Stop(ctx); err != nil {
		t.Fatalf("second Stop() error = %v", err)
	}
}

func stopSubscriber(t *testing.T, subscriber *RealtimeSubscriber) {
	t.Helper()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		if err := subscriber.Stop(ctx); err != nil {
			t.Errorf("Stop() error = %v", err)
		}
	})
}
