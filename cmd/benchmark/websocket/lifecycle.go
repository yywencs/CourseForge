package main

import (
	"sync/atomic"
	"time"
)

// runState 保存正式测量窗口。预热产生的消息不会污染最终结果。
type runState struct {
	startedAt atomic.Int64
	endedAt   atomic.Int64
}

func (s *runState) start(now time.Time) { s.startedAt.Store(now.UnixNano()) }
func (s *runState) end(now time.Time)   { s.endedAt.CompareAndSwap(0, now.UnixNano()) }

func (s *runState) shouldRecord(sentAt time.Time) bool {
	start := s.startedAt.Load()
	if start == 0 || sentAt.UnixNano() < start {
		return false
	}
	end := s.endedAt.Load()
	return end == 0 || sentAt.UnixNano() < end
}

func (s *runState) duration() time.Duration {
	start, end := s.startedAt.Load(), s.endedAt.Load()
	if start == 0 || end <= start {
		return 0
	}
	return time.Duration(end - start)
}
