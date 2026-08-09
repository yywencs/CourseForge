package enrollmentrepo

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	application "github.com/yywencs/courseforge/internal/enrollment/application"
	"github.com/yywencs/courseforge/internal/enrollment/domain"
)

func TestSelectionResultPayloadPreservesWireContract(t *testing.T) {
	appliedAt := time.Date(2026, time.September, 1, 8, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	result := &enrollment.SelectionResult{
		ApplicationID:   "application-001",
		RequestID:       "request-001",
		RoundID:         101,
		TermID:          202601,
		StudentID:       10001,
		CourseID:        20001,
		TeachingClassID: 30001,
		Credits:         enrollment.Credit(35),
		Source:          enrollment.ApplicationSourceWeb,
		State:           enrollment.ApplicationStateRejected,
		Failure: &enrollment.FailureReason{
			Code:    enrollment.FailureCodeCreditQuota,
			Message: "credit quota exceeded",
		},
		AppliedAt:   appliedAt,
		CompletedAt: appliedAt.Add(time.Second),
	}

	raw, err := json.Marshal(newSelectionResultPayload(result))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	encoded := string(raw)
	for _, fragment := range []string{
		`"application_id":"application-001"`,
		`"teaching_class_id":30001`,
		`"failure":{"code":"CREDIT_QUOTA_EXCEEDED","message":"credit quota exceeded"}`,
		`"completed_at":`,
	} {
		if !strings.Contains(encoded, fragment) {
			t.Fatalf("payload %s does not contain %s", encoded, fragment)
		}
	}
	if strings.Contains(encoded, `"ApplicationID"`) {
		t.Fatalf("payload leaked domain field names: %s", encoded)
	}

	var decoded selectionResultPayload
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	roundTrip := decoded.toDomain()
	if err := roundTrip.Validate(); err != nil {
		t.Fatalf("round-trip result is invalid: %v", err)
	}
	if roundTrip.ApplicationID != result.ApplicationID ||
		roundTrip.Failure == nil || roundTrip.Failure.Code != result.Failure.Code {
		t.Fatalf("round-trip result = %#v", roundTrip)
	}
}

func TestSelectionPublicationPayloadPreservesRedisContract(t *testing.T) {
	completedAt := time.Date(2026, time.September, 1, 8, 0, 1, 0, time.UTC)
	publication := &application.SelectionResultPublication{
		StreamID:         "1-0",
		StreamRecorded:   true,
		DurablyPersisted: true,
		Result: &enrollment.SelectionResult{
			ApplicationID:   "application-001",
			RequestID:       "request-001",
			RoundID:         101,
			TermID:          202601,
			StudentID:       10001,
			CourseID:        20001,
			TeachingClassID: 30001,
			Credits:         enrollment.Credit(35),
			Source:          enrollment.ApplicationSourceWeb,
			State:           enrollment.ApplicationStateSelected,
			AppliedAt:       completedAt.Add(-time.Second),
			CompletedAt:     completedAt,
		},
	}

	raw, err := json.Marshal(newSelectionResultPublicationPayload(publication))
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	encoded := string(raw)
	if !strings.Contains(encoded, `"stream_id":"1-0"`) ||
		!strings.Contains(encoded, `"stream_recorded":true`) ||
		!strings.Contains(encoded, `"mysql_persisted":true`) ||
		!strings.Contains(encoded, `"result":{"application_id":`) {
		t.Fatalf("publication wire contract changed: %s", encoded)
	}
	if strings.Contains(encoded, "broker_confirmed") {
		t.Fatalf("new publication retained obsolete broker field: %s", encoded)
	}
}

func TestSelectionPublicationPayloadReadsLegacyBrokerField(t *testing.T) {
	raw := []byte(`{
		"stream_id":"1-0",
		"broker_confirmed":true,
		"mysql_persisted":false,
		"result":{
			"application_id":"application-001","request_id":"request-001",
			"round_id":101,"term_id":202601,"student_id":10001,
			"course_id":20001,"teaching_class_id":30001,"credits":35,
			"source":"web","state":"selected",
			"applied_at":"2026-09-01T08:00:00Z",
			"completed_at":"2026-09-01T08:00:01Z"
		}
	}`)
	var payload selectionResultPublicationPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if publication := payload.toApplication(); !publication.StreamRecorded {
		t.Fatal("legacy broker confirmation was not migrated to stream-recorded state")
	}
}
