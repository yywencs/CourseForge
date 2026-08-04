package danmakuapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"prizeforge/internal/danmaku/domain"
)

const serviceTestMessageID = "ec40a0ec-572c-4af5-9067-65f702fa666c"

type repositoryStub struct {
	insertErr error
	existing  *danmaku.Danmaku
	inserted  *danmaku.Danmaku
}

func (r *repositoryStub) Insert(_ context.Context, item *danmaku.Danmaku) error {
	copy := *item
	r.inserted = &copy
	if r.insertErr != nil {
		return r.insertErr
	}
	item.ID = 88
	item.CreateTime = time.Unix(100, 0)
	return nil
}

func (r *repositoryStub) GetByClientMessage(context.Context, uint64, uint64, string) (*danmaku.Danmaku, error) {
	if r.existing == nil {
		return nil, danmaku.ErrNotFound
	}
	copy := *r.existing
	return &copy, nil
}

type videoReaderStub struct {
	video *danmaku.VideoTarget
	err   error
}

func (r videoReaderStub) GetVideo(context.Context, uint64) (*danmaku.VideoTarget, error) {
	return r.video, r.err
}

func readyVideo(duration uint64) *danmaku.VideoTarget {
	return &danmaku.VideoTarget{
		ID: 7, Kind: danmaku.VideoKindPreview,
		Status: danmaku.VideoStatusReady, DurationMS: &duration,
	}
}

func publishCommand() PublishCommand {
	return PublishCommand{
		VideoID: 7, StudentID: 1001, ClientMessageID: serviceTestMessageID,
		VideoTimeMS: 1200, Content: "  关键内容  ",
	}
}

func TestPublishPersistsValidatedDanmaku(t *testing.T) {
	repository := &repositoryStub{}
	service := NewService(repository, videoReaderStub{video: readyVideo(5000)})
	item, err := service.Publish(context.Background(), publishCommand())
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != 88 || item.Content != "关键内容" || repository.inserted == nil {
		t.Fatalf("item = %#v, inserted = %#v", item, repository.inserted)
	}
}

func TestPublishReturnsExistingDanmakuForMatchingIdempotentRetry(t *testing.T) {
	existing, err := danmaku.New(7, 1001, serviceTestMessageID, 1200, "关键内容")
	if err != nil {
		t.Fatal(err)
	}
	existing.ID = 42
	repository := &repositoryStub{insertErr: danmaku.ErrClientMessageExists, existing: existing}
	service := NewService(repository, videoReaderStub{video: readyVideo(5000)})
	item, err := service.Publish(context.Background(), publishCommand())
	if err != nil {
		t.Fatal(err)
	}
	if item.ID != existing.ID {
		t.Fatalf("id = %d, want %d", item.ID, existing.ID)
	}
}

func TestPublishRejectsReusedIdempotencyKeyWithDifferentPayload(t *testing.T) {
	existing, err := danmaku.New(7, 1001, serviceTestMessageID, 1200, "旧内容")
	if err != nil {
		t.Fatal(err)
	}
	repository := &repositoryStub{insertErr: danmaku.ErrClientMessageExists, existing: existing}
	service := NewService(repository, videoReaderStub{video: readyVideo(5000)})
	if _, err := service.Publish(context.Background(), publishCommand()); !errors.Is(err, danmaku.ErrIdempotencyConflict) {
		t.Fatalf("err = %v", err)
	}
}

func TestPublishValidatesPlayableVideoAndDuration(t *testing.T) {
	tests := []struct {
		name    string
		video   *danmaku.VideoTarget
		command PublishCommand
		want    error
	}{
		{name: "not preview", video: &danmaku.VideoTarget{ID: 7, Kind: "lesson", Status: danmaku.VideoStatusReady}, command: publishCommand(), want: danmaku.ErrVideoNotPlayable},
		{name: "not ready", video: &danmaku.VideoTarget{ID: 7, Kind: danmaku.VideoKindPreview, Status: "uploading"}, command: publishCommand(), want: danmaku.ErrVideoNotPlayable},
		{name: "duration missing", video: &danmaku.VideoTarget{ID: 7, Kind: danmaku.VideoKindPreview, Status: danmaku.VideoStatusReady}, command: publishCommand(), want: danmaku.ErrVideoDurationUnavailable},
		{name: "past duration", video: readyVideo(1000), command: publishCommand(), want: danmaku.ErrInvalidDanmaku},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			service := NewService(&repositoryStub{}, videoReaderStub{video: test.video})
			if _, err := service.Publish(context.Background(), test.command); !errors.Is(err, test.want) {
				t.Fatalf("err = %v, want %v", err, test.want)
			}
		})
	}
}

func TestPublishPreservesVideoLookupError(t *testing.T) {
	service := NewService(&repositoryStub{}, videoReaderStub{err: danmaku.ErrVideoNotFound})
	if _, err := service.Publish(context.Background(), publishCommand()); !errors.Is(err, danmaku.ErrVideoNotFound) {
		t.Fatalf("err = %v", err)
	}
}
