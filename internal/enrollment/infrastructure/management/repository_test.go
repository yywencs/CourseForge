package roundrepo

import (
	"errors"
	"testing"

	"github.com/yywencs/courseforge/internal/enrollment/domain"

	"gorm.io/gorm"
)

func TestRequireConditionalWriteMapsZeroRowsToConcurrentConflict(t *testing.T) {
	err := requireConditionalWrite(&gorm.DB{RowsAffected: 0})
	if !errors.Is(err, enrollment.ErrConflict) {
		t.Fatalf("requireConditionalWrite() error = %v, want %v", err, enrollment.ErrConflict)
	}
}

func TestRequireConditionalWritePreservesDatabaseError(t *testing.T) {
	want := errors.New("database unavailable")
	err := requireConditionalWrite(&gorm.DB{Error: want})
	if !errors.Is(err, want) {
		t.Fatalf("requireConditionalWrite() error = %v, want %v", err, want)
	}
}
