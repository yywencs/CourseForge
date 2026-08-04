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
	insertErr   error
	existing    *danmaku.Danmaku
	inserted    *danmaku.Danmaku
	listed      []danmaku.Danmaku
	listErr     error
	listVideoID uint64
	listSegment danmaku.HistorySegment
}

func (r *repositoryStub) ListVisibleSegment(
	_ context.Context,
	videoID uint64,
	segment danmaku.HistorySegment,
) ([]danmaku.Danmaku, error) {
	r.listVideoID = videoID
	r.listSegment = segment
	return append([]danmaku.Danmaku(nil), r.listed...), r.listErr
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

func TestListHistoryReturnsValidatedSixtySecondSegment(t *testing.T) {
	repository := &repositoryStub{listed: []danmaku.Danmaku{
		{ID: 2, VideoID: 7, VideoTimeMS: 120_500, Content: "第一条", Status: danmaku.StatusVisible},
		{ID: 3, VideoID: 7, VideoTimeMS: 125_000, Content: "第二条", Status: danmaku.StatusVisible},
	}}
	service := NewService(repository, videoReaderStub{video: readyVideo(180_000)})
	page, err := service.ListHistory(context.Background(), HistoryQuery{VideoID: 7, SegmentIndex: 3})
	if err != nil {
		t.Fatal(err)
	}
	if page.Segment.Index() != 3 || page.Segment.StartMS() != 120_000 ||
		page.Segment.EndMS() != 180_000 || len(page.Items) != 2 {
		t.Fatalf("page = %#v", page)
	}
	if repository.listVideoID != 7 || repository.listSegment.Index() != 3 {
		t.Fatalf("repository query video = %d, segment = %d", repository.listVideoID, repository.listSegment.Index())
	}
}

func TestListHistoryRejectsSegmentOutsideVideo(t *testing.T) {
	service := NewService(&repositoryStub{}, videoReaderStub{video: readyVideo(119_999)})
	_, err := service.ListHistory(context.Background(), HistoryQuery{VideoID: 7, SegmentIndex: 3})
	if !errors.Is(err, danmaku.ErrInvalidHistorySegment) {
		t.Fatalf("err = %v, want %v", err, danmaku.ErrInvalidHistorySegment)
	}
}
