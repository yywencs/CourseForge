package enrollmentrepo

import (
	application "github.com/yywencs/courseforge/internal/enrollment/application"
	"github.com/yywencs/courseforge/internal/platform/cache"

	"gorm.io/gorm"
)

type IDGenerator interface {
	NewID() (string, error)
}

type QueryStore struct {
	db    *gorm.DB
	redis *cache.Cache
}

type EligibilityStore struct {
	db *gorm.DB
}

type SelectionStore struct {
	redis   *cache.Cache
	queries *QueryStore
}

type ResultStore struct {
	db    *gorm.DB
	redis *cache.Cache
	ids   IDGenerator
}

type EnrollmentStore struct {
	db *gorm.DB
}

type ProjectionStore struct {
	redis *cache.Cache
}

type RepairStore struct {
	db *gorm.DB
}

type WaitlistStore struct {
	db *gorm.DB
}

// Stores exposes focused adapters for the individual application ports.
// Every adapter shares the same underlying clients but owns only one responsibility.
type Stores struct {
	Queries     *QueryStore
	Eligibility *EligibilityStore
	Selections  *SelectionStore
	Results     *ResultStore
	Enrollments *EnrollmentStore
	Projections *ProjectionStore
	Repairs     *RepairStore
	Waitlist    *WaitlistStore
}

func NewStores(db *gorm.DB, redis *cache.Cache, ids IDGenerator) *Stores {
	queries := &QueryStore{db: db, redis: redis}
	return &Stores{
		Queries:     queries,
		Eligibility: &EligibilityStore{db: db},
		Selections:  &SelectionStore{redis: redis, queries: queries},
		Results:     &ResultStore{db: db, redis: redis, ids: ids},
		Enrollments: &EnrollmentStore{db: db},
		Projections: &ProjectionStore{redis: redis},
		Repairs:     &RepairStore{db: db},
		Waitlist:    &WaitlistStore{db: db},
	}
}

var (
	_ application.SelectionQuery        = (*QueryStore)(nil)
	_ application.EligibilityQuery      = (*EligibilityStore)(nil)
	_ application.SelectionStore        = (*SelectionStore)(nil)
	_ application.EnrollmentStore       = (*EnrollmentStore)(nil)
	_ application.EnrollmentProjection  = (*ProjectionStore)(nil)
	_ application.ProjectionRepairStore = (*RepairStore)(nil)
	_ application.WaitlistStore         = (*WaitlistStore)(nil)
)
