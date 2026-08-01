package enrollmentapp

import "time"

// IDGenerator supplies business identifiers without coupling application
// services to a concrete ID algorithm or global generator.
type IDGenerator interface {
	NewID() (string, error)
}

type SelectionOutcome string

const (
	SelectionOutcomeSelected            SelectionOutcome = "selected"
	SelectionOutcomeInvalidParams       SelectionOutcome = "invalid_params"
	SelectionOutcomeNotFound            SelectionOutcome = "not_found"
	SelectionOutcomeRoundNotOpen        SelectionOutcome = "round_not_open"
	SelectionOutcomeStudentInactive     SelectionOutcome = "student_inactive"
	SelectionOutcomeClassNotOpen        SelectionOutcome = "class_not_open"
	SelectionOutcomeCreditQuotaExceeded SelectionOutcome = "credit_quota_exceeded"
	SelectionOutcomeCourseQuotaExceeded SelectionOutcome = "course_quota_exceeded"
	SelectionOutcomeClassFull           SelectionOutcome = "class_full"
	SelectionOutcomeDuplicate           SelectionOutcome = "duplicate"
	SelectionOutcomePrerequisiteNotMet  SelectionOutcome = "prerequisite_not_met"
	SelectionOutcomeMajorNotAllowed     SelectionOutcome = "major_not_allowed"
	SelectionOutcomeGradeNotAllowed     SelectionOutcome = "grade_not_allowed"
	SelectionOutcomeScheduleConflict    SelectionOutcome = "schedule_conflict"
	SelectionOutcomeIdempotencyConflict SelectionOutcome = "idempotency_conflict"
	SelectionOutcomeInProgress          SelectionOutcome = "in_progress"
	SelectionOutcomeCancelled           SelectionOutcome = "cancelled"
	SelectionOutcomeError               SelectionOutcome = "error"
)

type ProjectionOperation string

const (
	ProjectionOperationDropRelease ProjectionOperation = "drop_release"
	ProjectionOperationReconcile   ProjectionOperation = "reconcile"
)

type ProjectionOutcome string

const (
	ProjectionOutcomePending ProjectionOutcome = "pending"
	ProjectionOutcomeSuccess ProjectionOutcome = "success"
	ProjectionOutcomeFailed  ProjectionOutcome = "failed"
)

type WaitlistPromotionOutcome string

const (
	WaitlistPromotionOutcomeExpired   WaitlistPromotionOutcome = "expired"
	WaitlistPromotionOutcomePromoted  WaitlistPromotionOutcome = "promoted"
	WaitlistPromotionOutcomeRetry     WaitlistPromotionOutcome = "retry"
	WaitlistPromotionOutcomeCancelled WaitlistPromotionOutcome = "cancelled"
)

// EnrollmentObserver is an application port expressed in workflow outcomes.
// Prometheus labels and collectors belong to its infrastructure adapter.
type EnrollmentObserver interface {
	SelectionCompleted(SelectionOutcome, time.Duration)
	ProjectionUpdated(ProjectionOperation, ProjectionOutcome)
	WaitlistPromotionCompleted(WaitlistPromotionOutcome)
	ProjectionRepairBacklogObserved(int64)
}
