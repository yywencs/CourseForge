package catalog

import (
	"errors"
	"fmt"
)

var (
	ErrNotFound              = errors.New("记录不存在")
	ErrConflict              = errors.New("记录当前状态不允许执行该操作")
	ErrCourseInUse           = errors.New("课程已有教学或修读数据，不能删除")
	ErrTeachingClassInUse    = errors.New("教学班已有轮次配置或选课数据，不能删除")
	ErrRoundInUse            = errors.New("选课轮次已有配置或业务数据，不能删除")
	ErrTermMismatch          = errors.New("教学班与选课轮次不属于同一学期")
	ErrInvalidSchedule       = errors.New("上课时间范围不合法")
	ErrInvalidTimeRange      = errors.New("选课轮次结束时间必须晚于开始时间")
	ErrInvalidTeachingClass  = errors.New("教学班容量或年级范围不合法")
	ErrInvalidCourse         = errors.New("课程编码、名称和学分必须有效")
	ErrInvalidSelectionRound = errors.New("选课轮次信息不完整")

	ErrCourseCoreLocked         = fmt.Errorf("%w: 已存在非计划教学班，只能维护课程简介、标签和视频", ErrConflict)
	ErrTeachingClassNotEditable = fmt.Errorf("%w: 教学班已进入选课流程，不能通过基础维护修改", ErrConflict)
	ErrTeachingClassTermLocked  = fmt.Errorf("%w: 教学班已绑定轮次，不能修改所属学期", ErrConflict)
	ErrRoundNotEditable         = fmt.Errorf("%w: 轮次已进入选课流程，不能通过基础维护修改", ErrConflict)
	ErrRoundTermLocked          = fmt.Errorf("%w: 轮次已绑定教学班，不能修改所属学期", ErrConflict)
)
