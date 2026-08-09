package notificationmysql

import (
	"context"
	"errors"

	"github.com/yywencs/courseforge/internal/notification/domain"
	"github.com/yywencs/courseforge/internal/platform/observability/metrics"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

// Save is idempotent by event_id. RabbitMQ may redeliver after MySQL commits
// but before the consumer ACK reaches the broker.
func (r *Repository) Save(ctx context.Context, n *notification.Notification) error {
	if r == nil || r.db == nil {
		return errors.New("notification repository database is nil")
	}
	if err := n.Validate(); err != nil {
		return err
	}
	result := r.db.WithContext(ctx).Table("student_notification").
		// This repository is MySQL-specific. INSERT IGNORE keeps event_id
		// idempotency while still reporting RowsAffected=0 for duplicates.
		// GORM cannot infer a conflict update column from the map below and
		// otherwise emits an invalid empty "ON DUPLICATE KEY UPDATE" clause.
		Clauses(clause.Insert{Modifier: "IGNORE"}).
		Create(map[string]any{
			"event_id":    n.EventID,
			"student_id":  n.StudentID,
			"type":        n.Type,
			"title":       n.Title,
			"content":     n.Content,
			"payload":     string(n.Payload),
			"occurred_at": n.OccurredAt,
		})
	if result.Error != nil {
		metrics.IncNotificationPersistence(n.Type, "error")
		return result.Error
	}
	if result.RowsAffected == 0 {
		metrics.IncNotificationPersistence(n.Type, "duplicate")
	} else {
		metrics.IncNotificationPersistence(n.Type, "created")
	}
	return nil
}
