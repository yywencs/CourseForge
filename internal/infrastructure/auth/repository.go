package authinfra

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	authdomain "prizeforge/internal/domain/auth"

	"gorm.io/gorm"
)

type Repository struct {
	db  *gorm.DB
	now func() time.Time
}

var _ authdomain.Repository = (*Repository)(nil)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db, now: time.Now}
}

type studentAccountRow struct {
	ID           uint64 `gorm:"column:id"`
	StudentNo    string `gorm:"column:student_no"`
	StudentName  string `gorm:"column:student_name"`
	PasswordHash string `gorm:"column:password_hash"`
	State        string `gorm:"column:state"`
}

func (r *Repository) FindStudentByNumber(
	ctx context.Context,
	studentNo string,
) (*authdomain.StudentAccount, error) {
	var row studentAccountRow
	err := r.db.WithContext(ctx).
		Table("student").
		Select("id", "student_no", "student_name", "password_hash", "state").
		Where("student_no = ?", strings.TrimSpace(studentNo)).
		Take(&row).Error
	if err != nil {
		return nil, mapAccountQueryError(err)
	}
	return row.toEntity(), nil
}

func (r *Repository) FindStudentByID(
	ctx context.Context,
	studentID uint64,
) (*authdomain.StudentAccount, error) {
	var row studentAccountRow
	err := r.db.WithContext(ctx).
		Table("student").
		Select("id", "student_no", "student_name", "password_hash", "state").
		Where("id = ?", studentID).
		Take(&row).Error
	if err != nil {
		return nil, mapAccountQueryError(err)
	}
	return row.toEntity(), nil
}

func (r *Repository) FindCurrentSelectionContext(
	ctx context.Context,
) (*authdomain.SelectionContext, error) {
	var row struct {
		ID        uint64    `gorm:"column:id"`
		TermID    uint64    `gorm:"column:term_id"`
		StartTime time.Time `gorm:"column:start_time"`
		EndTime   time.Time `gorm:"column:end_time"`
	}
	now := r.now()
	err := r.db.WithContext(ctx).
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
	return &authdomain.SelectionContext{
		TermID:    row.TermID,
		RoundID:   row.ID,
		StartTime: row.StartTime,
		EndTime:   row.EndTime,
	}, nil
}

func mapAccountQueryError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return authdomain.ErrAccountNotFound
	}
	return fmt.Errorf("query student account: %w", err)
}

func (r studentAccountRow) toEntity() *authdomain.StudentAccount {
	return &authdomain.StudentAccount{
		ID:           r.ID,
		StudentNo:    r.StudentNo,
		StudentName:  r.StudentName,
		PasswordHash: r.PasswordHash,
		State:        r.State,
	}
}
