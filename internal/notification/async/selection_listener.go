package notificationasync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
	enrollmentintegration "github.com/yywencs/courseforge/internal/enrollment/integration"
	"github.com/yywencs/courseforge/internal/notification/domain"
)

type notificationWriter interface {
	Save(context.Context, *notification.Notification) error
}

type selectionEventEnvelope struct {
	ID   string          `json:"id"`
	Data json.RawMessage `json:"data"`
}

type SelectionListener struct {
	writer notificationWriter
}

func NewSelectionListener(writer notificationWriter) *SelectionListener {
	return &SelectionListener{writer: writer}
}

func (l *SelectionListener) Handle(ctx context.Context, body []byte) (bool, error) {
	if l == nil || l.writer == nil {
		return true, errors.New("selection notification listener dependency is missing")
	}
	var envelope selectionEventEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil || envelope.ID == "" || len(envelope.Data) == 0 {
		return false, fmt.Errorf("decode selection notification envelope: %w", invalidMessageError(err))
	}
	var event enrollmentintegration.SelectionNotification
	if err := json.Unmarshal(envelope.Data, &event); err != nil {
		return false, fmt.Errorf("decode selection notification payload: %w", err)
	}
	if err := event.Validate(); err != nil {
		return false, err
	}
	title, content := selectionNotificationCopy(&event)
	n := &notification.Notification{
		EventID:    envelope.ID,
		StudentID:  event.StudentID,
		Type:       notification.TypeSelectionResult,
		Title:      title,
		Content:    content,
		Payload:    append(json.RawMessage(nil), envelope.Data...),
		OccurredAt: event.CompletedAt,
	}
	if err := l.writer.Save(ctx, n); err != nil {
		return true, err
	}
	return false, nil
}

func selectionNotificationCopy(event *enrollmentintegration.SelectionNotification) (string, string) {
	if event.State == enrollment.ApplicationStateSelected {
		return "选课成功", fmt.Sprintf(
			"课程 %d（教学班 %d）已选课成功",
			event.CourseID,
			event.TeachingClassID,
		)
	}
	reason := event.Failure.Message
	if event.State == enrollment.ApplicationStateCancelled {
		return "选课已取消", fmt.Sprintf("课程 %d 的选课申请已取消：%s", event.CourseID, reason)
	}
	return "选课失败", fmt.Sprintf("课程 %d 选课失败：%s", event.CourseID, reason)
}

func invalidMessageError(err error) error {
	if err != nil {
		return err
	}
	return errors.New("missing event id or data")
}
