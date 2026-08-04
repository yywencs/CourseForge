package danmakuws

import (
	"bytes"
	"context"
	"errors"
	"sync"
)

const defaultBroadcastQueueSize = 256

var (
	// ErrHubStopped 表示本机实时连接调度器已经停止接收事件。
	ErrHubStopped = errors.New("danmaku websocket hub is stopped")
	// ErrBroadcastQueueFull 表示实时广播入口已达到容量上限。
	ErrBroadcastQueueFull = errors.New("danmaku websocket broadcast queue is full")
)

type registerRequest struct {
	client *clientConn
	result chan bool
}

type unregisterRequest struct {
	client *clientConn
	result chan unregisterResult
}

type unregisterResult struct {
	removed    bool
	videoEmpty bool
}

type broadcastEvent struct {
	videoID uint64
	payload []byte
}

// Hub 调度当前服务实例内的实时连接注册、注销和视频弹幕广播。
// 跨实例消息传播不属于 Hub，由后续接入的消息总线负责。
type Hub struct {
	connections *videoConnectionRegistry
	register    chan registerRequest
	unregister  chan unregisterRequest
	broadcast   chan broadcastEvent
	stopping    chan struct{}
	done        chan struct{}
	startOnce   sync.Once
	stopOnce    sync.Once
}

// NewHub 创建单实例实时连接调度器。非正数队列容量会使用默认值。
func NewHub(broadcastQueueSize int) *Hub {
	if broadcastQueueSize <= 0 {
		broadcastQueueSize = defaultBroadcastQueueSize
	}
	return &Hub{
		connections: newVideoConnectionRegistry(),
		register:    make(chan registerRequest),
		unregister:  make(chan unregisterRequest),
		broadcast:   make(chan broadcastEvent, broadcastQueueSize),
		stopping:    make(chan struct{}),
		done:        make(chan struct{}),
	}
}

// Start 启动 Hub 事件循环，可重复调用。
func (h *Hub) Start() {
	h.startOnce.Do(func() { go h.run() })
}

// Register 将连接注册到对应视频，并返回它是否为该视频的本机首个连接。
func (h *Hub) Register(ctx context.Context, client *clientConn) (bool, error) {
	request := registerRequest{client: client, result: make(chan bool, 1)}
	select {
	case h.register <- request:
	case <-h.stopping:
		client.close()
		return false, ErrHubStopped
	case <-ctx.Done():
		return false, ctx.Err()
	}

	select {
	case firstForVideo := <-request.result:
		return firstForVideo, nil
	case <-h.stopping:
		// 请求可能已交给事件循环但尚未注册，调用方仍需得到关闭保证。
		client.close()
		return false, ErrHubStopped
	}
}

// Unregister 删除连接，返回连接是否存在，以及该视频是否已无本机连接。
func (h *Hub) Unregister(
	ctx context.Context,
	client *clientConn,
) (removed bool, videoEmpty bool, err error) {
	request := unregisterRequest{client: client, result: make(chan unregisterResult, 1)}
	select {
	case h.unregister <- request:
	case <-h.stopping:
		client.close()
		return false, false, ErrHubStopped
	case <-ctx.Done():
		return false, false, ctx.Err()
	}

	select {
	case result := <-request.result:
		return result.removed, result.videoEmpty, nil
	case <-h.stopping:
		client.close()
		return false, false, ErrHubStopped
	}
}

// Broadcast 异步提交一条已编码的 Protobuf 消息，发送队列满时立即返回。
func (h *Hub) Broadcast(videoID uint64, payload []byte) error {
	event := broadcastEvent{videoID: videoID, payload: bytes.Clone(payload)}
	select {
	case <-h.stopping:
		return ErrHubStopped
	default:
	}

	select {
	case h.broadcast <- event:
		return nil
	case <-h.stopping:
		return ErrHubStopped
	default:
		return ErrBroadcastQueueFull
	}
}

// Stop 停止接收新事件，排空本机连接，并等待事件循环退出。
func (h *Hub) Stop(ctx context.Context) error {
	// 即使尚未显式启动，Stop 也能完成完整关闭流程。
	h.Start()
	h.stopOnce.Do(func() { close(h.stopping) })
	select {
	case <-h.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hub) run() {
	defer close(h.done)
	for {
		select {
		case <-h.stopping:
			h.shutdown()
			return
		default:
		}

		select {
		case request := <-h.register:
			request.result <- h.connections.Add(request.client)
		case request := <-h.unregister:
			removed, videoEmpty := h.connections.Remove(request.client)
			request.client.close()
			request.result <- unregisterResult{removed: removed, videoEmpty: videoEmpty}
		case event := <-h.broadcast:
			h.broadcastToVideo(event)
		case <-h.stopping:
			h.shutdown()
			return
		}
	}
}

func (h *Hub) broadcastToVideo(event broadcastEvent) {
	clients, exists := h.connections.SnapshotInto(event.videoID, nil)
	if !exists {
		return
	}
	for _, client := range clients {
		select {
		case client.send <- event.payload:
		default:
			// 单个慢连接不能阻塞同一视频的其他连接或整个 Hub。
		}
	}
}

func (h *Hub) shutdown() {
	for _, client := range h.connections.Drain() {
		client.close()
	}
}
