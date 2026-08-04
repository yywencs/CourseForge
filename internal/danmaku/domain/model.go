package danmaku

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// MaxContentCharacters 是单条弹幕内容允许包含的最大 Unicode 字符数。
const MaxContentCharacters = 200

// HistorySegmentDurationMS 是单个历史弹幕分段覆盖的固定播放时长。
const HistorySegmentDurationMS uint64 = 60_000

const maxHistorySegmentIndex = ^uint64(0) / HistorySegmentDurationMS

// HistorySegment 表示从1开始编号的固定60秒历史弹幕窗口。
type HistorySegment struct {
	index   uint64
	startMS uint64
	endMS   uint64
}

// HistoryQuery 是一次历史弹幕查询的完整领域参数。
type HistoryQuery struct {
	videoID uint64
	segment HistorySegment
}

// NewHistoryQuery 校验视频标识和分段编号，创建不可变的历史查询参数。
func NewHistoryQuery(videoID, segmentIndex uint64) (HistoryQuery, error) {
	if videoID == 0 {
		return HistoryQuery{}, ErrInvalidHistoryQuery
	}
	segment, err := NewHistorySegment(segmentIndex)
	if err != nil {
		return HistoryQuery{}, err
	}
	return HistoryQuery{videoID: videoID, segment: segment}, nil
}

// VideoID 返回目标课程视频标识。
func (q HistoryQuery) VideoID() uint64 { return q.videoID }

// Segment 返回已经校验的固定历史分段。
func (q HistoryQuery) Segment() HistorySegment { return q.segment }

// NewHistorySegment 创建历史弹幕分段，并防止时间范围计算溢出。
func NewHistorySegment(index uint64) (HistorySegment, error) {
	if index == 0 || index > maxHistorySegmentIndex {
		return HistorySegment{}, ErrInvalidHistorySegment
	}
	return HistorySegment{
		index: index, startMS: (index - 1) * HistorySegmentDurationMS,
		endMS: index * HistorySegmentDurationMS,
	}, nil
}

// HistorySegmentAt 返回包含指定视频播放位置的历史弹幕分段。
func HistorySegmentAt(videoTimeMS uint64) (HistorySegment, error) {
	return NewHistorySegment(videoTimeMS/HistorySegmentDurationMS + 1)
}

// Index 返回从1开始的分段编号。
func (s HistorySegment) Index() uint64 { return s.index }

// StartMS 返回分段包含的起始播放位置。
func (s HistorySegment) StartMS() uint64 { return s.startMS }

// EndMS 返回分段不包含的结束播放位置。
func (s HistorySegment) EndMS() uint64 { return s.endMS }

// Status 表示弹幕当前的展示及治理状态。
type Status string

const (
	// StatusVisible 表示弹幕可以被历史查询和实时广播展示。
	StatusVisible Status = "visible"
	// StatusHidden 表示弹幕被管理员隐藏，但仍保留用于审核。
	StatusHidden Status = "hidden"
	// StatusDeleted 表示弹幕已被逻辑删除。
	StatusDeleted Status = "deleted"
)

// Danmaku 是一条绑定到课程视频播放位置的持久化弹幕。
type Danmaku struct {
	ID              uint64
	VideoID         uint64
	StudentID       uint64
	ClientMessageID string
	VideoTimeMS     uint64
	Content         string
	Status          Status
	CreateTime      time.Time
	UpdateTime      time.Time
}

// VideoKind 表示弹幕上下文关注的课程视频类型。
type VideoKind string

const (
	// VideoKindPreview 表示允许学生观看的课程预览视频。
	VideoKindPreview VideoKind = "preview"
)

// VideoStatus 表示弹幕上下文关注的课程视频状态。
type VideoStatus string

const (
	// VideoStatusReady 表示视频已经完成上传并可以播放。
	VideoStatusReady VideoStatus = "ready"
)

// VideoTarget 是弹幕发布用例从课程上下文取得的视频事实快照。
type VideoTarget struct {
	ID         uint64
	Kind       VideoKind
	Status     VideoStatus
	DurationMS *uint64
}

// New 校验并规范化客户端提交的数据，创建一条默认可见的弹幕。
func New(
	videoID uint64,
	studentID uint64,
	clientMessageID string,
	videoTimeMS uint64,
	content string,
) (*Danmaku, error) {
	if videoID == 0 || studentID == 0 || !utf8.ValidString(content) {
		return nil, ErrInvalidDanmaku
	}
	parsedID, err := uuid.Parse(strings.TrimSpace(clientMessageID))
	if err != nil {
		return nil, ErrInvalidDanmaku
	}
	content = strings.TrimSpace(content)
	if content == "" || utf8.RuneCountInString(content) > MaxContentCharacters {
		return nil, ErrInvalidDanmaku
	}
	return &Danmaku{
		VideoID: videoID, StudentID: studentID, ClientMessageID: parsedID.String(),
		VideoTimeMS: videoTimeMS, Content: content, Status: StatusVisible,
	}, nil
}

// SameRequest 判断两条弹幕是否来自同一个幂等请求且业务载荷一致。
// 数据库生成的主键和时间字段不参与请求指纹比较。
func (d Danmaku) SameRequest(other Danmaku) bool {
	return d.VideoID == other.VideoID && d.StudentID == other.StudentID &&
		d.ClientMessageID == other.ClientMessageID &&
		d.VideoTimeMS == other.VideoTimeMS && d.Content == other.Content
}

// EnsureAccepts 判断视频是否允许接收指定播放位置的弹幕。
func (v VideoTarget) EnsureAccepts(item Danmaku) error {
	if v.ID != item.VideoID {
		return ErrVideoNotPlayable
	}
	if err := v.ensurePlayable(); err != nil {
		return err
	}
	if v.DurationMS == nil {
		return ErrVideoDurationUnavailable
	}
	if item.VideoTimeMS > *v.DurationMS {
		return ErrInvalidDanmaku
	}
	return nil
}

// EnsureReadableHistory 判断视频是否允许读取指定历史弹幕分段。
func (v VideoTarget) EnsureReadableHistory(videoID uint64, segment HistorySegment) error {
	if v.ID != videoID {
		return ErrVideoNotPlayable
	}
	if err := v.ensurePlayable(); err != nil {
		return err
	}
	if v.DurationMS == nil {
		return ErrVideoDurationUnavailable
	}
	if segment.Index() == 0 || segment.StartMS() > *v.DurationMS {
		return ErrInvalidHistorySegment
	}
	return nil
}

func (v VideoTarget) ensurePlayable() error {
	if v.ID == 0 || v.Kind != VideoKindPreview || v.Status != VideoStatusReady {
		return ErrVideoNotPlayable
	}
	return nil
}
