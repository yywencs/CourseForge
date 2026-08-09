package notificationasync

import (
	"context"
	"errors"
	"testing"

	"github.com/yywencs/courseforge/internal/notification/domain"
)

type notificationWriterStub struct {
	notification *notification.Notification
	err          error
}

func (w *notificationWriterStub) Save(_ context.Context, n *notification.Notification) error {
	w.notification = n
	return w.err
}

func TestSelectionListenerPersistsNotification(t *testing.T) {
	writer := &notificationWriterStub{}
	listener := NewSelectionListener(writer)
	body := []byte(`{
		"id":"selection:10001:application-001",
		"timestamp":"2026-09-01T08:00:01Z",
		"data":{
			"application_id":"application-001",
			"student_id":10001,
			"course_id":20001,
			"teaching_class_id":30001,
			"state":"selected",
			"completed_at":"2026-09-01T08:00:01Z"
		}
	}`)

	retry, err := listener.Handle(context.Background(), body)
	if err != nil || retry {
		t.Fatalf("Handle() = retry %v, error %v", retry, err)
	}
	if writer.notification == nil || writer.notification.EventID != "selection:10001:application-001" ||
		writer.notification.StudentID != 10001 || writer.notification.Title != "选课成功" {
		t.Fatalf("notification = %#v", writer.notification)
	}
}

func TestSelectionListenerRejectsMalformedMessagePermanently(t *testing.T) {
	retry, err := NewSelectionListener(&notificationWriterStub{}).
		Handle(context.Background(), []byte(`{"id":"","data":{}}`))
	if err == nil || retry {
		t.Fatalf("Handle() = retry %v, error %v, want permanent error", retry, err)
	}
}

func TestSelectionListenerRetriesDatabaseFailure(t *testing.T) {
	want := errors.New("mysql unavailable")
	writer := &notificationWriterStub{err: want}
	body := []byte(`{
		"id":"selection:10001:application-002",
		"data":{
			"application_id":"application-002",
			"student_id":10001,
			"course_id":20001,
			"teaching_class_id":30001,
			"state":"selected",
			"completed_at":"2026-09-01T08:00:01Z"
		}
	}`)
	retry, err := NewSelectionListener(writer).Handle(context.Background(), body)
	if !retry || !errors.Is(err, want) {
		t.Fatalf("Handle() = retry %v, error %v, want retryable database error", retry, err)
	}
}
