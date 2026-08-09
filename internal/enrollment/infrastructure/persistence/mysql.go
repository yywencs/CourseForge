package enrollmentrepo

import (
	"context"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"

	"gorm.io/gorm"
)

// SaveSelectionResult 将 Redis 已完成的选课结果幂等写入 MySQL。
// Redis 已经原子守住实时额度与容量；MySQL 在同一事务写入选课事实、学生额度镜像和
// 教学班计数增量，避免每条消费消息竞争同一教学班计数行。
func (r *ResultStore) SaveSelectionResult(
	ctx context.Context,
	result *enrollment.SelectionResult,
) error {
	return r.SaveSelectionResults(ctx, []*enrollment.SelectionResult{result})
}

func appendEnrollmentCountDelta(
	tx *gorm.DB,
	eventID string,
	teachingClassID uint64,
	delta int8,
	createdAt time.Time,
) error {
	return tx.Table("enrollment_count_delta").Create(map[string]interface{}{
		"event_id":          eventID,
		"teaching_class_id": teachingClassID,
		"delta":             delta,
		"create_time":       createdAt,
	}).Error
}
