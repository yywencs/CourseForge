package enrollmentrepo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"prizeforge/internal/domain/enrollment"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type waitlistRow struct {
	ID              uint64
	WaitlistID      string
	RequestID       string
	RoundID         uint64
	TermID          uint64
	StudentID       uint64
	CourseID        uint64
	TeachingClassID uint64
	Credits         string
	State           string
	FailureCode     string
	FailureMessage  string
	JoinedAt        timeValue
	PromotedAt      *time.Time
	CancelledAt     *time.Time
}

func (r *Repository) JoinWaitlist(
	ctx context.Context,
	entry *enrollment.WaitlistEntry,
) (*enrollment.WaitlistEntry, error) {
	if entry == nil || entry.State != enrollment.WaitlistStateWaiting {
		return nil, enrollment.ErrInvalidParams
	}
	row := waitlistRowFromEntity(entry)
	activeKey := fmt.Sprintf("%d:%d:%d", entry.RoundID, entry.StudentID, entry.TeachingClassID)
	err := r.db.WithContext(ctx).Table("selection_waitlist").Create(map[string]interface{}{
		"waitlist_id":       row.WaitlistID,
		"request_id":        row.RequestID,
		"round_id":          row.RoundID,
		"term_id":           row.TermID,
		"student_id":        row.StudentID,
		"course_id":         row.CourseID,
		"teaching_class_id": row.TeachingClassID,
		"credits":           row.Credits,
		"state":             row.State,
		"active_key":        activeKey,
		"joined_at":         entry.JoinedAt,
		"create_time":       entry.JoinedAt,
		"update_time":       entry.JoinedAt,
	}).Error
	if err != nil {
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			existing, queryErr := r.queryWaitlistByRequest(
				ctx, entry.RoundID, entry.StudentID, entry.RequestID,
			)
			if queryErr != nil {
				return nil, queryErr
			}
			if existing != nil {
				if sameWaitlistIdentity(existing, entry) {
					return existing, nil
				}
				return nil, enrollment.ErrIdempotencyConflict
			}
			return nil, enrollment.ErrWaitlistAlreadyExists
		}
		return nil, err
	}
	return r.QueryWaitlist(ctx, entry.WaitlistID, entry.StudentID)
}

func (r *Repository) QueryWaitlist(
	ctx context.Context,
	waitlistID string,
	studentID uint64,
) (*enrollment.WaitlistEntry, error) {
	if waitlistID == "" || studentID == 0 {
		return nil, enrollment.ErrInvalidParams
	}
	var row waitlistRow
	err := waitlistSelect(r.db.WithContext(ctx)).
		Where("waitlist_id = ? AND student_id = ?", waitlistID, studentID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row.toEntity()
}

func (r *Repository) queryWaitlistByRequest(
	ctx context.Context,
	roundID uint64,
	studentID uint64,
	requestID string,
) (*enrollment.WaitlistEntry, error) {
	var row waitlistRow
	err := waitlistSelect(r.db.WithContext(ctx)).
		Where("round_id = ? AND student_id = ? AND request_id = ?", roundID, studentID, requestID).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row.toEntity()
}

func (r *Repository) ListStudentWaitlist(
	ctx context.Context,
	studentID uint64,
	termID uint64,
	limit int,
	offset int,
) (*enrollment.WaitlistPage, error) {
	if studentID == 0 || termID == 0 || limit <= 0 || limit > 100 || offset < 0 {
		return nil, enrollment.ErrInvalidParams
	}
	query := r.db.WithContext(ctx).Table("selection_waitlist").
		Where("student_id = ? AND term_id = ?", studentID, termID)
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []waitlistRow
	if err := waitlistSelect(query).
		Order("id DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return nil, err
	}
	items := make([]*enrollment.WaitlistEntry, 0, len(rows))
	for index := range rows {
		item, err := rows[index].toEntity()
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return &enrollment.WaitlistPage{Items: items, Limit: limit, Offset: offset, Total: total}, nil
}

func (r *Repository) CancelWaitlist(ctx context.Context, entry *enrollment.WaitlistEntry) error {
	if entry == nil || entry.State != enrollment.WaitlistStateCancelled ||
		entry.CancelledAt == nil || entry.Failure == nil {
		return enrollment.ErrInvalidParams
	}
	result := r.db.WithContext(ctx).Table("selection_waitlist").
		Where("waitlist_id = ? AND student_id = ? AND state IN ?", entry.WaitlistID, entry.StudentID,
			[]string{string(enrollment.WaitlistStateWaiting), string(enrollment.WaitlistStatePromoting)}).
		Updates(map[string]interface{}{
			"state":           entry.State,
			"failure_code":    entry.Failure.Code,
			"failure_message": entry.Failure.Message,
			"active_key":      nil,
			"cancelled_at":    entry.CancelledAt,
			"update_time":     entry.CancelledAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		current, err := r.QueryWaitlist(ctx, entry.WaitlistID, entry.StudentID)
		if err != nil {
			return err
		}
		if current == nil {
			return enrollment.ErrRecordNotFound
		}
		if current.State != enrollment.WaitlistStateCancelled {
			return enrollment.ErrInvalidWaitlistState
		}
	}
	return nil
}

func (r *Repository) ClaimPromotableEntries(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]*enrollment.WaitlistEntry, error) {
	if now.IsZero() || limit <= 0 || limit > 100 {
		return nil, enrollment.ErrInvalidParams
	}
	var claimed []*enrollment.WaitlistEntry
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// promoting 是带租约的处理中状态。进程在晋级中崩溃时，一分钟后自动回到队列，
		// 同一个 PromotionRequestID 会让后续重试复用原选课结果。
		if err := tx.Table("selection_waitlist").
			Where("state = ? AND update_time < ?", enrollment.WaitlistStatePromoting, now.Add(-time.Minute)).
			Updates(map[string]interface{}{
				"state":       enrollment.WaitlistStateWaiting,
				"update_time": now,
			}).Error; err != nil {
			return err
		}
		var rows []waitlistRow
		err := waitlistSelect(tx).
			Joins("JOIN selection_round sr ON sr.id = selection_waitlist.round_id").
			Joins("JOIN teaching_class tc ON tc.id = selection_waitlist.teaching_class_id").
			Where(`selection_waitlist.state = ?
				AND sr.state = 'open' AND sr.start_time <= ? AND sr.end_time > ?
				AND tc.state = 'open' AND tc.selected_count < tc.capacity`,
				enrollment.WaitlistStateWaiting, now, now).
			Order("selection_waitlist.id ASC").
			Limit(limit).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Find(&rows).Error
		if err != nil {
			return err
		}
		for index := range rows {
			update := tx.Table("selection_waitlist").
				Where("id = ? AND state = ?", rows[index].ID, enrollment.WaitlistStateWaiting).
				Updates(map[string]interface{}{
					"state":       enrollment.WaitlistStatePromoting,
					"update_time": now,
				})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected == 0 {
				continue
			}
			rows[index].State = string(enrollment.WaitlistStatePromoting)
			entity, err := rows[index].toEntity()
			if err != nil {
				return err
			}
			claimed = append(claimed, entity)
		}
		return nil
	})
	return claimed, err
}

func (r *Repository) ClaimExpiredEntries(
	ctx context.Context,
	now time.Time,
	limit int,
) ([]*enrollment.WaitlistEntry, error) {
	if now.IsZero() || limit <= 0 || limit > 100 {
		return nil, enrollment.ErrInvalidParams
	}
	var claimed []*enrollment.WaitlistEntry
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []waitlistRow
		err := waitlistSelect(tx).
			Joins("JOIN selection_round sr ON sr.id = selection_waitlist.round_id").
			Where(`selection_waitlist.state = ?
				AND (sr.state = 'closed' OR sr.end_time <= ?)`,
				enrollment.WaitlistStateWaiting, now).
			Order("selection_waitlist.id ASC").
			Limit(limit).
			Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Find(&rows).Error
		if err != nil {
			return err
		}
		for index := range rows {
			update := tx.Table("selection_waitlist").
				Where("id = ? AND state = ?", rows[index].ID, enrollment.WaitlistStateWaiting).
				Updates(map[string]interface{}{
					"state":       enrollment.WaitlistStatePromoting,
					"update_time": now,
				})
			if update.Error != nil {
				return update.Error
			}
			if update.RowsAffected == 0 {
				continue
			}
			rows[index].State = string(enrollment.WaitlistStatePromoting)
			entity, err := rows[index].toEntity()
			if err != nil {
				return err
			}
			claimed = append(claimed, entity)
		}
		return nil
	})
	return claimed, err
}

func (r *Repository) MarkWaitlistPromoted(ctx context.Context, entry *enrollment.WaitlistEntry) error {
	if entry == nil || entry.State != enrollment.WaitlistStatePromoted || entry.PromotedAt == nil {
		return enrollment.ErrInvalidParams
	}
	result := r.db.WithContext(ctx).Table("selection_waitlist").
		Where("waitlist_id = ? AND state = ?", entry.WaitlistID, enrollment.WaitlistStatePromoting).
		Updates(map[string]interface{}{
			"state":       entry.State,
			"active_key":  nil,
			"promoted_at": entry.PromotedAt,
			"update_time": entry.PromotedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return enrollment.ErrInvalidWaitlistState
	}
	return nil
}

func (r *Repository) ReturnWaitlistToQueue(ctx context.Context, entry *enrollment.WaitlistEntry) error {
	if entry == nil || entry.State != enrollment.WaitlistStatePromoting {
		return enrollment.ErrInvalidParams
	}
	return r.db.WithContext(ctx).Table("selection_waitlist").
		Where("waitlist_id = ? AND state = ?", entry.WaitlistID, enrollment.WaitlistStatePromoting).
		Updates(map[string]interface{}{
			"state":       enrollment.WaitlistStateWaiting,
			"update_time": time.Now(),
		}).Error
}

func waitlistSelect(db *gorm.DB) *gorm.DB {
	return db.Table("selection_waitlist").Select(`
		selection_waitlist.id,
		selection_waitlist.waitlist_id,
		selection_waitlist.request_id,
		selection_waitlist.round_id,
		selection_waitlist.term_id,
		selection_waitlist.student_id,
		selection_waitlist.course_id,
		selection_waitlist.teaching_class_id,
		CAST(selection_waitlist.credits AS CHAR) AS credits,
		selection_waitlist.state,
		selection_waitlist.failure_code,
		selection_waitlist.failure_message,
		selection_waitlist.joined_at,
		selection_waitlist.promoted_at,
		selection_waitlist.cancelled_at
	`)
}

func (row *waitlistRow) toEntity() (*enrollment.WaitlistEntry, error) {
	credits, err := creditFromDecimal(row.Credits)
	if err != nil {
		return nil, err
	}
	entity := &enrollment.WaitlistEntry{
		WaitlistID:      row.WaitlistID,
		RequestID:       row.RequestID,
		RoundID:         row.RoundID,
		TermID:          row.TermID,
		StudentID:       row.StudentID,
		CourseID:        row.CourseID,
		TeachingClassID: row.TeachingClassID,
		Credits:         credits,
		State:           enrollment.WaitlistState(row.State),
		Position:        row.ID,
		JoinedAt:        row.JoinedAt.Time,
		PromotedAt:      row.PromotedAt,
		CancelledAt:     row.CancelledAt,
	}
	if row.FailureCode != "" {
		entity.Failure = &enrollment.FailureReason{
			Code:    enrollment.FailureCode(row.FailureCode),
			Message: row.FailureMessage,
		}
	}
	return entity, nil
}

func waitlistRowFromEntity(entry *enrollment.WaitlistEntry) waitlistRow {
	return waitlistRow{
		WaitlistID:      entry.WaitlistID,
		RequestID:       entry.RequestID,
		RoundID:         entry.RoundID,
		TermID:          entry.TermID,
		StudentID:       entry.StudentID,
		CourseID:        entry.CourseID,
		TeachingClassID: entry.TeachingClassID,
		Credits:         creditToDecimal(entry.Credits),
		State:           string(entry.State),
		JoinedAt:        timeValue{Time: entry.JoinedAt},
	}
}

func sameWaitlistIdentity(left, right *enrollment.WaitlistEntry) bool {
	return left != nil && right != nil &&
		left.RequestID == right.RequestID &&
		left.RoundID == right.RoundID &&
		left.TermID == right.TermID &&
		left.StudentID == right.StudentID &&
		left.CourseID == right.CourseID &&
		left.TeachingClassID == right.TeachingClassID &&
		left.Credits == right.Credits
}
