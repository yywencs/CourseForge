package danmakuws

import "sync"

// videoConnectionRegistry 并发安全地维护视频与本机实时连接的对应关系。
type videoConnectionRegistry struct {
	mu                 sync.RWMutex
	connectionsByVideo map[uint64]map[*clientConn]struct{}
}

// newVideoConnectionRegistry 创建一个空的本机视频连接注册表。
func newVideoConnectionRegistry() *videoConnectionRegistry {
	return &videoConnectionRegistry{
		connectionsByVideo: make(map[uint64]map[*clientConn]struct{}),
	}
}

// Add 注册连接，并返回它是否为该视频在本机注册的首个连接。
func (r *videoConnectionRegistry) Add(client *clientConn) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	clients, exists := r.connectionsByVideo[client.videoID]
	if !exists {
		clients = make(map[*clientConn]struct{})
		r.connectionsByVideo[client.videoID] = clients
	}
	clients[client] = struct{}{}
	return !exists
}

// Remove 删除连接，返回连接是否存在，以及该视频是否已无本机连接。
func (r *videoConnectionRegistry) Remove(client *clientConn) (
	removed bool,
	videoEmpty bool,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	clients, exists := r.connectionsByVideo[client.videoID]
	if !exists {
		return false, false
	}
	if _, exists = clients[client]; !exists {
		return false, false
	}

	delete(clients, client)
	if len(clients) == 0 {
		delete(r.connectionsByVideo, client.videoID)
		return true, true
	}
	return true, false
}

// SnapshotInto 将指定视频的本机连接快照追加到 dst。
// 调用方应在方法返回后执行网络或队列写入，避免在注册表锁内执行慢操作。
func (r *videoConnectionRegistry) SnapshotInto(
	videoID uint64,
	dst []*clientConn,
) ([]*clientConn, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	clients, exists := r.connectionsByVideo[videoID]
	if !exists {
		return dst, false
	}
	for client := range clients {
		dst = append(dst, client)
	}
	return dst, true
}

// Counts 返回各视频当前本机连接数的独立快照。
func (r *videoConnectionRegistry) Counts() map[uint64]int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	counts := make(map[uint64]int, len(r.connectionsByVideo))
	for videoID, clients := range r.connectionsByVideo {
		counts[videoID] = len(clients)
	}
	return counts
}

// Drain 原子取出并移除当前注册的全部连接，供服务优雅关闭使用。
func (r *videoConnectionRegistry) Drain() []*clientConn {
	r.mu.Lock()
	defer r.mu.Unlock()

	clients := make([]*clientConn, 0)
	for _, videoConnections := range r.connectionsByVideo {
		for client := range videoConnections {
			clients = append(clients, client)
		}
	}
	r.connectionsByVideo = make(map[uint64]map[*clientConn]struct{})
	return clients
}
