package enrollmenthttp

import (
	"encoding/json"
	"strings"
	"testing"

	"prizeforge/internal/enrollment/domain"
)

func TestApplicationResponseRetainsLegacyDeliveryAndFailureFields(t *testing.T) {
	response := applicationResponse{
		ApplicationID: "application-001",
		Failure: toFailureReasonResponse(&enrollment.FailureReason{
			Code:    enrollment.FailureCodeInternal,
			Message: "failed",
		}),
		BrokerConfirmed: true,
		MySQLPersisted:  true,
	}
	raw, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	encoded := string(raw)
	for _, fragment := range []string{
		`"broker_confirmed":true`,
		`"mysql_persisted":true`,
		`"failure":{"code":"INTERNAL_ERROR","message":"failed"}`,
	} {
		if !strings.Contains(encoded, fragment) {
			t.Fatalf("response contract %s does not contain %s", encoded, fragment)
		}
	}
}
