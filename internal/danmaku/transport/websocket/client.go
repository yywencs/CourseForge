package danmakuws

import (
	"context"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultClientSendQueueSize = 64
	defaultReadLimit           = 64 << 10
	defaultWriteTimeout        = 10 * time.Second
	defaultPongTimeout         = 60 * time.Second
	defaultPingInterval        = 50 * time.Second
)

// websocketConnection 描述读写循环使用的 Gorilla WebSocket 能力。
// 使用窄接口可以在不建立真实网络连接的情况下测试超时和控制帧行为。
type websocketConnection interface {
	SetReadLimit(int64)
	SetReadDeadline(time.Time) error
	SetPongHandler(func(string) error)
	ReadMessage() (messageType int, payload []byte, err error)
	SetWriteDeadline(time.Time) error
	WriteMessage(messageType int, payload []byte) error
	WriteControl(messageType int, payload []byte, deadline time.Time) error
	Close() error
}

type connectionSettings struct {
	sendQueueSize int
	readLimit     int64
	writeTimeout  time.Duration
	pongTimeout   time.Duration
	pingInterval  time.Duration
}

func (s connectionSettings) withDefaults() connectionSettings {
	if s.sendQueueSize <= 0 {
		s.sendQueueSize = defaultClientSendQueueSize
	}
	if s.readLimit <= 0 {
		s.readLimit = defaultReadLimit
	}
	if s.writeTimeout <= 0 {
		s.writeTimeout = defaultWriteTimeout
	}
	if s.pongTimeout <= 0 {
		s.pongTimeout = defaultPongTimeout
	}
	if s.pingInterval <= 0 {
		s.pingInterval = defaultPingInterval
	}
	if s.pingInterval >= s.pongTimeout {
		s.pingInterval = s.pongTimeout / 2
	}
	return s
}

// binaryMessageHandler 处理客户端发来的一个完整 Protobuf 二进制帧。
type binaryMessageHandler func(context.Context, *clientConn, []byte)

// clientConn 表示一条已经绑定学生和课程视频的本机实时连接。
type clientConn struct {
	studentID  uint64
	videoID    uint64
	connection websocketConnection
	settings   connectionSettings
	send       chan []byte
	done       chan struct{}
	closeOnce  sync.Once
}

// newClientConn 创建一条具有独立有界发送队列的实时连接。
func newClientConn(
	studentID uint64,
	videoID uint64,
	connection websocketConnection,
	settings connectionSettings,
) *clientConn {
	settings = settings.withDefaults()
	return &clientConn{
		studentID:  studentID,
		videoID:    videoID,
		connection: connection,
		settings:   settings,
		send:       make(chan []byte, settings.sendQueueSize),
		done:       make(chan struct{}),
	}
}

// readPump 持续读取客户端二进制帧并交给协议处理器。
// 文本帧和其他数据帧会以 Unsupported Data 关闭码拒绝。
func (c *clientConn) readPump(ctx context.Context, hub *Hub, handler binaryMessageHandler) {
	defer c.unregister(hub)

	c.connection.SetReadLimit(c.settings.readLimit)
	if err := c.refreshReadDeadline(); err != nil {
		return
	}
	c.connection.SetPongHandler(func(string) error { return c.refreshReadDeadline() })

	for {
		messageType, payload, err := c.connection.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.BinaryMessage {
			_ = c.writeControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(
					websocket.CloseUnsupportedData,
					"binary protobuf frames required",
				),
			)
			return
		}
		if handler != nil {
			handler(ctx, c, payload)
		}
	}
}

// writePump 将有界发送队列中的 Protobuf 数据写成二进制帧，并定期发送 Ping。
func (c *clientConn) writePump(ctx context.Context) {
	ticker := time.NewTicker(c.settings.pingInterval)
	defer ticker.Stop()
	defer c.connection.Close()

	for {
		select {
		case payload := <-c.send:
			if err := c.writeMessage(websocket.BinaryMessage, payload); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.writeControl(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-ctx.Done():
			c.close()
			_ = c.writeControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down"),
			)
			return
		case <-c.done:
			_ = c.writeControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseGoingAway, "connection closing"),
			)
			return
		}
	}
}

func (c *clientConn) refreshReadDeadline() error {
	return c.connection.SetReadDeadline(time.Now().Add(c.settings.pongTimeout))
}

func (c *clientConn) writeMessage(messageType int, payload []byte) error {
	if err := c.connection.SetWriteDeadline(time.Now().Add(c.settings.writeTimeout)); err != nil {
		return err
	}
	return c.connection.WriteMessage(messageType, payload)
}

func (c *clientConn) writeControl(messageType int, payload []byte) error {
	return c.connection.WriteControl(
		messageType,
		payload,
		time.Now().Add(c.settings.writeTimeout),
	)
}

func (c *clientConn) unregister(hub *Hub) {
	if hub == nil {
		c.close()
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), c.settings.writeTimeout)
	defer cancel()
	_, _, _ = hub.Unregister(ctx, c)
}

// close 幂等地通知连接读写循环退出。
// send 队列不会关闭，避免广播快照与连接注销并发时向已关闭通道写入。
func (c *clientConn) close() {
	c.closeOnce.Do(func() { close(c.done) })
}
