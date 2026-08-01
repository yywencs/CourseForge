package catalogrepo

import (
	"errors"
	"testing"

	"prizeforge/internal/catalog/domain"

	"gorm.io/gorm"
)

func TestRequireConditionalWriteMapsZeroRowsToConcurrentConflict(t *testing.T) {
	err := requireConditionalWrite(&gorm.DB{RowsAffected: 0})
	if !errors.Is(err, catalog.ErrConflict) {
		t.Fatalf("requireConditionalWrite() error = %v, want %v", err, catalog.ErrConflict)
	}
}

func TestRequireConditionalWritePreservesDatabaseError(t *testing.T) {
	want := errors.New("database unavailable")
	err := requireConditionalWrite(&gorm.DB{Error: want})
	if !errors.Is(err, want) {
		t.Fatalf("requireConditionalWrite() error = %v, want %v", err, want)
	}
}
