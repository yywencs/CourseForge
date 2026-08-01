package catalogapp

import (
	"context"
	"errors"
	"testing"

	domain "prizeforge/internal/catalog/domain"
)

func TestUpdateCourseStopsBeforeSaveWhenDomainRejectsCoreChange(t *testing.T) {
	repository := &repositoryStub{
		course:      &domain.Course{ID: 1, CourseCode: "CS-101", CourseName: "程序设计", Credits: 3},
		courseUsage: domain.CourseUsage{NonPlannedTeachingClassCount: 1},
	}
	service := NewService(repository)

	_, err := service.UpdateCourse(context.Background(), 1, CourseInput{
		CourseCode: "CS-102", CourseName: "程序设计", Credits: 3,
	})
	if !errors.Is(err, domain.ErrCourseCoreLocked) {
		t.Fatalf("UpdateCourse() error = %v, want %v", err, domain.ErrCourseCoreLocked)
	}
	if repository.saveCourseCalled {
		t.Fatal("SaveCourse() called after domain rejected change")
	}
	if repository.transactions != 1 {
		t.Fatalf("transactions = %d, want 1", repository.transactions)
	}
}

func TestUpdateCoursePersistsDomainChangeInsideTransaction(t *testing.T) {
	repository := &repositoryStub{
		course: &domain.Course{ID: 1, CourseCode: "CS-101", CourseName: "程序设计", Credits: 3},
	}
	service := NewService(repository)

	updated, err := service.UpdateCourse(context.Background(), 1, CourseInput{
		CourseCode: "CS-101", CourseName: "程序设计", Credits: 3,
		Introduction: "新的课程简介",
	})
	if err != nil {
		t.Fatalf("UpdateCourse() error = %v", err)
	}
	if !repository.saveCourseCalled || updated.Introduction != "新的课程简介" {
		t.Fatalf("saved = %v, updated = %#v", repository.saveCourseCalled, updated)
	}
}

func TestUpdateTeachingClassLoadsFactsThenUsesDomainBehavior(t *testing.T) {
	repository := &repositoryStub{
		course: &domain.Course{ID: 1},
		class: &domain.TeachingClass{
			ID: 2, ClassCode: "CS-101-01", TermID: 202601, CourseID: 1,
			Capacity: 30, State: domain.TeachingClassStateOpen,
		},
	}
	service := NewService(repository)

	_, err := service.UpdateTeachingClass(context.Background(), 2, validTeachingClassInput())
	if !errors.Is(err, domain.ErrTeachingClassNotEditable) {
		t.Fatalf("UpdateTeachingClass() error = %v, want %v", err, domain.ErrTeachingClassNotEditable)
	}
	wantEvents := []string{"course-lock", "class-lock", "class-usage"}
	if !equalStrings(repository.events, wantEvents) {
		t.Fatalf("events = %v, want %v", repository.events, wantEvents)
	}
	if repository.saveClassCalled {
		t.Fatal("SaveTeachingClass() called after domain rejected change")
	}
}

func TestCreateTeachingClassValidatesBeforeOpeningTransaction(t *testing.T) {
	repository := &repositoryStub{}
	service := NewService(repository)
	input := validTeachingClassInput()
	input.Capacity = 0

	_, err := service.CreateTeachingClass(context.Background(), input)
	if !errors.Is(err, domain.ErrInvalidTeachingClass) {
		t.Fatalf("CreateTeachingClass() error = %v, want %v", err, domain.ErrInvalidTeachingClass)
	}
	if repository.transactions != 0 {
		t.Fatalf("transactions = %d, want 0", repository.transactions)
	}
}

func validTeachingClassInput() TeachingClassInput {
	return TeachingClassInput{
		ClassCode: "CS-101-01", TermID: 202601, CourseID: 1, Capacity: 30,
		Schedules: []domain.Schedule{{DayOfWeek: 1, StartWeek: 1, EndWeek: 16, StartSection: 1, EndSection: 2}},
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

type repositoryStub struct {
	transactions int
	events       []string

	course      *domain.Course
	class       *domain.TeachingClass
	courseUsage domain.CourseUsage
	classUsage  domain.TeachingClassUsage

	saveCourseCalled bool
	saveClassCalled  bool
}

func (r *repositoryStub) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	r.transactions++
	return fn(ctx)
}

func (r *repositoryStub) ListCourses(context.Context, string) ([]domain.Course, error) {
	return nil, nil
}
func (r *repositoryStub) GetCourse(context.Context, uint64) (*domain.Course, error) {
	return r.course, nil
}
func (r *repositoryStub) GetCourseForUpdate(context.Context, uint64) (*domain.Course, error) {
	r.events = append(r.events, "course-lock")
	if r.course == nil {
		return nil, domain.ErrNotFound
	}
	return r.course, nil
}
func (r *repositoryStub) InsertCourse(context.Context, *domain.Course) error { return nil }
func (r *repositoryStub) SaveCourse(context.Context, *domain.Course) error {
	r.saveCourseCalled = true
	return nil
}
func (r *repositoryStub) RemoveCourse(context.Context, uint64) error { return nil }
func (r *repositoryStub) InspectCourseUsage(context.Context, uint64) (domain.CourseUsage, error) {
	r.events = append(r.events, "course-usage")
	return r.courseUsage, nil
}

func (r *repositoryStub) ListTeachingClasses(context.Context, uint64, string) ([]TeachingClassView, error) {
	return nil, nil
}
func (r *repositoryStub) ListStudentCatalog(context.Context, StudentCatalogQuery) ([]TeachingClassView, error) {
	return nil, nil
}
func (r *repositoryStub) GetTeachingClass(context.Context, uint64) (*TeachingClassView, error) {
	if r.class == nil {
		return nil, domain.ErrNotFound
	}
	return &TeachingClassView{
		ID: r.class.ID, ClassCode: r.class.ClassCode, TermID: r.class.TermID,
		CourseID: r.class.CourseID, Capacity: r.class.Capacity, State: r.class.State,
	}, nil
}
func (r *repositoryStub) GetTeachingClassForUpdate(context.Context, uint64) (*domain.TeachingClass, error) {
	r.events = append(r.events, "class-lock")
	if r.class == nil {
		return nil, domain.ErrNotFound
	}
	return r.class, nil
}
func (r *repositoryStub) InsertTeachingClass(context.Context, *domain.TeachingClass) error {
	return nil
}
func (r *repositoryStub) SaveTeachingClass(context.Context, *domain.TeachingClass) error {
	r.saveClassCalled = true
	return nil
}
func (r *repositoryStub) RemoveTeachingClass(context.Context, uint64) error { return nil }
func (r *repositoryStub) InspectTeachingClassUsage(context.Context, uint64) (domain.TeachingClassUsage, error) {
	r.events = append(r.events, "class-usage")
	return r.classUsage, nil
}
