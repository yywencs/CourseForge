package catalogrepo

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestCourseVideoRowConfiguresAutomaticTimestamps(t *testing.T) {
	modelSchema, err := schema.Parse(&courseVideoRow{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatal(err)
	}
	createTime := modelSchema.LookUpField("CreateTime")
	if createTime == nil || createTime.AutoCreateTime == 0 {
		t.Fatal("CreateTime must be populated automatically before INSERT")
	}
	updateTime := modelSchema.LookUpField("UpdateTime")
	if updateTime == nil || updateTime.AutoUpdateTime == 0 {
		t.Fatal("UpdateTime must be populated automatically before INSERT and UPDATE")
	}
}
