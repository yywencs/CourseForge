package enrollmentasync

import (
	"time"

	"prizeforge/internal/enrollment/domain"
)

// selectionResultPayload is the version-preserving RabbitMQ integration
// contract. Domain entities remain free of JSON serialization metadata.
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

type failureReasonPayload struct {
	Code    enrollment.FailureCode `json:"code"`
	Message string                 `json:"message"`
}

func newSelectionResultPayload(result *enrollment.SelectionResult) *selectionResultPayload {
	if result == nil {
		return nil
	}
	payload := &selectionResultPayload{
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
		AppliedAt:       result.AppliedAt,
		CompletedAt:     result.CompletedAt,
	}
	if result.Failure != nil {
		payload.Failure = &failureReasonPayload{
			Code:    result.Failure.Code,
			Message: result.Failure.Message,
		}
	}
	return payload
}

func (p *selectionResultPayload) toDomain() *enrollment.SelectionResult {
	if p == nil {
		return nil
	}
	result := &enrollment.SelectionResult{
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
		AppliedAt:       p.AppliedAt,
		CompletedAt:     p.CompletedAt,
	}
	if p.Failure != nil {
		result.Failure = &enrollment.FailureReason{
			Code:    p.Failure.Code,
			Message: p.Failure.Message,
		}
	}
	return result
}
