package authinfra

import (
	"context"
	"errors"
	"fmt"
	"strings"

	authdomain "prizeforge/internal/domain/auth"

	"gorm.io/gorm"
)

type AccountRepository struct {
	db *gorm.DB
}

var _ authdomain.AccountRepository = (*AccountRepository)(nil)

func NewAccountRepository(db *gorm.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

type studentAccountRow struct {
	ID           uint64 `gorm:"column:id"`
	StudentNo    string `gorm:"column:student_no"`
	StudentName  string `gorm:"column:student_name"`
	PasswordHash string `gorm:"column:password_hash"`
	State        string `gorm:"column:state"`
}

func (r *AccountRepository) FindStudentByNumber(
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

func (r *AccountRepository) FindStudentByID(
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
		State:        authdomain.AccountState(r.State),
	}
}
