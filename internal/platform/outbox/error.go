package outbox

import "errors"

var (
	errNewEventRequired        = errors.New("待写入的发件箱事件不能为空")
	errEventRequired           = errors.New("发件箱事件不能为空")
	errEventIDMissing          = errors.New("发件箱事件编号缺失")
	errInvalidEventState       = errors.New("发件箱事件状态不合法")
	errIncompleteEventIdentity = errors.New("发件箱事件标识信息不完整")
	errEventIdentityTooLong    = errors.New("发件箱事件标识信息超出数据库字段长度限制")
	errInvalidEventPayload     = errors.New("发件箱事件载荷格式不合法")
)
