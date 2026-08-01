package enrollmentapp

import (
	"context"
	"time"

	"prizeforge/internal/enrollment/domain"
)

// EligibilityQuery loads the facts needed by the pure domain eligibility policy.
type EligibilityQuery interface {
	QueryEligibilitySnapshot(
		context.Context,
		uint64,
		uint64,
		uint64,
		uint64,
	) (*enrollment.EligibilitySnapshot, error)
}

// SelectionAdmissionQuery supplies the business facts needed to admit a request.
type SelectionAdmissionQuery interface {
	QuerySelectionRound(context.Context, uint64) (*enrollment.SelectionRound, error)
	QueryTeachingClass(context.Context, uint64, uint64) (*enrollment.TeachingClass, error)
	QueryStudentSelectionQuota(
		context.Context,
		uint64,
		uint64,
	) (*enrollment.StudentSelectionQuota, error)
	HasExistingEnrollment(context.Context, uint64, uint64, uint64) (bool, error)
}

// SelectionQuery contains the read operations consumed by enrollment use cases.
// Cache/database lookup order is an adapter concern and is intentionally absent here.
type SelectionQuery interface {
	SelectionAdmissionQuery
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

// SelectionStore persists the selection application lifecycle atomically.
type SelectionStore interface {
	ReserveSelection(
		context.Context,
		*enrollment.SelectionApplication,
	) (*SelectionReservation, error)
	CompleteSelection(
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
