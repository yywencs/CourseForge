package danmaku

import (
	"strings"
	"testing"
)

const testClientMessageID = "ec40a0ec-572c-4af5-9067-65f702fa666c"

func TestNewNormalizesDanmaku(t *testing.T) {
	item, err := New(7, 1001, "  "+strings.ToUpper(testClientMessageID)+"  ", 0, "  开始了  ")
	if err != nil {
		t.Fatal(err)
	}
	if item.ClientMessageID != testClientMessageID || item.Content != "开始了" || item.Status != StatusVisible {
		t.Fatalf("item = %#v", item)
	}
}

func TestNewRejectsInvalidDanmaku(t *testing.T) {
	tests := []struct {
		name      string
		videoID   uint64
		studentID uint64
		messageID string
		content   string
	}{
		{name: "video", studentID: 1, messageID: testClientMessageID, content: "ok"},
		{name: "student", videoID: 1, messageID: testClientMessageID, content: "ok"},
		{name: "message id", videoID: 1, studentID: 1, messageID: "not-uuid", content: "ok"},
		{name: "blank", videoID: 1, studentID: 1, messageID: testClientMessageID, content: "  "},
		{name: "too long", videoID: 1, studentID: 1, messageID: testClientMessageID, content: strings.Repeat("幕", MaxContentCharacters+1)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := New(test.videoID, test.studentID, test.messageID, 0, test.content); err != ErrInvalidDanmaku {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

func TestSameRequestComparesPersistedRequestFingerprint(t *testing.T) {
	first, err := New(7, 1001, testClientMessageID, 1200, "内容")
	if err != nil {
		t.Fatal(err)
	}
	second := *first
	second.ID = 99
	if !first.SameRequest(second) {
		t.Fatal("database-generated fields must not change the request fingerprint")
	}
	second.Content = "不同内容"
	if first.SameRequest(second) {
		t.Fatal("different content must conflict")
	}
}

func TestVideoTargetEnsureAcceptsDanmaku(t *testing.T) {
	item, err := New(7, 1001, testClientMessageID, 1200, "内容")
	if err != nil {
		t.Fatal(err)
	}
	duration := uint64(5000)
	target := VideoTarget{
		ID: 7, Kind: VideoKindPreview, Status: VideoStatusReady, DurationMS: &duration,
	}
	if err := target.EnsureAccepts(*item); err != nil {
		t.Fatal(err)
	}
}

func TestVideoTargetRejectsInvalidPublishingFacts(t *testing.T) {
	item, err := New(7, 1001, testClientMessageID, 1200, "内容")
	if err != nil {
		t.Fatal(err)
	}
	duration := uint64(5000)
	shortDuration := uint64(1000)
	tests := []struct {
		name   string
		target VideoTarget
		want   error
	}{
		{name: "different video", target: VideoTarget{ID: 8, Kind: VideoKindPreview, Status: VideoStatusReady, DurationMS: &duration}, want: ErrVideoNotPlayable},
		{name: "unsupported kind", target: VideoTarget{ID: 7, Kind: "lesson", Status: VideoStatusReady, DurationMS: &duration}, want: ErrVideoNotPlayable},
		{name: "not ready", target: VideoTarget{ID: 7, Kind: VideoKindPreview, Status: "uploading", DurationMS: &duration}, want: ErrVideoNotPlayable},
		{name: "duration missing", target: VideoTarget{ID: 7, Kind: VideoKindPreview, Status: VideoStatusReady}, want: ErrVideoDurationUnavailable},
		{name: "past duration", target: VideoTarget{ID: 7, Kind: VideoKindPreview, Status: VideoStatusReady, DurationMS: &shortDuration}, want: ErrInvalidDanmaku},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.target.EnsureAccepts(*item); err != test.want {
				t.Fatalf("err = %v, want %v", err, test.want)
			}
		})
	}
}

func TestHistorySegmentUsesFixedSixtySecondWindow(t *testing.T) {
	segment, err := NewHistorySegment(3)
	if err != nil {
		t.Fatal(err)
	}
	if segment.Index() != 3 || segment.StartMS() != 120_000 || segment.EndMS() != 180_000 {
		t.Fatalf("segment = %#v", segment)
	}
	if _, err := NewHistorySegment(0); err != ErrInvalidHistorySegment {
		t.Fatalf("NewHistorySegment(0) error = %v", err)
	}
}

func TestHistorySegmentAtUsesHalfOpenBoundaries(t *testing.T) {
	tests := []struct {
		videoTimeMS uint64
		wantIndex   uint64
	}{
		{videoTimeMS: 0, wantIndex: 1},
		{videoTimeMS: 59_999, wantIndex: 1},
		{videoTimeMS: 60_000, wantIndex: 2},
	}
	for _, test := range tests {
		segment, err := HistorySegmentAt(test.videoTimeMS)
		if err != nil {
			t.Fatal(err)
		}
		if segment.Index() != test.wantIndex {
			t.Fatalf("HistorySegmentAt(%d) = %d, want %d", test.videoTimeMS, segment.Index(), test.wantIndex)
		}
	}
}

func TestNewHistoryQueryOwnsParameterValidation(t *testing.T) {
	query, err := NewHistoryQuery(7, 3)
	if err != nil {
		t.Fatal(err)
	}
	if query.VideoID() != 7 || query.Segment().Index() != 3 {
		t.Fatalf("query video = %d, segment = %d", query.VideoID(), query.Segment().Index())
	}
	if _, err := NewHistoryQuery(0, 3); err != ErrInvalidHistoryQuery {
		t.Fatalf("zero video ID error = %v, want %v", err, ErrInvalidHistoryQuery)
	}
	if _, err := NewHistoryQuery(7, 0); err != ErrInvalidHistorySegment {
		t.Fatalf("zero segment error = %v, want %v", err, ErrInvalidHistorySegment)
	}
}

func TestVideoTargetValidatesReadableHistorySegment(t *testing.T) {
	segment, err := NewHistorySegment(3)
	if err != nil {
		t.Fatal(err)
	}
	duration := uint64(180_000)
	target := VideoTarget{
		ID: 7, Kind: VideoKindPreview, Status: VideoStatusReady, DurationMS: &duration,
	}
	if err := target.EnsureReadableHistory(7, segment); err != nil {
		t.Fatalf("EnsureReadableHistory() error = %v", err)
	}
	shortDuration := uint64(119_999)
	target.DurationMS = &shortDuration
	if err := target.EnsureReadableHistory(7, segment); err != ErrInvalidHistorySegment {
		t.Fatalf("outside segment error = %v, want %v", err, ErrInvalidHistorySegment)
	}
}
