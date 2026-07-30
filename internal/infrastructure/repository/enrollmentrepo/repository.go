package enrollmentrepo

import (
	"prizeforge/internal/domain/enrollment"
	"prizeforge/pkg/cache"
	"prizeforge/pkg/idgen"

	"gorm.io/gorm"
)

// Repository 同时实现选课只读查询、Redis-first 申请生命周期和 MySQL 结果落库。
// 当前最小版本使用独立 courseforge 单库，后续可按 student_id 拆分持久化实现。
type Repository struct {
	db    *gorm.DB
	redis *cache.Cache
	newID func() (string, error)
}

var (
	_ enrollment.QueryRepository                = (*Repository)(nil)
	_ enrollment.EligibilityRepository          = (*Repository)(nil)
	_ enrollment.EnrollmentRepository           = (*Repository)(nil)
	_ enrollment.EnrollmentProjectionRepository = (*Repository)(nil)
	_ enrollment.WaitlistRepository             = (*Repository)(nil)
	_ enrollment.ProjectionRepairRepository     = (*Repository)(nil)
	_ enrollment.ApplicationRepository          = (*Repository)(nil)
	_ enrollment.ResultPersistenceRepository    = (*Repository)(nil)
)

func NewRepository(db *gorm.DB, redis *cache.Cache) *Repository {
	return &Repository{
		db:    db,
		redis: redis,
		newID: idgen.NewOrderID,
	}
}
