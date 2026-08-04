package danmaku

import "errors"

var (
	// ErrNotFound 表示指定弹幕不存在。
	ErrNotFound = errors.New("弹幕不存在")
	// ErrInvalidDanmaku 表示弹幕标识、播放位置或内容不符合领域约束。
	ErrInvalidDanmaku = errors.New("弹幕信息不合法")
	// ErrClientMessageExists 表示同一视频和学生下的客户端消息ID已经持久化。
	ErrClientMessageExists = errors.New("弹幕幂等请求已存在")
	// ErrIdempotencyConflict 表示重复使用幂等键时请求内容与原请求不一致。
	ErrIdempotencyConflict = errors.New("弹幕幂等请求与原请求不一致")
	// ErrVideoNotFound 表示弹幕关联的课程视频不存在。
	ErrVideoNotFound = errors.New("课程视频不存在")
	// ErrVideoNotPlayable 表示课程视频类型或状态不允许发布弹幕。
	ErrVideoNotPlayable = errors.New("课程视频当前不可发布弹幕")
	// ErrVideoDurationUnavailable 表示缺少校验播放位置所需的视频时长。
	ErrVideoDurationUnavailable = errors.New("课程视频时长不可用")
)
