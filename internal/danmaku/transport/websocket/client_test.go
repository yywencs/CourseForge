package danmakuws

import (
	"context"
	"encoding/binary"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type fakeReadResult struct {
	messageType int
	payload     []byte
	err         error
}

type fakeFrame struct {
	messageType int
	payload     []byte
}

type fakeWebSocketConnection struct {
	reads    chan fakeReadResult
	writes   chan fakeFrame
	controls chan fakeFrame
	closed   chan struct{}

	mu             sync.Mutex
	readLimit      int64
	readDeadlines  []time.Time
	writeDeadlines []time.Time
	pongHandler    func(string) error
	closeOnce      sync.Once
}

func newFakeWebSocketConnection() *fakeWebSocketConnection {
	return &fakeWebSocketConnection{
		reads:    make(chan fakeReadResult, 8),
		writes:   make(chan fakeFrame, 8),
		controls: make(chan fakeFrame, 8),
		closed:   make(chan struct{}),
	}
}

func (c *fakeWebSocketConnection) SetReadLimit(limit int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readLimit = limit
}

func (c *fakeWebSocketConnection) SetReadDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.readDeadlines = append(c.readDeadlines, deadline)
	return nil
}

func (c *fakeWebSocketConnection) SetPongHandler(handler func(string) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pongHandler = handler
}

func (c *fakeWebSocketConnection) ReadMessage() (int, []byte, error) {
	select {
	case result := <-c.reads:
		return result.messageType, result.payload, result.err
	case <-c.closed:
		return 0, nil, errors.New("connection closed")
	}
}

func (c *fakeWebSocketConnection) SetWriteDeadline(deadline time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writeDeadlines = append(c.writeDeadlines, deadline)
	return nil
}

func (c *fakeWebSocketConnection) WriteMessage(messageType int, payload []byte) error {
	select {
	case c.writes <- fakeFrame{messageType: messageType, payload: payload}:
		return nil
	case <-c.closed:
		return errors.New("connection closed")
	}
}

func (c *fakeWebSocketConnection) WriteControl(
	messageType int,
	payload []byte,
	_ time.Time,
) error {
	select {
	case c.controls <- fakeFrame{messageType: messageType, payload: payload}:
		return nil
	case <-c.closed:
		return errors.New("connection closed")
	}
}

func (c *fakeWebSocketConnection) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return nil
}

func (c *fakeWebSocketConnection) invokePong(t *testing.T) {
	t.Helper()
	c.mu.Lock()
	handler := c.pongHandler
	c.mu.Unlock()
	if handler == nil {
		t.Fatal("pong handler was not installed")
	}
	if err := handler("pong"); err != nil {
		t.Fatalf("pong handler error = %v", err)
	}
}

func (c *fakeWebSocketConnection) readDeadlineCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.readDeadlines)
}

func TestClientReadPumpHandlesBinaryFrameAndUnregisters(t *testing.T) {
	hub := NewHub(4)
	hub.Start()
	t.Cleanup(func() { stopHub(t, hub) })
	connection := newFakeWebSocketConnection()
	client := newClientConn(1, 101, connection, connectionSettings{})
	if _, err := hub.Register(context.Background(), client); err != nil {
		t.Fatal(err)
	}

	handled := make(chan []byte, 1)
	go client.writePump(context.Background())
	go client.readPump(
		context.Background(),
		hub,
		func(_ context.Context, _ *clientConn, payload []byte) { handled <- payload },
	)

	want := []byte("protobuf-frame")
	connection.reads <- fakeReadResult{messageType: websocket.BinaryMessage, payload: want}
	select {
	case got := <-handled:
		if string(got) != string(want) {
			t.Fatalf("handled payload = %q, want %q", got, want)
		}
	case <-time.After(hubTestTimeout):
		t.Fatal("timed out waiting for binary frame handler")
	}

	connection.reads <- fakeReadResult{err: errors.New("peer disconnected")}
	assertClientClosed(t, client)
	select {
	case <-connection.closed:
	case <-time.After(hubTestTimeout):
		t.Fatal("timed out waiting for socket close")
	}
	if counts := hub.connections.Counts(); len(counts) != 0 {
		t.Fatalf("connection counts after read exit = %v, want empty", counts)
	}
}

func TestClientReadPumpRejectsTextFrame(t *testing.T) {
	hub := NewHub(4)
	hub.Start()
	t.Cleanup(func() { stopHub(t, hub) })
	connection := newFakeWebSocketConnection()
	client := newClientConn(1, 101, connection, connectionSettings{})
	if _, err := hub.Register(context.Background(), client); err != nil {
		t.Fatal(err)
	}

	go client.readPump(context.Background(), hub, nil)
	connection.reads <- fakeReadResult{messageType: websocket.TextMessage, payload: []byte("json")}

	select {
	case frame := <-connection.controls:
		if frame.messageType != websocket.CloseMessage {
			t.Fatalf("control type = %d, want CloseMessage", frame.messageType)
		}
		if len(frame.payload) < 2 {
			t.Fatalf("close payload length = %d, want at least 2", len(frame.payload))
		}
		code := int(binary.BigEndian.Uint16(frame.payload[:2]))
		if code != websocket.CloseUnsupportedData {
			t.Fatalf("close code = %d, want CloseUnsupportedData", code)
		}
	case <-time.After(hubTestTimeout):
		t.Fatal("timed out waiting for unsupported data close frame")
	}
	assertClientClosed(t, client)
}

func TestClientPongRefreshesReadDeadline(t *testing.T) {
	connection := newFakeWebSocketConnection()
	client := newClientConn(1, 101, connection, connectionSettings{
		pongTimeout: 100 * time.Millisecond,
	})
	go client.readPump(context.Background(), nil, nil)

	waitForCondition(t, func() bool { return connection.readDeadlineCount() == 1 })
	connection.invokePong(t)
	if count := connection.readDeadlineCount(); count != 2 {
		t.Fatalf("read deadline count after Pong = %d, want 2", count)
	}
	connection.reads <- fakeReadResult{err: errors.New("done")}
	assertClientClosed(t, client)
}

func TestClientWritePumpUsesBinaryFramesAndPing(t *testing.T) {
	connection := newFakeWebSocketConnection()
	client := newClientConn(1, 101, connection, connectionSettings{
		writeTimeout: 100 * time.Millisecond,
		pongTimeout:  100 * time.Millisecond,
		pingInterval: 10 * time.Millisecond,
	})
	go client.writePump(context.Background())

	payload := []byte("server-protobuf-frame")
	client.send <- payload
	select {
	case frame := <-connection.writes:
		if frame.messageType != websocket.BinaryMessage || string(frame.payload) != string(payload) {
			t.Fatalf("written frame = (%d, %q), want binary %q", frame.messageType, frame.payload, payload)
		}
	case <-time.After(hubTestTimeout):
		t.Fatal("timed out waiting for binary write")
	}

	select {
	case frame := <-connection.controls:
		if frame.messageType != websocket.PingMessage {
			t.Fatalf("control type = %d, want PingMessage", frame.messageType)
		}
	case <-time.After(hubTestTimeout):
		t.Fatal("timed out waiting for Ping")
	}

	client.close()
	select {
	case frame := <-connection.controls:
		if frame.messageType != websocket.CloseMessage {
			t.Fatalf("control type = %d, want CloseMessage", frame.messageType)
		}
	case <-time.After(hubTestTimeout):
		t.Fatal("timed out waiting for close frame")
	}
	select {
	case <-connection.closed:
	case <-time.After(hubTestTimeout):
		t.Fatal("timed out waiting for socket close")
	}
}

func TestClientWritePumpStopsWhenContextIsCanceled(t *testing.T) {
	connection := newFakeWebSocketConnection()
	client := newClientConn(1, 101, connection, connectionSettings{})
	ctx, cancel := context.WithCancel(context.Background())
	go client.writePump(ctx)
	cancel()

	select {
	case frame := <-connection.controls:
		if frame.messageType != websocket.CloseMessage {
			t.Fatalf("control type = %d, want CloseMessage", frame.messageType)
		}
	case <-time.After(hubTestTimeout):
		t.Fatal("timed out waiting for shutdown close frame")
	}
	assertClientClosed(t, client)
	select {
	case <-connection.closed:
	case <-time.After(hubTestTimeout):
		t.Fatal("timed out waiting for socket close")
	}
}

func waitForCondition(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(hubTestTimeout)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for condition")
		}
		time.Sleep(time.Millisecond)
	}
}
