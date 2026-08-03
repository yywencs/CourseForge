package catalogapp

import (
	"context"

	domain "prizeforge/internal/catalog/domain"
)

type CourseInput struct {
	CourseCode   string
	CourseName   string
	Credits      float64
	Introduction string
	Tags         []string
}

func (i CourseInput) details() domain.CourseDetails {
	return domain.CourseDetails{
		CourseCode: i.CourseCode, CourseName: i.CourseName, Credits: i.Credits,
		Introduction: i.Introduction, Tags: i.Tags,
	}
}

type TeachingClassInput struct {
	ClassCode        string
	TermID           uint64
	CourseID         uint64
	TeacherName      string
	Location         string
	Capacity         uint32
	MinimumGradeYear *uint16
	MaximumGradeYear *uint16
	Schedules        []domain.Schedule
}

func (i TeachingClassInput) plan() domain.TeachingClassPlan {
	return domain.TeachingClassPlan{
		ClassCode: i.ClassCode, TermID: i.TermID, CourseID: i.CourseID,
		TeacherName: i.TeacherName, Location: i.Location, Capacity: i.Capacity,
		MinimumGradeYear: i.MinimumGradeYear, MaximumGradeYear: i.MaximumGradeYear,
		Schedules: i.Schedules,
	}
}

type StudentCatalogQuery struct {
	RoundID uint64
	Keyword string
}

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

// 查询侧没有业务状态变更，可以直接使用只读端口。
func (s *Service) ListCourses(ctx context.Context, keyword string) ([]domain.Course, error) {
	return s.repository.ListCourses(ctx, keyword)
}

func (s *Service) GetCourse(ctx context.Context, id uint64) (*domain.Course, error) {
	return s.repository.GetCourse(ctx, id)
}

func (s *Service) CreateCourse(ctx context.Context, input CourseInput) (*domain.Course, error) {
	course, err := domain.NewCourse(input.details())
	if err != nil {
		return nil, err
	}
	if err := s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		return s.repository.InsertCourse(txCtx, course)
	}); err != nil {
		return nil, err
	}
	return course, nil
}

func (s *Service) UpdateCourse(ctx context.Context, id uint64, input CourseInput) (*domain.Course, error) {
	var course *domain.Course
	err := s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		current, err := s.repository.GetCourseForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		usage, err := s.repository.InspectCourseUsage(txCtx, id)
		if err != nil {
			return err
		}
		if err := current.Change(input.details(), usage); err != nil {
			return err
		}
		if err := s.repository.SaveCourse(txCtx, current); err != nil {
			return err
		}
		course = current
		return nil
	})
	return course, err
}

func (s *Service) DeleteCourse(ctx context.Context, id uint64) error {
	return s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		course, err := s.repository.GetCourseForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		usage, err := s.repository.InspectCourseUsage(txCtx, id)
		if err != nil {
			return err
		}
		if err := course.EnsureDeletable(usage); err != nil {
			return err
		}
		return s.repository.RemoveCourse(txCtx, id)
	})
}

func (s *Service) ListTeachingClasses(ctx context.Context, termID uint64, keyword string) ([]TeachingClassView, error) {
	return s.repository.ListTeachingClasses(ctx, termID, keyword)
}

func (s *Service) ListStudentCatalog(ctx context.Context, query StudentCatalogQuery) ([]TeachingClassView, error) {
	return s.repository.ListStudentCatalog(ctx, query)
}

func (s *Service) GetTeachingClass(ctx context.Context, id uint64) (*TeachingClassView, error) {
	return s.repository.GetTeachingClass(ctx, id)
}

func (s *Service) CreateTeachingClass(ctx context.Context, input TeachingClassInput) (*TeachingClassView, error) {
	class, err := domain.NewTeachingClass(input.plan())
	if err != nil {
		return nil, err
	}
	var created *TeachingClassView
	err = s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		// 锁住被引用课程，使“删除课程”和“创建教学班”不能并发穿透逻辑外键。
		if _, err := s.repository.GetCourseForUpdate(txCtx, class.CourseID); err != nil {
			return err
		}
		if err := s.repository.InsertTeachingClass(txCtx, class); err != nil {
			return err
		}
		// 在提交前读取完整投影；读取失败则整体回滚，避免“已创建但接口报错”。
		created, err = s.repository.GetTeachingClass(txCtx, class.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *Service) UpdateTeachingClass(ctx context.Context, id uint64, input TeachingClassInput) (*TeachingClassView, error) {
	var class *TeachingClassView
	err := s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		// 先锁目标课程，再锁教学班，保持逻辑外键操作的锁顺序一致。
		if _, err := s.repository.GetCourseForUpdate(txCtx, input.CourseID); err != nil {
			return err
		}
		current, err := s.repository.GetTeachingClassForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		usage, err := s.repository.InspectTeachingClassUsage(txCtx, id)
		if err != nil {
			return err
		}
		if err := current.ChangePlan(input.plan(), usage); err != nil {
			return err
		}
		if err := s.repository.SaveTeachingClass(txCtx, current); err != nil {
			return err
		}
		// 同一事务内刷新课程投影字段，读取失败时不提交半完成操作。
		class, err = s.repository.GetTeachingClass(txCtx, current.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return class, nil
}

func (s *Service) DeleteTeachingClass(ctx context.Context, id uint64) error {
	return s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		class, err := s.repository.GetTeachingClassForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		usage, err := s.repository.InspectTeachingClassUsage(txCtx, id)
		if err != nil {
			return err
		}
		if err := class.EnsureDeletable(usage); err != nil {
			return err
		}
		return s.repository.RemoveTeachingClass(txCtx, id)
	})
}
