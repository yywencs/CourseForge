package enrollmentrepo

import (
	"time"

	application "github.com/yywencs/courseforge/internal/enrollment/application"
	"github.com/yywencs/courseforge/internal/enrollment/domain"
)

// These payloads preserve the existing Redis Stream and audit-event JSON
// schema while keeping serialization concerns out of domain entities.
type failureReasonPayload struct {
	Code    enrollment.FailureCode `json:"code"`
	Message string                 `json:"message"`
}

func newFailureReasonPayload(reason *enrollment.FailureReason) *failureReasonPayload {
	if reason == nil {
		return nil
	}
	return &failureReasonPayload{Code: reason.Code, Message: reason.Message}
}

func (p *failureReasonPayload) toDomain() *enrollment.FailureReason {
	if p == nil {
		return nil
	}
	return &enrollment.FailureReason{Code: p.Code, Message: p.Message}
}

type selectionResultPayload struct {
	ApplicationID   string                       `json:"application_id"`
	RequestID       string                       `json:"request_id"`
	RoundID         uint64                       `json:"round_id"`
	TermID          uint64                       `json:"term_id"`
	StudentID       uint64                       `json:"student_id"`
	CourseID        uint64                       `json:"course_id"`
	TeachingClassID uint64                       `json:"teaching_class_id"`
	Credits         enrollment.Credit            `json:"credits"`
	Source          enrollment.ApplicationSource `json:"source"`
	State           enrollment.ApplicationState  `json:"state"`
	Failure         *failureReasonPayload        `json:"failure,omitempty"`
	AppliedAt       time.Time                    `json:"applied_at"`
	CompletedAt     time.Time                    `json:"completed_at"`
}

func newSelectionResultPayload(result *enrollment.SelectionResult) *selectionResultPayload {
	if result == nil {
		return nil
	}
	return &selectionResultPayload{
		ApplicationID:   result.ApplicationID,
		RequestID:       result.RequestID,
		RoundID:         result.RoundID,
		TermID:          result.TermID,
		StudentID:       result.StudentID,
		CourseID:        result.CourseID,
		TeachingClassID: result.TeachingClassID,
		Credits:         result.Credits,
		Source:          result.Source,
		State:           result.State,
		Failure:         newFailureReasonPayload(result.Failure),
		AppliedAt:       result.AppliedAt,
		CompletedAt:     result.CompletedAt,
	}
}

func (p *selectionResultPayload) toDomain() *enrollment.SelectionResult {
	if p == nil {
		return nil
	}
	return &enrollment.SelectionResult{
		ApplicationID:   p.ApplicationID,
		RequestID:       p.RequestID,
		RoundID:         p.RoundID,
		TermID:          p.TermID,
		StudentID:       p.StudentID,
		CourseID:        p.CourseID,
		TeachingClassID: p.TeachingClassID,
		Credits:         p.Credits,
		Source:          p.Source,
		State:           p.State,
		Failure:         p.Failure.toDomain(),
		AppliedAt:       p.AppliedAt,
		CompletedAt:     p.CompletedAt,
	}
}

type selectionResultPublicationPayload struct {
	StreamID              string                  `json:"stream_id"`
	StreamRecorded        bool                    `json:"stream_recorded"`
	LegacyBrokerConfirmed *bool                   `json:"broker_confirmed,omitempty"`
	MySQLPersisted        bool                    `json:"mysql_persisted"`
	Result                *selectionResultPayload `json:"result"`
}

func newSelectionResultPublicationPayload(
	publication *application.SelectionResultPublication,
) *selectionResultPublicationPayload {
	if publication == nil {
		return nil
	}
	return &selectionResultPublicationPayload{
		StreamID:       publication.StreamID,
		StreamRecorded: publication.StreamRecorded,
		MySQLPersisted: publication.DurablyPersisted,
		Result:         newSelectionResultPayload(publication.Result),
	}
}

func (p *selectionResultPublicationPayload) toApplication() *application.SelectionResultPublication {
	if p == nil {
		return nil
	}
	streamRecorded := p.StreamRecorded
	if p.LegacyBrokerConfirmed != nil {
		streamRecorded = streamRecorded || *p.LegacyBrokerConfirmed
	}
	return &application.SelectionResultPublication{
		StreamID:         p.StreamID,
		StreamRecorded:   streamRecorded,
		DurablyPersisted: p.MySQLPersisted,
		Result:           p.Result.toDomain(),
	}
}

type droppedEnrollmentEventPayload struct {
	EnrollmentID    string                     `json:"EnrollmentID"`
	ApplicationID   string                     `json:"ApplicationID"`
	RoundID         uint64                     `json:"RoundID"`
	TermID          uint64                     `json:"TermID"`
	StudentID       uint64                     `json:"StudentID"`
	CourseID        uint64                     `json:"CourseID"`
	TeachingClassID uint64                     `json:"TeachingClassID"`
	Credits         enrollment.Credit          `json:"Credits"`
	State           enrollment.EnrollmentState `json:"State"`
	EnrolledAt      time.Time                  `json:"EnrolledAt"`
	DroppedAt       *time.Time                 `json:"DroppedAt"`
}

func newDroppedEnrollmentEventPayload(
	target *enrollment.StudentEnrollment,
) *droppedEnrollmentEventPayload {
	if target == nil {
		return nil
	}
	return &droppedEnrollmentEventPayload{
		EnrollmentID:    target.EnrollmentID,
		ApplicationID:   target.ApplicationID,
		RoundID:         target.RoundID,
		TermID:          target.TermID,
		StudentID:       target.StudentID,
		CourseID:        target.CourseID,
		TeachingClassID: target.TeachingClassID,
		Credits:         target.Credits,
		State:           target.State,
		EnrolledAt:      target.EnrolledAt,
		DroppedAt:       target.DroppedAt,
	}
}
