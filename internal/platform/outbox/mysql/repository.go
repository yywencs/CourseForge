package outboxrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/yywencs/courseforge/internal/platform/outbox"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const maxLastErrorRunes = 512

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Append stores an event as pending. To preserve Outbox atomicity, construct
// Repository with the *gorm.DB received by the caller's business transaction.
func (r *Repository) Append(ctx context.Context, event *outbox.NewEvent) error {
	return r.AppendBatch(ctx, []*outbox.NewEvent{event})
}

// AppendBatch writes integration events with one bulk INSERT. The repository
// must be constructed from the business transaction's *gorm.DB so these rows
// cannot commit independently from the corresponding aggregate changes.
func (r *Repository) AppendBatch(ctx context.Context, events []*outbox.NewEvent) error {
	if r == nil || r.db == nil {
		return errors.New("outbox repository database is nil")
	}
	if len(events) == 0 {
		return nil
	}
	now := time.Now()
	rows := make([]map[string]any, 0, len(events))
	for _, event := range events {
		if err := event.Validate(); err != nil {
			return err
		}
		rows = append(rows, map[string]any{
			"event_id":       event.EventID,
			"aggregate_type": event.AggregateType,
			"aggregate_id":   event.AggregateID,
			"topic":          event.Topic,
			"event_type":     event.EventType,
			"payload":        string(event.Payload),
			"state":          string(outbox.StatePending),
			"retry_count":    0,
			"next_retry_at":  now,
			"last_error":     "",
			"create_time":    now,
			"update_time":    now,
		})
	}
	return r.db.WithContext(ctx).Table("outbox_event").CreateInBatches(rows, len(rows)).Error
}

type eventRow struct {
	ID            uint64
	EventID       string
	AggregateType string
	AggregateID   string
	Topic         string
	EventType     string
	Payload       json.RawMessage
	State         string
	RetryCount    uint32
	NextRetryAt   time.Time
	PublishedAt   *time.Time
	LastError     string
	CreateTime    time.Time
	UpdateTime    time.Time
}

func (r *Repository) ClaimPending(
	ctx context.Context,
	limit int,
	now time.Time,
	lease time.Duration,
) ([]*outbox.Event, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("outbox repository database is nil")
	}
	if limit <= 0 {
		return nil, errors.New("outbox claim limit must be greater than zero")
	}
	if lease <= 0 {
		return nil, errors.New("outbox claim lease must be greater than zero")
	}

	var claimed []*outbox.Event
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []eventRow
		staleBefore := now.Add(-lease)
		if err := tx.
			Table("outbox_event").
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where(
				`(state IN ? AND next_retry_at <= ?)
				 OR (state = ? AND update_time <= ?)`,
				[]string{string(outbox.StatePending), string(outbox.StateFailed)},
				now,
				string(outbox.StatePublishing),
				staleBefore,
			).
			Order("id ASC").
			Limit(limit).
			Find(&rows).Error; err != nil {
			return fmt.Errorf("query dispatchable outbox events: %w", err)
		}
		if len(rows) == 0 {
			return nil
		}

		ids := make([]uint64, 0, len(rows))
		for _, row := range rows {
			ids = append(ids, row.ID)
		}
		result := tx.
			Table("outbox_event").
			Where("id IN ?", ids).
			Updates(map[string]any{
				"state":         string(outbox.StatePublishing),
				"retry_count":   gorm.Expr("retry_count + 1"),
				"next_retry_at": now.Add(lease),
				"last_error":    "",
				"update_time":   now,
			})
		if result.Error != nil {
			return fmt.Errorf("claim outbox events: %w", result.Error)
		}
		if result.RowsAffected != int64(len(rows)) {
			return fmt.Errorf(
				"claim outbox events affected %d rows, want %d",
				result.RowsAffected,
				len(rows),
			)
		}

		claimed = make([]*outbox.Event, 0, len(rows))
		for _, row := range rows {
			row.State = string(outbox.StatePublishing)
			row.RetryCount++
			row.NextRetryAt = now.Add(lease)
			row.UpdateTime = now
			claimed = append(claimed, row.toEntity())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return claimed, nil
}

func (r *Repository) MarkPublished(
	ctx context.Context,
	eventID uint64,
	publishedAt time.Time,
) error {
	if r == nil || r.db == nil {
		return errors.New("outbox repository database is nil")
	}
	if eventID == 0 || publishedAt.IsZero() {
		return errors.New("outbox published state arguments are invalid")
	}
	result := r.db.WithContext(ctx).
		Table("outbox_event").
		Where("id = ? AND state = ?", eventID, string(outbox.StatePublishing)).
		Updates(map[string]any{
			"state":        string(outbox.StatePublished),
			"published_at": publishedAt,
			"last_error":   "",
			"update_time":  publishedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("mark outbox event %d published affected %d rows", eventID, result.RowsAffected)
	}
	return nil
}

func (r *Repository) MarkFailed(
	ctx context.Context,
	eventID uint64,
	nextRetryAt time.Time,
	lastError string,
) error {
	if r == nil || r.db == nil {
		return errors.New("outbox repository database is nil")
	}
	if eventID == 0 || nextRetryAt.IsZero() {
		return errors.New("outbox failed state arguments are invalid")
	}
	result := r.db.WithContext(ctx).
		Table("outbox_event").
		Where("id = ? AND state = ?", eventID, string(outbox.StatePublishing)).
		Updates(map[string]any{
			"state":         string(outbox.StateFailed),
			"next_retry_at": nextRetryAt,
			"last_error":    truncateRunes(lastError, maxLastErrorRunes),
			"update_time":   time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return fmt.Errorf("mark outbox event %d failed affected %d rows", eventID, result.RowsAffected)
	}
	return nil
}

func (r *Repository) ReadBacklog(ctx context.Context) (*outbox.Backlog, error) {
	if r == nil || r.db == nil {
		return nil, errors.New("outbox repository database is nil")
	}
	var row struct {
		Pending       int64      `gorm:"column:pending"`
		Publishing    int64      `gorm:"column:publishing"`
		Failed        int64      `gorm:"column:failed"`
		OldestPending *time.Time `gorm:"column:oldest_pending"`
	}
	err := r.db.WithContext(ctx).Table("outbox_event").Select(`
		COALESCE(SUM(state = 'pending'), 0) AS pending,
		COALESCE(SUM(state = 'publishing'), 0) AS publishing,
		COALESCE(SUM(state = 'failed'), 0) AS failed,
		MIN(CASE WHEN state IN ('pending', 'failed') THEN create_time END) AS oldest_pending
	`).Scan(&row).Error
	if err != nil {
		return nil, fmt.Errorf("read outbox backlog: %w", err)
	}
	return &outbox.Backlog{
		Pending:       row.Pending,
		Publishing:    row.Publishing,
		Failed:        row.Failed,
		OldestPending: row.OldestPending,
	}, nil
}

func (r eventRow) toEntity() *outbox.Event {
	return &outbox.Event{
		ID:            r.ID,
		EventID:       r.EventID,
		AggregateType: r.AggregateType,
		AggregateID:   r.AggregateID,
		Topic:         r.Topic,
		EventType:     r.EventType,
		Payload:       append(json.RawMessage(nil), r.Payload...),
		State:         outbox.State(r.State),
		RetryCount:    r.RetryCount,
		NextRetryAt:   r.NextRetryAt,
		PublishedAt:   r.PublishedAt,
		LastError:     r.LastError,
		CreateTime:    r.CreateTime,
		UpdateTime:    r.UpdateTime,
	}
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
