package enrollment

import "time"

// SelectionApplicationRecord 是申请查询结果。
// BrokerConfirmed 只描述消息是否获得 Broker Confirm，MySQLPersisted 描述是否已持久化。
type SelectionApplicationRecord struct {
	Application     *SelectionApplication
	BrokerConfirmed bool
	MySQLPersisted  bool
}

// EnrollmentState 是正式选课记录状态。
type EnrollmentState string

const (
	EnrollmentStateEnrolled  EnrollmentState = "enrolled"
	EnrollmentStateDropped   EnrollmentState = "dropped"
	EnrollmentStateCompleted EnrollmentState = "completed"
)

func (s EnrollmentState) Valid() bool {
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

// Validate 校验正式选课记录的标识、学分和状态不变量。
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
		!e.State.Valid() ||
		e.EnrolledAt.IsZero() {
		return ErrInvalidParams
	}
	if e.State == EnrollmentStateDropped && e.DroppedAt == nil {
		return ErrInvalidParams
	}
	return nil
}

// EnrollmentPage 是本人选课列表的稳定分页结果。
type EnrollmentPage struct {
	Items  []*StudentEnrollment
	Limit  int
	Offset int
	Total  int64
}
