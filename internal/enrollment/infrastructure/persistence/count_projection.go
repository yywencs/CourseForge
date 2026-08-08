package enrollmentrepo

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type enrollmentCountDeltaRow struct {
	ID              uint64
	TeachingClassID uint64
	Delta           int64
}

// ProjectPendingEnrollmentCounts 锁定一批未处理增量，按教学班聚合后更新计数，
// 并在同一个事务内标记已处理。事务回滚时计数和处理标记会一起回滚。
func (r *CountProjectionStore) ProjectPendingEnrollmentCounts(
	ctx context.Context,
	limit int,
	processedAt time.Time,
) (int, error) {
	if r == nil || r.db == nil || limit <= 0 || processedAt.IsZero() {
		return 0, fmt.Errorf("投影教学班计数: 参数非法")
	}
	processed := 0
	// READ COMMITTED 避免默认 REPEATABLE READ 对 processed_at 范围产生间隙锁，
	// 否则消费者追加新增量可能与投影扫描互相等待甚至死锁。
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []enrollmentCountDeltaRow
		if err := tx.Table("enrollment_count_delta").
			Select("id, teaching_class_id, delta").
			Where("processed_at IS NULL").
			Order("id ASC").
			Limit(limit).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Find(&rows).Error; err != nil {
			return err
		}
		if len(rows) == 0 {
			return nil
		}

		deltas := make(map[uint64]int64)
		ids := make([]uint64, 0, len(rows))
		for _, row := range rows {
			deltas[row.TeachingClassID] += row.Delta
			ids = append(ids, row.ID)
		}
		classIDs := make([]uint64, 0, len(deltas))
		for classID := range deltas {
			classIDs = append(classIDs, classID)
		}
		sort.Slice(classIDs, func(i, j int) bool { return classIDs[i] < classIDs[j] })
		for _, classID := range classIDs {
			delta := deltas[classID]
			if delta == 0 {
				continue
			}
			updated := tx.Table("teaching_class").
				Where(
					"id = ? AND CAST(selected_count AS SIGNED) + ? BETWEEN 0 AND capacity",
					classID,
					delta,
				).
				Updates(map[string]interface{}{
					"selected_count": gorm.Expr("CAST(selected_count AS SIGNED) + ?", delta),
					"update_time":    processedAt,
				})
			if updated.Error != nil {
				return updated.Error
			}
			if updated.RowsAffected != 1 {
				return fmt.Errorf(
					"投影教学班计数失败: teaching_class_id=%d delta=%d",
					classID,
					delta,
				)
			}
		}

		marked := tx.Table("enrollment_count_delta").
			Where("id IN ? AND processed_at IS NULL", ids).
			Update("processed_at", processedAt)
		if marked.Error != nil {
			return marked.Error
		}
		if marked.RowsAffected != int64(len(ids)) {
			return fmt.Errorf(
				"标记教学班计数增量失败: updated=%d expected=%d",
				marked.RowsAffected,
				len(ids),
			)
		}
		processed = len(ids)
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	return processed, err
}

// DeleteProcessedEnrollmentCountDeltas 小批量删除超过保留期的已处理增量。
func (r *CountProjectionStore) DeleteProcessedEnrollmentCountDeltas(
	ctx context.Context,
	before time.Time,
	limit int,
) (int64, error) {
	if r == nil || r.db == nil || limit <= 0 || before.IsZero() {
		return 0, fmt.Errorf("清理教学班计数增量: 参数非法")
	}
	result := r.db.WithContext(ctx).
		Table("enrollment_count_delta").
		Where("processed_at IS NOT NULL AND processed_at < ?", before).
		Order("id ASC").
		Limit(limit).
		Delete(map[string]interface{}{})
	return result.RowsAffected, result.Error
}
