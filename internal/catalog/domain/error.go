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
	ErrInvalidSchedule       = errors.New("上课时间范围不合法")
	ErrInvalidTeachingClass  = errors.New("教学班容量或年级范围不合法")
	ErrInvalidCourse         = errors.New("课程编码、名称和学分必须有效")
	ErrInvalidCourseVideo    = errors.New("课程视频信息不合法")
	ErrVideoUploadIncomplete = errors.New("视频尚未完整上传")

	ErrCourseCoreLocked                = fmt.Errorf("%w: 已存在非计划教学班，只能维护课程简介和标签", ErrConflict)
	ErrTeachingClassNotEditable        = fmt.Errorf("%w: 教学班已进入选课流程，不能通过基础维护修改", ErrConflict)
	ErrTeachingClassTermLocked         = fmt.Errorf("%w: 教学班已绑定轮次，不能修改所属学期", ErrConflict)
	ErrCourseVideoNotUploadable        = fmt.Errorf("%w: 视频当前状态不能继续上传", ErrConflict)
	ErrCourseVideoNotPlayable          = fmt.Errorf("%w: 视频尚不可播放", ErrConflict)
	ErrCourseVideoUploadNotCompletable = fmt.Errorf("%w: 视频上传任务当前状态不能完成", ErrConflict)
)
