package catalogrepo

import (
	"context"
	"encoding/json"
	"errors"

	applicationcatalog "prizeforge/internal/catalog/application"
	"prizeforge/internal/catalog/domain"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

var _ applicationcatalog.Repository = (*Repository)(nil)

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type transactionContextKey struct{}

// WithinTransaction 将事务边界交给 Application 控制，并通过 context 让同一用例中的
// 多个 Repository 操作共享一个 GORM 事务。
func (r *Repository) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(context.WithValue(ctx, transactionContextKey{}, tx))
	})
	return normalizeDBError(err)
}

func (r *Repository) dbFor(ctx context.Context) *gorm.DB {
	if tx, ok := ctx.Value(transactionContextKey{}).(*gorm.DB); ok && tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}

func normalizeDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return catalog.ErrNotFound
	}
	var mysqlErr *mysqldriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return catalog.ErrConflict
	}
	return err
}

// requireConditionalWrite 只负责把条件写入失败解释为并发冲突；
// 条件背后的业务原因必须已经由 Domain 判断。
func requireConditionalWrite(result *gorm.DB) error {
	if result.Error != nil {
		return normalizeDBError(result.Error)
	}
	if result.RowsAffected != 1 {
		return catalog.ErrConflict
	}
	return nil
}

func encodeTags(tags []string) ([]byte, error) {
	if tags == nil {
		tags = []string{}
	}
	return json.Marshal(tags)
}

func decodeTags(raw []byte) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var tags []string
	if err := json.Unmarshal(raw, &tags); err != nil || tags == nil {
		return []string{}
	}
	return tags
}
