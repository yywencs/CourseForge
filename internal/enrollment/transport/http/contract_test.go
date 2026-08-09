package enrollmenthttp

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
)

func TestApplicationResponseReportsStreamAndPersistenceState(t *testing.T) {
	response := applicationResponse{
		ApplicationID: "application-001",
		Failure: toFailureReasonResponse(&enrollment.FailureReason{
			Code:    enrollment.FailureCodeInternal,
			Message: "failed",
		}),
		StreamRecorded: true,
		MySQLPersisted: true,
	}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	encoded := string(raw)
	for _, fragment := range []string{
		`"stream_recorded":true`,
		`"mysql_persisted":true`,
		`"failure":{"code":"INTERNAL_ERROR","message":"failed"}`,
	} {
		if !strings.Contains(encoded, fragment) {
			t.Fatalf("response contract %s does not contain %s", encoded, fragment)
		}
	}
	if strings.Contains(encoded, "broker_confirmed") {
		t.Fatalf("response contract retained obsolete broker field: %s", encoded)
	}
}
