package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	danmakuv1 "github.com/yywencs/courseforge/gen/courseforge/danmaku/v1"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

const duplicateWindowSize = 256

type duplicateTracker struct {
	seen [duplicateWindowSize]uint64
}

func (t *duplicateTracker) isDuplicate(messageID uint64) bool {
	if messageID == 0 {
		return false
	}
	slot := messageID % duplicateWindowSize
	if t.seen[slot] == messageID {
		return true
	}
	t.seen[slot] = messageID
	return false
}

func runClient(
	ctx context.Context,
	index int,
	cfg benchmarkConfig,
	token string,
	state *runState,
	metrics *benchmarkMetrics,
	connectionResult chan<- bool,
) {
	metrics.connectAttempted.Add(1)
	dialer := *websocket.DefaultDialer
	dialer.HandshakeTimeout = cfg.Timeout
	connection, response, err := dialer.DialContext(ctx, cfg.websocketURL(index), nil)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		metrics.connectFailed.Add(1)
		connectionResult <- false
		return
	}
	defer connection.Close()

	authentication, err := proto.Marshal(&danmakuv1.ClientFrame{
		RequestId: uuid.NewString(),
		Payload: &danmakuv1.ClientFrame_Authenticate{
			Authenticate: &danmakuv1.AuthenticateRequest{AccessToken: token},
		},
	})
	if err != nil || connection.WriteMessage(websocket.BinaryMessage, authentication) != nil ||
		waitUntilReady(connection, cfg.VideoID, cfg.Timeout) != nil {
		metrics.connectFailed.Add(1)
		connectionResult <- false
		return
	}
	metrics.connectSucceeded.Add(1)
	metrics.connected.Add(1)
	connectionResult <- true
	defer metrics.connected.Add(-1)

	closeDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = connection.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, "benchmark finished"),
				time.Now().Add(time.Second))
			_ = connection.Close()
		case <-closeDone:
		}
	}()
	defer close(closeDone)

	tracker := duplicateTracker{}
	for {
		messageType, payload, err := connection.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.BinaryMessage {
			metrics.protocolErrors.Add(1)
			continue
		}
		var frame danmakuv1.ServerFrame
		if proto.Unmarshal(payload, &frame) != nil {
			metrics.protocolErrors.Add(1)
			continue
		}
		published := frame.GetDanmakuPublished()
		if published == nil {
			if frame.GetError() != nil {
				metrics.protocolErrors.Add(1)
			}
			continue
		}
		sentAt, ok := benchmarkSentAt(published.GetContent())
		if !ok || !state.shouldRecord(sentAt) {
			continue
		}
		metrics.received.Add(1)
		if tracker.isDuplicate(published.GetId()) {
			metrics.duplicates.Add(1)
		}
		metrics.latency.record(time.Since(sentAt))
	}
}

func waitUntilReady(connection *websocket.Conn, videoID uint64, timeout time.Duration) error {
	if err := connection.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	defer connection.SetReadDeadline(time.Time{})
	messageType, payload, err := connection.ReadMessage()
	if err != nil {
		return err
	}
	if messageType != websocket.BinaryMessage {
		return fmt.Errorf("鉴权响应不是二进制帧")
	}
	var frame danmakuv1.ServerFrame
	if err := proto.Unmarshal(payload, &frame); err != nil {
		return err
	}
	ready := frame.GetConnectionReady()
	if ready == nil || ready.GetVideoId() != videoID {
		if failure := frame.GetError(); failure != nil {
			return fmt.Errorf("WebSocket 鉴权失败: %s", failure.GetMessage())
		}
		return fmt.Errorf("未收到 connection_ready")
	}
	return nil
}

func benchmarkSentAt(content string) (time.Time, bool) {
	parts := strings.SplitN(content, ":", 3)
	if len(parts) != 3 || parts[0] != "cfbench" {
		return time.Time{}, false
	}
	nanoseconds, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(0, nanoseconds), true
}
