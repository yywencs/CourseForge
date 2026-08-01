package enrollmentasync

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSelectionResultMessagePayloadUsesStableJSONNames(t *testing.T) {
	raw, err := json.Marshal(newSelectionResultPayload(testSelectionPublicationForListener()))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	encoded := string(raw)
	if !strings.Contains(encoded, `"application_id":"application-001"`) ||
		!strings.Contains(encoded, `"teaching_class_id":30001`) ||
		strings.Contains(encoded, `"ApplicationID"`) {
		t.Fatalf("message wire contract changed: %s", encoded)
	}
}
