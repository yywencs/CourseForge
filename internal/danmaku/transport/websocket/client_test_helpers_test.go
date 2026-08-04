package danmakuws

func newTestClientConn(studentID, videoID uint64, sendQueueSize int) *clientConn {
	return newClientConn(studentID, videoID, nil, connectionSettings{
		sendQueueSize: sendQueueSize,
	})
}
