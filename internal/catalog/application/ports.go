package catalogapp

import (
	"context"
	"errors"
	"time"

	domain "prizeforge/internal/catalog/domain"
)

var ErrStoredObjectNotFound = errors.New("stored object not found")

type StoredObject struct {
	Size        int64
	ContentType string
}

// ObjectStorage 只暴露课程视频用例需要的能力，避免应用层依赖具体的 MinIO 或云 OSS SDK。
// 上传和播放均使用短期签名 URL，视频内容不经过 API 服务中转。
type ObjectStorage interface {
	PresignUpload(context.Context, string, time.Duration) (string, error)
	StatObject(context.Context, string) (StoredObject, error)
	PresignPlayback(context.Context, string, time.Duration) (string, error)
}

// Repository 是 Catalog 用例所消费的端口。事务、锁和 SQL 由基础设施实现，
// 业务决策由 Application 加载事实后交给 Domain 完成。
type Repository interface {
	WithinTransaction(context.Context, func(context.Context) error) error

	ListCourses(context.Context, string) ([]domain.Course, error)
	GetCourse(context.Context, uint64) (*domain.Course, error)
	GetCourseForUpdate(context.Context, uint64) (*domain.Course, error)
	InsertCourse(context.Context, *domain.Course) error
	SaveCourse(context.Context, *domain.Course) error
	RemoveCourse(context.Context, uint64) error
	InspectCourseUsage(context.Context, uint64) (domain.CourseUsage, error)
	ListCourseVideos(context.Context, uint64) ([]domain.CourseVideo, error)
	GetCourseVideo(context.Context, uint64) (*domain.CourseVideo, error)
	GetCourseVideoForUpdate(context.Context, uint64) (*domain.CourseVideo, error)
	GetCourseVideoByPositionForUpdate(context.Context, uint64, domain.CourseVideoKind, uint32) (*domain.CourseVideo, error)
	InsertCourseVideo(context.Context, *domain.CourseVideo) error
	SaveCourseVideo(context.Context, *domain.CourseVideo, domain.CourseVideoStatus) error
	InsertCourseVideoUpload(context.Context, *domain.CourseVideoUpload) error

	ListTeachingClasses(context.Context, uint64, string) ([]TeachingClassView, error)
	ListStudentCatalog(context.Context, StudentCatalogQuery) ([]TeachingClassView, error)
	GetTeachingClass(context.Context, uint64) (*TeachingClassView, error)
	GetTeachingClassForUpdate(context.Context, uint64) (*domain.TeachingClass, error)
	InsertTeachingClass(context.Context, *domain.TeachingClass) error
	SaveTeachingClass(context.Context, *domain.TeachingClass) error
	RemoveTeachingClass(context.Context, uint64) error
	InspectTeachingClassUsage(context.Context, uint64) (domain.TeachingClassUsage, error)
}
