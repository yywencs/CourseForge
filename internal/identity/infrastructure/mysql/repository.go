package identitymysql

import (
	"context"
	"errors"
	"fmt"
	"strings"

	authdomain "github.com/yywencs/courseforge/internal/identity/domain"

	"gorm.io/gorm"
)

type AccountRepository struct {
	db *gorm.DB
}

var _ authdomain.AccountRepository = (*AccountRepository)(nil)
var _ authdomain.AdministratorAccountRepository = (*AccountRepository)(nil)

func NewAccountRepository(db *gorm.DB) *AccountRepository {
	return &AccountRepository{db: db}
}

type studentAccountRow struct {
	StudentID    uint64 `gorm:"column:student_id"`
	AccountID    uint64 `gorm:"column:account_id"`
	StudentNo    string `gorm:"column:student_no"`
	StudentName  string `gorm:"column:student_name"`
	PasswordHash string `gorm:"column:password_hash"`
	AccountState string `gorm:"column:account_state"`
}

type administratorAccountRow struct {
	AdministratorID uint64 `gorm:"column:administrator_id"`
	AccountID       uint64 `gorm:"column:account_id"`
	Username        string `gorm:"column:username"`
	PasswordHash    string `gorm:"column:password_hash"`
	AccountState    string `gorm:"column:account_state"`
}

func (r *AccountRepository) FindStudentByNumber(
	ctx context.Context,
	studentNo string,
) (*authdomain.StudentAccount, error) {
	var row studentAccountRow
	err := r.db.WithContext(ctx).
		Table("student AS s").
		Select(`
			s.id AS student_id,
			a.id AS account_id,
			s.student_no,
			s.student_name,
			a.password_hash,
			a.state AS account_state
		`).
		Joins("JOIN user_account AS a ON a.id = s.account_id").
		Where("s.student_no = ?", strings.TrimSpace(studentNo)).
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
		Table("student AS s").
		Select(`
			s.id AS student_id,
			a.id AS account_id,
			s.student_no,
			s.student_name,
			a.password_hash,
			a.state AS account_state
		`).
		Joins("JOIN user_account AS a ON a.id = s.account_id").
		Where("s.id = ?", studentID).
		Take(&row).Error
	if err != nil {
		return nil, mapAccountQueryError(err)
	}
	return row.toEntity(), nil
}

func (r *AccountRepository) FindAdministratorByUsername(
	ctx context.Context,
	username string,
) (*authdomain.AdministratorAccount, error) {
	var row administratorAccountRow
	err := r.db.WithContext(ctx).
		Table("administrator AS administrator").
		Select(`
			administrator.id AS administrator_id,
			a.id AS account_id,
			administrator.username,
			a.password_hash,
			a.state AS account_state
		`).
		Joins("JOIN user_account AS a ON a.id = administrator.account_id").
		Where("administrator.username = ?", strings.TrimSpace(username)).
		Take(&row).Error
	if err != nil {
		return nil, mapAdministratorQueryError(err)
	}
	return row.toEntity(), nil
}

func (r *AccountRepository) FindAdministratorByID(
	ctx context.Context,
	administratorID uint64,
) (*authdomain.AdministratorAccount, error) {
	var row administratorAccountRow
	err := r.db.WithContext(ctx).
		Table("administrator AS administrator").
		Select(`
			administrator.id AS administrator_id,
			a.id AS account_id,
			administrator.username,
			a.password_hash,
			a.state AS account_state
		`).
		Joins("JOIN user_account AS a ON a.id = administrator.account_id").
		Where("administrator.id = ?", administratorID).
		Take(&row).Error
	if err != nil {
		return nil, mapAdministratorQueryError(err)
	}
	return row.toEntity(), nil
}

func mapAccountQueryError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return authdomain.ErrAccountNotFound
	}
	return fmt.Errorf("query student account: %w", err)
}

func mapAdministratorQueryError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return authdomain.ErrAdministratorNotFound
	}
	return fmt.Errorf("query administrator account: %w", err)
}

func (r studentAccountRow) toEntity() *authdomain.StudentAccount {
	return &authdomain.StudentAccount{
		ID:           r.StudentID,
		AccountID:    r.AccountID,
		StudentNo:    r.StudentNo,
		StudentName:  r.StudentName,
		PasswordHash: r.PasswordHash,
		AccountState: authdomain.AccountState(r.AccountState),
	}
}

func (r administratorAccountRow) toEntity() *authdomain.AdministratorAccount {
	return &authdomain.AdministratorAccount{
		ID:           r.AdministratorID,
		AccountID:    r.AccountID,
		Username:     r.Username,
		PasswordHash: r.PasswordHash,
		AccountState: authdomain.AccountState(r.AccountState),
	}
}
