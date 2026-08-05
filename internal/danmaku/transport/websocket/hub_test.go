package danmakuws

import (
	"context"
	"errors"
	"testing"
	"time"

	platformmetrics "github.com/yywencs/courseforge/internal/platform/observability/metrics"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

const hubTestTimeout = time.Second

func TestHubBroadcastsOnlyToRequestedVideo(t *testing.T) {
	hub := NewHub(4)
	hub.Start()
	t.Cleanup(func() { stopHub(t, hub) })

	first := newTestClientConn(1, 101, 1)
	second := newTestClientConn(2, 101, 1)
	otherVideo := newTestClientConn(3, 202, 1)

	firstForVideo, err := hub.Register(context.Background(), first)
	if err != nil || !firstForVideo {
		t.Fatalf("Register(first) = (%v, %v), want (true, nil)", firstForVideo, err)
	}
	firstForVideo, err = hub.Register(context.Background(), second)
	if err != nil || firstForVideo {
		t.Fatalf("Register(second) = (%v, %v), want (false, nil)", firstForVideo, err)
	}
	if _, err = hub.Register(context.Background(), otherVideo); err != nil {
		t.Fatalf("Register(otherVideo) error = %v", err)
	}

	payload := []byte("protobuf-payload")
	if err = hub.Broadcast(101, payload); err != nil {
		t.Fatalf("Broadcast() error = %v", err)
	}
	assertClientPayload(t, first, payload)
	assertClientPayload(t, second, payload)
	assertNoClientPayload(t, otherVideo)
}

func TestHubDoesNotBlockOnSlowConnection(t *testing.T) {
	queuedBefore := prometheusMetricValue(t,
		platformmetrics.WebSocketDeliveriesTotal.WithLabelValues("queued"),
	)
	droppedBefore := prometheusMetricValue(t,
		platformmetrics.WebSocketDeliveriesTotal.WithLabelValues("client_queue_full"),
	)
	hub := NewHub(4)
	hub.Start()
	t.Cleanup(func() { stopHub(t, hub) })

	slow := newTestClientConn(1, 101, 1)
	fast := newTestClientConn(2, 101, 1)
	if _, err := hub.Register(context.Background(), slow); err != nil {
		t.Fatal(err)
	}
	if _, err := hub.Register(context.Background(), fast); err != nil {
		t.Fatal(err)
	}
	slow.send <- []byte("already-full")

	payload := []byte("new-payload")
	if err := hub.Broadcast(101, payload); err != nil {
		t.Fatalf("Broadcast() error = %v", err)
	}
	assertClientPayload(t, fast, payload)
	if got := <-slow.send; string(got) != "already-full" {
		t.Fatalf("slow client payload = %q, want existing queued payload", got)
	}
	if got := prometheusMetricValue(t,
		platformmetrics.WebSocketDeliveriesTotal.WithLabelValues("queued"),
	); got != queuedBefore+1 {
		t.Fatalf("queued deliveries = %v, want %v", got, queuedBefore+1)
	}
	if got := prometheusMetricValue(t,
		platformmetrics.WebSocketDeliveriesTotal.WithLabelValues("client_queue_full"),
	); got != droppedBefore+1 {
		t.Fatalf("dropped deliveries = %v, want %v", got, droppedBefore+1)
	}
}

func TestHubUnregisterClosesConnection(t *testing.T) {
	activeBefore := prometheusMetricValue(t, platformmetrics.WebSocketActiveConnections)
	connectedBefore := prometheusMetricValue(t,
		platformmetrics.WebSocketConnectionsTotal.WithLabelValues("connected"),
	)
	closedBefore := prometheusMetricValue(t,
		platformmetrics.WebSocketConnectionsTotal.WithLabelValues("closed"),
	)
	hub := NewHub(4)
	hub.Start()
	t.Cleanup(func() { stopHub(t, hub) })
	client := newTestClientConn(1, 101, 1)
	if _, err := hub.Register(context.Background(), client); err != nil {
		t.Fatal(err)
	}
	if got := prometheusMetricValue(t, platformmetrics.WebSocketActiveConnections); got != activeBefore+1 {
		t.Fatalf("active connections after register = %v, want %v", got, activeBefore+1)
	}
	if got := prometheusMetricValue(t,
		platformmetrics.WebSocketConnectionsTotal.WithLabelValues("connected"),
	); got != connectedBefore+1 {
		t.Fatalf("connected events = %v, want %v", got, connectedBefore+1)
	}

	removed, videoEmpty, err := hub.Unregister(context.Background(), client)
	if err != nil || !removed || !videoEmpty {
		t.Fatalf(
			"Unregister() = (%v, %v, %v), want (true, true, nil)",
			removed,
			videoEmpty,
			err,
		)
	}
	assertClientClosed(t, client)
	if got := prometheusMetricValue(t, platformmetrics.WebSocketActiveConnections); got != activeBefore {
		t.Fatalf("active connections after unregister = %v, want %v", got, activeBefore)
	}
	if got := prometheusMetricValue(t,
		platformmetrics.WebSocketConnectionsTotal.WithLabelValues("closed"),
	); got != closedBefore+1 {
		t.Fatalf("closed events = %v, want %v", got, closedBefore+1)
	}

	if err := hub.Broadcast(101, []byte("after-unregister")); err != nil {
		t.Fatalf("Broadcast() error = %v", err)
	}
	assertNoClientPayload(t, client)
}

func TestHubBroadcastQueueIsBounded(t *testing.T) {
	acceptedBefore := prometheusMetricValue(t,
		platformmetrics.WebSocketBroadcastEventsTotal.WithLabelValues("accepted"),
	)
	fullBefore := prometheusMetricValue(t,
		platformmetrics.WebSocketBroadcastEventsTotal.WithLabelValues("queue_full"),
	)
	hub := NewHub(1)
	if err := hub.Broadcast(101, []byte("first")); err != nil {
		t.Fatalf("first Broadcast() error = %v", err)
	}
	if err := hub.Broadcast(101, []byte("second")); !errors.Is(err, ErrBroadcastQueueFull) {
		t.Fatalf("second Broadcast() error = %v, want ErrBroadcastQueueFull", err)
	}
	if got := prometheusMetricValue(t,
		platformmetrics.WebSocketBroadcastEventsTotal.WithLabelValues("accepted"),
	); got != acceptedBefore+1 {
		t.Fatalf("accepted broadcast events = %v, want %v", got, acceptedBefore+1)
	}
	if got := prometheusMetricValue(t,
		platformmetrics.WebSocketBroadcastEventsTotal.WithLabelValues("queue_full"),
	); got != fullBefore+1 {
		t.Fatalf("full broadcast events = %v, want %v", got, fullBefore+1)
	}
	stopHub(t, hub)
}

func TestHubStopDrainsConnectionsAndRejectsNewEvents(t *testing.T) {
	hub := NewHub(4)
	hub.Start()
	first := newTestClientConn(1, 101, 1)
	second := newTestClientConn(2, 202, 1)
	if _, err := hub.Register(context.Background(), first); err != nil {
		t.Fatal(err)
	}
	if _, err := hub.Register(context.Background(), second); err != nil {
		t.Fatal(err)
	}

	stopHub(t, hub)
	assertClientClosed(t, first)
	assertClientClosed(t, second)
	if counts := hub.connections.Counts(); len(counts) != 0 {
		t.Fatalf("connection counts after Stop() = %v, want empty", counts)
	}

	rejected := newTestClientConn(3, 303, 1)
	if _, err := hub.Register(context.Background(), rejected); !errors.Is(err, ErrHubStopped) {
		t.Fatalf("Register() after Stop() error = %v, want ErrHubStopped", err)
	}
	assertClientClosed(t, rejected)
	if err := hub.Broadcast(101, []byte("stopped")); !errors.Is(err, ErrHubStopped) {
		t.Fatalf("Broadcast() after Stop() error = %v, want ErrHubStopped", err)
	}
}

func assertClientPayload(t *testing.T, client *clientConn, want []byte) {
	t.Helper()
	select {
	case got := <-client.send:
		if string(got) != string(want) {
			t.Fatalf("client payload = %q, want %q", got, want)
		}
	case <-time.After(hubTestTimeout):
		t.Fatal("timed out waiting for client payload")
	}
}

func assertNoClientPayload(t *testing.T, client *clientConn) {
	t.Helper()
	select {
	case got := <-client.send:
		t.Fatalf("unexpected client payload %q", got)
	case <-time.After(20 * time.Millisecond):
	}
}

func assertClientClosed(t *testing.T, client *clientConn) {
	t.Helper()
	select {
	case <-client.done:
	case <-time.After(hubTestTimeout):
		t.Fatal("timed out waiting for client close signal")
	}
}

func stopHub(t *testing.T, hub *Hub) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), hubTestTimeout)
	defer cancel()
	if err := hub.Stop(ctx); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
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
