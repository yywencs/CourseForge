package danmakurepo

import (
	"errors"
	"sync"
	"testing"

	"github.com/yywencs/courseforge/internal/danmaku/domain"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm/schema"
)

func TestDanmakuRowMappingAndTimestamps(t *testing.T) {
	modelSchema, err := schema.Parse(&danmakuRow{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	if (danmakuRow{}).TableName() != "video_danmaku" {
		t.Fatalf("table = %q", (danmakuRow{}).TableName())
	}
	if field := modelSchema.LookUpField("CreateTime"); field == nil || field.AutoCreateTime == 0 {
		t.Fatal("CreateTime must be populated automatically")
	}
	if field := modelSchema.LookUpField("UpdateTime"); field == nil || field.AutoUpdateTime == 0 {
		t.Fatal("UpdateTime must be populated automatically")
	}
}

func TestNormalizeDBErrorRecognizesDuplicateClientMessage(t *testing.T) {
	err := normalizeDBError(&mysqldriver.MySQLError{Number: 1062, Message: "duplicate"})
	if !errors.Is(err, danmaku.ErrClientMessageExists) {
		t.Fatalf("err = %v", err)
	}
}
