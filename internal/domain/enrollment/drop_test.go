package enrollment

import (
	"errors"
	"testing"
	"time"
)

func TestStudentEnrollmentDropIsIdempotent(t *testing.T) {
	enrolledAt := time.Date(2026, time.September, 1, 8, 0, 0, 0, time.Local)
	target := &StudentEnrollment{
		EnrollmentID:    "enrollment-1",
		ApplicationID:   "application-1",
		RoundID:         101,
		TermID:          202601,
		StudentID:       10001,
		CourseID:        20001,
		TeachingClassID: 30001,
		Credits:         Credit(35),
		State:           EnrollmentStateEnrolled,
		EnrolledAt:      enrolledAt,
	}
	applied, err := target.Drop(enrolledAt.Add(time.Hour))
	if err != nil || !applied || target.State != EnrollmentStateDropped {
		t.Fatalf("Drop() = applied:%v err:%v state:%s", applied, err, target.State)
	}
	applied, err = target.Drop(enrolledAt.Add(2 * time.Hour))
	if err != nil || applied {
		t.Fatalf("重复 Drop() = applied:%v err:%v，期望幂等成功", applied, err)
	}
}

func TestStudentEnrollmentCannotDropCompletedCourse(t *testing.T) {
	enrolledAt := time.Now().Add(-time.Hour)
	target := &StudentEnrollment{
		EnrollmentID:    "enrollment-1",
		ApplicationID:   "application-1",
		RoundID:         101,
		TermID:          202601,
		StudentID:       10001,
		CourseID:        20001,
		TeachingClassID: 30001,
		Credits:         Credit(35),
		State:           EnrollmentStateCompleted,
		EnrolledAt:      enrolledAt,
	}
	if _, err := target.Drop(time.Now()); !errors.Is(err, ErrInvalidEnrollmentState) {
		t.Fatalf("Drop() error = %v, want %v", err, ErrInvalidEnrollmentState)
	}
}
