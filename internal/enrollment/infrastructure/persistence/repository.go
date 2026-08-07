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
	redis *cache.Cache
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
	Queries          *QueryStore
	Eligibility      *EligibilityStore
	Selections       *SelectionStore
	Results          *ResultStore
	Enrollments      *EnrollmentStore
	Projections      *ProjectionStore
	Repairs          *RepairStore
	Waitlist         *WaitlistStore
	EligibilityIndex *EligibilityIndex
}

func NewStores(db *gorm.DB, redis *cache.Cache, ids IDGenerator) *Stores {
	queries := &QueryStore{db: db, redis: redis}
	eligibilityIndex := NewEligibilityIndex(redis)
	return &Stores{
		Queries:          queries,
		Eligibility:      &EligibilityStore{db: db},
		Selections:       &SelectionStore{redis: redis},
		Results:          &ResultStore{db: db, redis: redis, ids: ids},
		Enrollments:      &EnrollmentStore{db: db},
		Projections:      &ProjectionStore{redis: redis},
		Repairs:          &RepairStore{db: db},
		Waitlist:         &WaitlistStore{db: db},
		EligibilityIndex: eligibilityIndex,
	}
}

var (
	_ application.SelectionQuery          = (*QueryStore)(nil)
	_ application.RoundWarmupSource       = (*EligibilityStore)(nil)
	_ application.EligibilityWarmupIndex  = (*EligibilityIndex)(nil)
	_ application.SelectionAdmissionQuery = (*EligibilityIndex)(nil)
	_ application.SelectionStore          = (*SelectionStore)(nil)
	_ application.EnrollmentStore         = (*EnrollmentStore)(nil)
	_ application.EnrollmentProjection    = (*ProjectionStore)(nil)
	_ application.ProjectionRepairStore   = (*RepairStore)(nil)
	_ application.WaitlistStore           = (*WaitlistStore)(nil)
)
