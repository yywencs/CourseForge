package catalogdto

import (
	"encoding/json"
	"strings"
	"testing"

	applicationcatalog "prizeforge/internal/application/catalog"
)

func TestTeachingClassResponseUsesHTTPFieldNames(t *testing.T) {
	payload, err := json.Marshal(TeachingClass(applicationcatalog.TeachingClassView{
		ID: 7, ClassCode: "CS-101-01", CourseName: "程序设计",
	}))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	body := string(payload)
	for _, field := range []string{`"class_code"`, `"course_name"`, `"selected_count"`, `"schedules"`} {
		if !strings.Contains(body, field) {
			t.Fatalf("response %s does not contain %s", body, field)
		}
	}
	if strings.Contains(body, `"ClassCode"`) {
		t.Fatalf("response leaks Go field name: %s", body)
	}
}
