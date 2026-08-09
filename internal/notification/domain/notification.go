package notification

import (
	"encoding/json"
	"errors"
	"time"
)

const TypeSelectionResult = "selection_result"

type Notification struct {
	EventID    string
	StudentID  uint64
	Type       string
	Title      string
	Content    string
	Payload    json.RawMessage
	OccurredAt time.Time
}

func (n *Notification) Validate() error {
	if n == nil || n.EventID == "" || n.StudentID == 0 || n.Type == "" ||
		n.Title == "" || n.Content == "" || !json.Valid(n.Payload) || n.OccurredAt.IsZero() {
		return errors.New("invalid notification")
	}
	return nil
}
