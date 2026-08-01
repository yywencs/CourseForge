//go:build integration

package integration

import (
	enrollmentrepo "prizeforge/internal/enrollment/infrastructure/persistence"
	"prizeforge/internal/platform/cache"
	"prizeforge/internal/platform/identifier"

	"gorm.io/gorm"
)

// enrollmentRepositoryFixture is intentionally test-local. Production wiring
// consumes focused stores instead of an aggregate repository facade.
type enrollmentRepositoryFixture struct {
	*enrollmentrepo.QueryStore
	*enrollmentrepo.EligibilityStore
	*enrollmentrepo.SelectionStore
	*enrollmentrepo.ResultStore
	*enrollmentrepo.EnrollmentStore
	*enrollmentrepo.ProjectionStore
	*enrollmentrepo.RepairStore
	*enrollmentrepo.WaitlistStore
}

func newEnrollmentRepositoryFixture(
	db *gorm.DB,
	redis *cache.Cache,
) *enrollmentRepositoryFixture {
	stores := enrollmentrepo.NewStores(db, redis, identifier.NewOrderIDGenerator())
	return &enrollmentRepositoryFixture{
		QueryStore:       stores.Queries,
		EligibilityStore: stores.Eligibility,
		SelectionStore:   stores.Selections,
		ResultStore:      stores.Results,
		EnrollmentStore:  stores.Enrollments,
		ProjectionStore:  stores.Projections,
		RepairStore:      stores.Repairs,
		WaitlistStore:    stores.Waitlist,
	}
}
