package query

import (
	"context"
	"errors"
	"fmt"
	"time"

	applicationapi "prizeforge/internal/application/api"

	"gorm.io/gorm"
)

type SelectionContextQuery struct {
	db  *gorm.DB
	now func() time.Time
}

var _ applicationapi.SelectionContextQuery = (*SelectionContextQuery)(nil)

func NewSelectionContextQuery(db *gorm.DB) *SelectionContextQuery {
	return &SelectionContextQuery{db: db, now: time.Now}
}

func (q *SelectionContextQuery) FindCurrentSelectionContext(
	ctx context.Context,
) (*applicationapi.SelectionContext, error) {
	var row struct {
		ID        uint64    `gorm:"column:id"`
		TermID    uint64    `gorm:"column:term_id"`
		StartTime time.Time `gorm:"column:start_time"`
		EndTime   time.Time `gorm:"column:end_time"`
	}
	now := q.now()
	err := q.db.WithContext(ctx).
		Table("selection_round").
		Select("id", "term_id", "start_time", "end_time").
		Where("state = ? AND start_time <= ? AND end_time > ?", "open", now, now).
		Order("start_time DESC, id DESC").
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query current selection round: %w", err)
	}
	return &applicationapi.SelectionContext{
		TermID:    row.TermID,
		RoundID:   row.ID,
		StartTime: row.StartTime,
		EndTime:   row.EndTime,
	}, nil
}
