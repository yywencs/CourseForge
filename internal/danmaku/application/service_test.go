package danmakuapp

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yywencs/courseforge/internal/danmaku/domain"
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

type segmentCacheStub struct {
	items           []danmaku.Danmaku
	getErr          error
	invalidateErr   error
	getCalls        int
	invalidateCalls int
	load            bool
	segment         danmaku.HistorySegment
}

func (c *segmentCacheStub) GetOrLoad(
	ctx context.Context,
	_ uint64,
	segment danmaku.HistorySegment,
	loader SegmentLoader,
) ([]danmaku.Danmaku, error) {
	c.getCalls++
	c.segment = segment
	if c.load {
		return loader(ctx)
	}
	return append([]danmaku.Danmaku(nil), c.items...), c.getErr
}

func (c *segmentCacheStub) Invalidate(
	_ context.Context,
	_ uint64,
	segment danmaku.HistorySegment,
) error {
	c.invalidateCalls++
	c.segment = segment
	return c.invalidateErr
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

func TestPublishInvalidatesContainingSegmentWithoutFailingOnCacheError(t *testing.T) {
	repository := &repositoryStub{}
	segmentCache := &segmentCacheStub{invalidateErr: errors.New("redis unavailable")}
	service := NewService(
		repository,
		videoReaderStub{video: readyVideo(120_000)},
		WithSegmentCache(segmentCache),
	)
	command := publishCommand()
	command.VideoTimeMS = 60_000
	if _, err := service.Publish(context.Background(), command); err != nil {
		t.Fatal(err)
	}
	if segmentCache.invalidateCalls != 1 || segmentCache.segment.Index() != 2 {
		t.Fatalf("invalidate calls = %d, segment = %d", segmentCache.invalidateCalls, segmentCache.segment.Index())
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

func TestListHistoryReturnsCachedSegmentWithoutQueryingDanmakuRepository(t *testing.T) {
	repository := &repositoryStub{listErr: errors.New("must not query database")}
	segmentCache := &segmentCacheStub{
		items: []danmaku.Danmaku{{ID: 9, VideoID: 7, VideoTimeMS: 120_500}},
	}
	service := NewService(
		repository,
		videoReaderStub{video: readyVideo(180_000)},
		WithSegmentCache(segmentCache),
	)
	page, err := service.ListHistory(context.Background(), HistoryQuery{VideoID: 7, SegmentIndex: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || page.Items[0].ID != 9 || repository.listSegment.Index() != 0 {
		t.Fatalf("page = %#v, repository segment = %d", page, repository.listSegment.Index())
	}
}

func TestListHistoryLoadsRepositoryOnCacheMiss(t *testing.T) {
	repository := &repositoryStub{listed: []danmaku.Danmaku{{ID: 2, VideoID: 7}}}
	segmentCache := &segmentCacheStub{load: true}
	service := NewService(
		repository,
		videoReaderStub{video: readyVideo(180_000)},
		WithSegmentCache(segmentCache),
	)
	page, err := service.ListHistory(context.Background(), HistoryQuery{VideoID: 7, SegmentIndex: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || repository.listSegment.Index() != 3 || segmentCache.segment.Index() != 3 {
		t.Fatalf("page = %#v, repository segment = %d, cache segment = %d", page, repository.listSegment.Index(), segmentCache.segment.Index())
	}
}

func TestListHistoryFallsBackToDatabaseWhenCacheIsUnavailable(t *testing.T) {
	repository := &repositoryStub{listed: []danmaku.Danmaku{{ID: 2, VideoID: 7}}}
	segmentCache := &segmentCacheStub{getErr: errors.New("redis unavailable")}
	service := NewService(
		repository,
		videoReaderStub{video: readyVideo(180_000)},
		WithSegmentCache(segmentCache),
	)
	page, err := service.ListHistory(context.Background(), HistoryQuery{VideoID: 7, SegmentIndex: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Items) != 1 || repository.listSegment.Index() != 3 {
		t.Fatalf("page = %#v, repository segment = %d", page, repository.listSegment.Index())
	}
}

func TestListHistoryRejectsSegmentOutsideVideo(t *testing.T) {
	service := NewService(&repositoryStub{}, videoReaderStub{video: readyVideo(119_999)})
	_, err := service.ListHistory(context.Background(), HistoryQuery{VideoID: 7, SegmentIndex: 3})
	if !errors.Is(err, danmaku.ErrInvalidHistorySegment) {
		t.Fatalf("err = %v, want %v", err, danmaku.ErrInvalidHistorySegment)
	}
}
