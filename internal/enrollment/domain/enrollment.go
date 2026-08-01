package enrollment

import (
	"time"
)

type EnrollmentState string

const (
	EnrollmentStateEnrolled  EnrollmentState = "enrolled"
	EnrollmentStateDropped   EnrollmentState = "dropped"
	EnrollmentStateCompleted EnrollmentState = "completed"
)

func (s EnrollmentState) valid() bool {
	switch s {
	case EnrollmentStateEnrolled, EnrollmentStateDropped, EnrollmentStateCompleted:
		return true
	default:
		return false
	}
}

// StudentEnrollment 是学生正式选课记录的领域投影。
type StudentEnrollment struct {
	EnrollmentID    string
	ApplicationID   string
	RoundID         uint64
	TermID          uint64
	StudentID       uint64
	CourseID        uint64
	TeachingClassID uint64
	Credits         Credit
	State           EnrollmentState
	EnrolledAt      time.Time
	DroppedAt       *time.Time
}

func (e *StudentEnrollment) Validate() error {
	if e == nil ||
		e.EnrollmentID == "" ||
		e.ApplicationID == "" ||
		e.RoundID == 0 ||
		e.TermID == 0 ||
		e.StudentID == 0 ||
		e.CourseID == 0 ||
		e.TeachingClassID == 0 ||
		!e.Credits.Valid() ||
		!e.State.valid() ||
		e.EnrolledAt.IsZero() {
		return ErrInvalidParams
	}
	if e.State == EnrollmentStateDropped && e.DroppedAt == nil {
		return ErrInvalidParams
	}
	return nil
}

// Drop 将正式选课记录转换为已退课。重复退课是幂等成功。
func (e *StudentEnrollment) Drop(droppedAt time.Time) (bool, error) {
	if err := e.Validate(); err != nil {
		return false, err
	}
	if droppedAt.IsZero() || droppedAt.Before(e.EnrolledAt) {
		return false, ErrInvalidParams
	}
	switch e.State {
	case EnrollmentStateDropped:
		return false, nil
	case EnrollmentStateEnrolled:
		e.State = EnrollmentStateDropped
		e.DroppedAt = &droppedAt
		return true, nil
	default:
		return false, ErrInvalidEnrollmentState
	}
}

type EnrollmentPage struct {
	Items  []*StudentEnrollment
	Limit  int
	Offset int
	Total  int64
}

// ProjectionRepair records a pending repair of the enrollment read projection.
type ProjectionRepair struct {
	RepairID    string
	Enrollment  *StudentEnrollment
	RetryCount  uint32
	NextRetryAt time.Time
	LastError   string
}
