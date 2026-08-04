package danmaku

import (
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
)

// MaxContentCharacters 是单条弹幕内容允许包含的最大 Unicode 字符数。
const MaxContentCharacters = 200

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
	if v.ID == 0 || v.ID != item.VideoID ||
		v.Kind != VideoKindPreview || v.Status != VideoStatusReady {
		return ErrVideoNotPlayable
	}
	if v.DurationMS == nil {
		return ErrVideoDurationUnavailable
	}
	if item.VideoTimeMS > *v.DurationMS {
		return ErrInvalidDanmaku
	}
	return nil
}
