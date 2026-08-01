package enrollmentapp

import (
	"strings"

	"prizeforge/internal/enrollment/domain"
)

// SelectionApplicationRecord is the application-layer read model for the
// asynchronous selection workflow. Transport adapters retain legacy wire names.
type SelectionApplicationRecord struct {
	Application       *enrollment.SelectionApplication
	DeliveryConfirmed bool
	DurablyPersisted  bool
}

// SelectionRequestRecord is the idempotency lookup result used by SelectCourse.
type SelectionRequestRecord struct {
	Application      *enrollment.SelectionApplication
	Publication      *SelectionResultPublication
	DurablyPersisted bool
}

type ReservationStatus string

const (
	ReservationStatusAcquired  ReservationStatus = "acquired"
	ReservationStatusReused    ReservationStatus = "reused"
	ReservationStatusCompleted ReservationStatus = "completed"
)

type SelectionReservation struct {
	Status      ReservationStatus
	Application *enrollment.SelectionApplication
	Publication *SelectionResultPublication
}

// SelectionResultPublication is the reliable-delivery state consumed by the
// application workflow and messaging adapter.
type SelectionResultPublication struct {
	DeliveryCursor    string
	DeliveryConfirmed bool
	Result            *enrollment.SelectionResult
}

func (p *SelectionResultPublication) Validate() error {
	if p == nil || strings.TrimSpace(p.DeliveryCursor) == "" || p.Result == nil {
		return enrollment.ErrInvalidParams
	}
	return p.Result.Validate()
}
