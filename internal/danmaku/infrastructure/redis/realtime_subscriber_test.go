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
	platformmetrics "github.com/yywencs/courseforge/internal/platform/observability/metrics"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
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

type subscribeResult struct {
	subscription platformcache.PubSubSubscription
	err          error
}

type sequencedMessageSubscriberStub struct {
	mu      sync.Mutex
	results []subscribeResult
	calls   int
}

func (s *sequencedMessageSubscriberStub) Subscribe(
	_ context.Context,
	_ ...string,
) (platformcache.PubSubSubscription, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	index := s.calls
	s.calls++
	if index >= len(s.results) {
		return nil, errors.New("unexpected subscribe call")
	}
	return s.results[index].subscription, s.results[index].err
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

func TestRealtimeSubscriberReconnectsWithExponentialBackoff(t *testing.T) {
	attemptsBefore := prometheusMetricValue(t,
		platformmetrics.DanmakuSubscriberReconnectTotal.WithLabelValues("attempt"),
	)
	failuresBefore := prometheusMetricValue(t,
		platformmetrics.DanmakuSubscriberReconnectTotal.WithLabelValues("failure"),
	)
	successesBefore := prometheusMetricValue(t,
		platformmetrics.DanmakuSubscriberReconnectTotal.WithLabelValues("success"),
	)
	initial := newSubscriptionStub()
	reconnected := newSubscriptionStub()
	client := &sequencedMessageSubscriberStub{results: []subscribeResult{
		{subscription: initial},
		{err: errors.New("redis unavailable 1")},
		{err: errors.New("redis unavailable 2")},
		{subscription: reconnected},
	}}
	broadcaster := &broadcasterStub{records: make(chan broadcastRecord, 1)}
	subscriber := NewRealtimeSubscriber(client, broadcaster)
	waits := make(chan time.Duration, 3)
	subscriber.retryInitialDelay = 10 * time.Millisecond
	subscriber.retryMaxDelay = 30 * time.Millisecond
	subscriber.retryJitter = func(delay time.Duration) time.Duration {
		return delay * 3 / 4
	}
	subscriber.retryWait = func(ctx context.Context, delay time.Duration) bool {
		select {
		case waits <- delay:
			return true
		case <-ctx.Done():
			return false
		}
	}
	if err := subscriber.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	stopSubscriber(t, subscriber)

	close(initial.messages)
	for index, want := range []time.Duration{
		7500 * time.Microsecond,
		15 * time.Millisecond,
		22500 * time.Microsecond,
	} {
		select {
		case got := <-waits:
			if got != want {
				t.Fatalf("retry wait %d = %v, want %v", index+1, got, want)
			}
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for retry wait %d", index+1)
		}
	}

	payload, err := danmakurealtime.MarshalPublished(danmaku.Danmaku{
		ID: 90, VideoID: 9, CreateTime: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	reconnected.messages <- &redis.Message{
		Channel: RealtimePublishedChannel,
		Payload: string(payload),
	}
	select {
	case record := <-broadcaster.records:
		if record.videoID != 9 {
			t.Fatalf("video id = %d, want 9", record.videoID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for broadcast after reconnect")
	}
	if got := prometheusMetricValue(t,
		platformmetrics.DanmakuSubscriberReconnectTotal.WithLabelValues("attempt"),
	); got != attemptsBefore+3 {
		t.Fatalf("reconnect attempts = %v, want %v", got, attemptsBefore+3)
	}
	if got := prometheusMetricValue(t,
		platformmetrics.DanmakuSubscriberReconnectTotal.WithLabelValues("failure"),
	); got != failuresBefore+2 {
		t.Fatalf("reconnect failures = %v, want %v", got, failuresBefore+2)
	}
	if got := prometheusMetricValue(t,
		platformmetrics.DanmakuSubscriberReconnectTotal.WithLabelValues("success"),
	); got != successesBefore+1 {
		t.Fatalf("reconnect successes = %v, want %v", got, successesBefore+1)
	}

	select {
	case <-initial.closed:
	default:
		t.Fatal("interrupted subscription was not closed")
	}
}

func TestEqualJitterRealtimeSubscriberRetryStaysWithinHalfAndFullDelay(t *testing.T) {
	delay := 30 * time.Second
	for range 100 {
		got := equalJitterRealtimeSubscriberRetry(delay)
		if got < delay/2 || got > delay {
			t.Fatalf("jittered retry delay = %v, want within [%v, %v]", got, delay/2, delay)
		}
	}
}

func TestNextRealtimeSubscriberRetryDelayCapsAtMaximum(t *testing.T) {
	if got := nextRealtimeSubscriberRetryDelay(16*time.Second, 30*time.Second); got != 30*time.Second {
		t.Fatalf("next retry delay = %v, want 30s", got)
	}
	if got := nextRealtimeSubscriberRetryDelay(30*time.Second, 30*time.Second); got != 30*time.Second {
		t.Fatalf("capped retry delay = %v, want 30s", got)
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

func prometheusMetricValue(t *testing.T, metric prometheus.Metric) float64 {
	t.Helper()
	value := &dto.Metric{}
	if err := metric.Write(value); err != nil {
		t.Fatalf("write Prometheus metric: %v", err)
	}
	if value.Gauge != nil {
		return value.GetGauge().GetValue()
	}
	return value.GetCounter().GetValue()
}
