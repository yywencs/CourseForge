package catalogapp

import (
	"context"

	domain "prizeforge/internal/catalog/domain"
)

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

	ListTeachingClasses(context.Context, uint64, string) ([]TeachingClassView, error)
	ListStudentCatalog(context.Context, StudentCatalogQuery) ([]TeachingClassView, error)
	GetTeachingClass(context.Context, uint64) (*TeachingClassView, error)
	GetTeachingClassForUpdate(context.Context, uint64) (*domain.TeachingClass, error)
	InsertTeachingClass(context.Context, *domain.TeachingClass) error
	SaveTeachingClass(context.Context, *domain.TeachingClass) error
	RemoveTeachingClass(context.Context, uint64) error
	InspectTeachingClassUsage(context.Context, uint64) (domain.TeachingClassUsage, error)
}
