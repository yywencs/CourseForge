package enrollmentrepo

import (
	"database/sql/driver"
	"fmt"
	"time"
)

// timeValue 兼容 MySQL driver 返回的 time.Time，并保持查询投影结构简洁。
type timeValue struct {
	time.Time
}

func (v *timeValue) Scan(src interface{}) error {
	switch value := src.(type) {
	case time.Time:
		v.Time = value
		return nil
	case nil:
		v.Time = time.Time{}
		return nil
	default:
		return fmt.Errorf("无法扫描时间类型 %T", src)
	}
}

func (v timeValue) Value() (driver.Value, error) {
	return v.Time, nil
}
