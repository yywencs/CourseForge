package roundrepo

import (
	"context"
	"errors"

	enrollmentapp "prizeforge/internal/enrollment/application"
	"prizeforge/internal/enrollment/domain"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

var _ enrollmentapp.RoundManagementRepository = (*Repository)(nil)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type transactionContextKey struct{}

var transactionKey transactionContextKey

func (r *Repository) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, transactionKey, tx))
	})
}

func (r *Repository) dbFor(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(transactionKey).(*gorm.DB); ok && tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

func normalizeDBError(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return enrollment.ErrNotFound
	}
	var mysqlError *mysql.MySQLError
	if errors.As(err, &mysqlError) && mysqlError.Number == 1062 {
		return enrollment.ErrConflict
	}
	return err
}

func requireConditionalWrite(result *gorm.DB) error {
	if result.Error != nil {
		return normalizeDBError(result.Error)
	}
	if result.RowsAffected != 1 {
		return enrollment.ErrConflict
	}
	return nil
}
