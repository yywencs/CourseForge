package enrollmentintegration

import (
	"errors"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
)

const (
	SelectionNotificationTopic     = "enrollment.selection.notification"
	SelectionResultPersisted       = "selection.result.persisted"
	SelectionNotificationAggregate = "selection_application"
)

// SelectionNotification is the stable integration-event payload emitted only
// after a selection result and its Outbox row commit in the same transaction.
type SelectionNotification struct {
	ApplicationID   string                      `json:"application_id"`
	StudentID       uint64                      `json:"student_id"`
	CourseID        uint64                      `json:"course_id"`
	TeachingClassID uint64                      `json:"teaching_class_id"`
	State           enrollment.ApplicationState `json:"state"`
	Failure         *Failure                    `json:"failure,omitempty"`
	CompletedAt     time.Time                   `json:"completed_at"`
}

type Failure struct {
	Code    enrollment.FailureCode `json:"code"`
	Message string                 `json:"message"`
}

func NewSelectionNotification(result *enrollment.SelectionResult) *SelectionNotification {
	if result == nil {
		return nil
	}
	event := &SelectionNotification{
		ApplicationID:   result.ApplicationID,
		StudentID:       result.StudentID,
		CourseID:        result.CourseID,
		TeachingClassID: result.TeachingClassID,
		State:           result.State,
		CompletedAt:     result.CompletedAt,
	}
	if result.Failure != nil {
		event.Failure = &Failure{
			Code:    result.Failure.Code,
			Message: result.Failure.Message,
		}
	}
	return event
}

func (e *SelectionNotification) Validate() error {
	if e == nil || e.ApplicationID == "" || e.StudentID == 0 || e.CourseID == 0 ||
		e.TeachingClassID == 0 || e.CompletedAt.IsZero() || !e.State.Terminal() {
		return errors.New("invalid selection notification")
	}
	if e.State == enrollment.ApplicationStateSelected {
		if e.Failure != nil {
			return errors.New("selected notification must not contain failure")
		}
		return nil
	}
	if e.Failure == nil || e.Failure.Code == "" || e.Failure.Message == "" {
		return errors.New("unsuccessful notification requires failure")
	}
	return nil
}
