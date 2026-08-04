package catalog

import (
	"errors"
	"testing"
	"time"
)

func TestNewCourseNormalizesDetails(t *testing.T) {
	course, err := NewCourse(CourseDetails{
		CourseCode: " CS-101 ", CourseName: " 程序设计 ", Credits: 3,
		Tags: []string{"核心", " 核心 ", ""},
	})
	if err != nil {
		t.Fatalf("NewCourse() error = %v", err)
	}
	if course.CourseCode != "CS-101" || len(course.Tags) != 1 || course.Tags[0] != "核心" {
		t.Fatalf("course = %#v", course)
	}
}

func TestCourseChangeProtectsCoreFieldsAfterTeachingStarts(t *testing.T) {
	course, err := NewCourse(CourseDetails{CourseCode: "CS-101", CourseName: "程序设计", Credits: 3})
	if err != nil {
		t.Fatal(err)
	}
	usage := CourseUsage{NonPlannedTeachingClassCount: 1}
	if err := course.Change(CourseDetails{
		CourseCode: "CS-102", CourseName: "程序设计", Credits: 3,
	}, usage); !errors.Is(err, ErrCourseCoreLocked) {
		t.Fatalf("Change() error = %v, want %v", err, ErrCourseCoreLocked)
	}
	if err := course.Change(CourseDetails{
		CourseCode: "CS-101", CourseName: "程序设计", Credits: 3,
		Introduction: "新的课程简介", Tags: []string{"更新"},
	}, usage); err != nil {
		t.Fatalf("metadata Change() error = %v", err)
	}
	if course.Introduction != "新的课程简介" {
		t.Fatalf("Introduction = %q", course.Introduction)
	}
}

func TestCourseEnsureDeletableUsesDependencyFacts(t *testing.T) {
	course := Course{ID: 1}
	if err := course.EnsureDeletable(CourseUsage{CourseVideoCount: 1}); !errors.Is(err, ErrCourseInUse) {
		t.Fatalf("EnsureDeletable() error = %v, want %v", err, ErrCourseInUse)
	}
	if err := course.EnsureDeletable(CourseUsage{}); err != nil {
		t.Fatalf("EnsureDeletable() error = %v", err)
	}
}

func TestCourseVideoLifecycle(t *testing.T) {
	video, err := NewCourseVideo(1, CourseVideoKindPreview, " 课程预览 ", "course-videos/1/a.mp4", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := video.EnsurePreviewPlayable(); !errors.Is(err, ErrCourseVideoNotPlayable) {
		t.Fatalf("uploading EnsurePreviewPlayable() error = %v", err)
	}
	duration := uint64(10_000)
	if err := video.CompleteUpload(&duration); err != nil {
		t.Fatal(err)
	}
	if err := video.EnsurePreviewPlayable(); err != nil {
		t.Fatalf("ready EnsurePreviewPlayable() error = %v", err)
	}
}

func TestCourseVideoRestartUploadUsesNewObjectKey(t *testing.T) {
	video, err := NewCourseVideo(1, CourseVideoKindPreview, "旧预览", "course-videos/1/old.mp4", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := video.RestartUpload("新预览", "course-videos/1/new.mp4"); err != nil {
		t.Fatal(err)
	}
	if video.Title != "新预览" || video.ObjectKey != "course-videos/1/new.mp4" ||
		video.Status != CourseVideoStatusUploading {
		t.Fatalf("video = %#v", video)
	}
}

func TestCourseVideoUploadPromoteRequiresPending(t *testing.T) {
	upload, err := NewCourseVideoUpload(7, "course-videos/1/upload.mp4", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := upload.Promote(); err != nil {
		t.Fatal(err)
	}
	if upload.Status != CourseVideoUploadStatusPromoted {
		t.Fatalf("status = %q, want promoted", upload.Status)
	}
	if err := upload.Promote(); !errors.Is(err, ErrCourseVideoUploadNotCompletable) {
		t.Fatalf("second Promote() error = %v, want %v", err, ErrCourseVideoUploadNotCompletable)
	}
}

func TestCourseVideoUploadCleanupLifecycleIsIdempotent(t *testing.T) {
	upload, err := NewCourseVideoUpload(7, "course-videos/1/upload.mp4", time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err := upload.Fail(); err != nil {
		t.Fatal(err)
	}
	if err := upload.Fail(); err != nil {
		t.Fatalf("repeated Fail() error = %v", err)
	}
	if err := upload.Clean(); err != nil {
		t.Fatal(err)
	}
	if err := upload.Clean(); err != nil {
		t.Fatalf("repeated Clean() error = %v", err)
	}
	if upload.Status != CourseVideoUploadStatusCleaned {
		t.Fatalf("status = %q, want cleaned", upload.Status)
	}
}

func TestTeachingClassChangePlanOwnsStateAndBindingRules(t *testing.T) {
	class := validTeachingClass()
	class.State = TeachingClassStateOpen
	if err := class.ChangePlan(validTeachingClassPlan(), TeachingClassUsage{}); !errors.Is(err, ErrTeachingClassNotEditable) {
		t.Fatalf("open ChangePlan() error = %v, want %v", err, ErrTeachingClassNotEditable)
	}

	class.State = TeachingClassStatePlanned
	changed := validTeachingClassPlan()
	changed.TermID = 202602
	if err := class.ChangePlan(changed, TeachingClassUsage{RoundBindingCount: 1}); !errors.Is(err, ErrTeachingClassTermLocked) {
		t.Fatalf("bound ChangePlan() error = %v, want %v", err, ErrTeachingClassTermLocked)
	}
}

func TestTeachingClassRejectsInvalidScheduleAndUsage(t *testing.T) {
	plan := validTeachingClassPlan()
	plan.Schedules[0].DayOfWeek = 8
	if _, err := NewTeachingClass(plan); !errors.Is(err, ErrInvalidSchedule) {
		t.Fatalf("NewTeachingClass() error = %v, want %v", err, ErrInvalidSchedule)
	}
	class := validTeachingClass()
	if err := class.EnsureDeletable(TeachingClassUsage{ApplicationCount: 1}); !errors.Is(err, ErrTeachingClassInUse) {
		t.Fatalf("EnsureDeletable() error = %v, want %v", err, ErrTeachingClassInUse)
	}
}

func validTeachingClassPlan() TeachingClassPlan {
	return TeachingClassPlan{
		ClassCode: "CS-101-01", TermID: 202601, CourseID: 1, Capacity: 30,
		Schedules: []Schedule{{DayOfWeek: 1, StartWeek: 1, EndWeek: 16, StartSection: 1, EndSection: 2}},
	}
}

func validTeachingClass() TeachingClass {
	class, err := NewTeachingClass(validTeachingClassPlan())
	if err != nil {
		panic(err)
	}
	class.ID = 1
	return *class
}
