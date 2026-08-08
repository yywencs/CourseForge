package enrollmentapp

import (
	"context"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
)

// SelectionAdmissionQuery 从选课实时索引读取一次准入所需的完整快照。
type SelectionAdmissionQuery interface {
	QuerySelectionAdmission(
		context.Context,
		uint64,
		uint64,
		uint64,
		time.Time,
	) (*SelectionAdmissionSnapshot, error)
}

// SelectionQuery contains the read operations consumed by enrollment use cases.
// Cache/database lookup order is an adapter concern and is intentionally absent here.
type SelectionQuery interface {
	QuerySelectionByRequest(
		context.Context,
		uint64,
		uint64,
		string,
	) (*SelectionRequestRecord, error)
	QuerySelectionApplication(
		context.Context,
		string,
		uint64,
	) (*SelectionApplicationRecord, error)
	ListStudentEnrollments(
		context.Context,
		uint64,
		uint64,
		int,
		int,
	) (*enrollment.EnrollmentPage, error)
}

// SelectionStore 原子提交选课结果及其可靠投递记录。
type SelectionStore interface {
	CommitSelection(
		context.Context,
		*enrollment.SelectionResult,
	) (*SelectionResultPublication, error)
}

type EnrollmentStore interface {
	QueryStudentEnrollment(context.Context, string, uint64) (*enrollment.StudentEnrollment, error)
	DropEnrollment(context.Context, *enrollment.StudentEnrollment) (bool, error)
}

type EnrollmentProjection interface {
	ReleaseDroppedEnrollment(context.Context, *enrollment.StudentEnrollment) error
}

type ProjectionRepairStore interface {
	QueryPendingProjectionRepairs(
		context.Context,
		time.Time,
		int,
	) ([]*enrollment.ProjectionRepair, error)
	MarkProjectionRepairCompleted(context.Context, string, time.Time) error
	MarkProjectionRepairFailed(context.Context, string, time.Time, string) error
	CountPendingProjectionRepairs(context.Context) (int64, error)
}

// EnrollmentCountProjectionStore 原子批量应用教学班已选人数增量，并清理历史增量。
type EnrollmentCountProjectionStore interface {
	ProjectPendingEnrollmentCounts(context.Context, int, time.Time) (int, error)
	DeleteProcessedEnrollmentCountDeltas(context.Context, time.Time, int) (int64, error)
}

type WaitlistStore interface {
	JoinWaitlist(context.Context, *enrollment.WaitlistEntry) (*enrollment.WaitlistEntry, error)
	QueryWaitlist(context.Context, string, uint64) (*enrollment.WaitlistEntry, error)
	ListStudentWaitlist(
		context.Context,
		uint64,
		uint64,
		int,
		int,
	) (*enrollment.WaitlistPage, error)
	CancelWaitlist(context.Context, *enrollment.WaitlistEntry) error
	ClaimPromotableEntries(context.Context, time.Time, int) ([]*enrollment.WaitlistEntry, error)
	ClaimExpiredEntries(context.Context, time.Time, int) ([]*enrollment.WaitlistEntry, error)
	MarkWaitlistPromoted(context.Context, *enrollment.WaitlistEntry) error
	ReturnWaitlistToQueue(context.Context, *enrollment.WaitlistEntry) error
}
