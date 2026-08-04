package danmakurepo

import (
	"context"
	"errors"
	"time"

	danmakuapp "prizeforge/internal/danmaku/application"
	"prizeforge/internal/danmaku/domain"

	mysqldriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

// Repository 使用 GORM 实现弹幕持久化和课程视频只读查询端口。
type Repository struct {
	db *gorm.DB
}

var (
	_ danmakuapp.Repository  = (*Repository)(nil)
	_ danmakuapp.VideoReader = (*Repository)(nil)
)

// NewRepository 创建 MySQL 弹幕仓储。
func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

type danmakuRow struct {
	ID              uint64         `gorm:"column:id;primaryKey"`
	VideoID         uint64         `gorm:"column:video_id"`
	StudentID       uint64         `gorm:"column:student_id"`
	ClientMessageID string         `gorm:"column:client_msg_id"`
	VideoTimeMS     uint64         `gorm:"column:video_time_ms"`
	Content         string         `gorm:"column:content"`
	Status          danmaku.Status `gorm:"column:status"`
	CreateTime      time.Time      `gorm:"column:create_time;autoCreateTime"`
	UpdateTime      time.Time      `gorm:"column:update_time;autoUpdateTime"`
}

func (danmakuRow) TableName() string { return "video_danmaku" }

func (r danmakuRow) domain() danmaku.Danmaku {
	return danmaku.Danmaku{
		ID: r.ID, VideoID: r.VideoID, StudentID: r.StudentID,
		ClientMessageID: r.ClientMessageID, VideoTimeMS: r.VideoTimeMS,
		Content: r.Content, Status: r.Status,
		CreateTime: r.CreateTime, UpdateTime: r.UpdateTime,
	}
}

// Insert 持久化弹幕，并回填数据库生成的主键和时间字段。
func (r *Repository) Insert(ctx context.Context, item *danmaku.Danmaku) error {
	row := danmakuRow{
		VideoID: item.VideoID, StudentID: item.StudentID,
		ClientMessageID: item.ClientMessageID, VideoTimeMS: item.VideoTimeMS,
		Content: item.Content, Status: item.Status,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return normalizeDBError(err)
	}
	*item = row.domain()
	return nil
}

// GetByClientMessage 按视频、学生和客户端消息ID查询已持久化的幂等记录。
func (r *Repository) GetByClientMessage(
	ctx context.Context,
	videoID uint64,
	studentID uint64,
	clientMessageID string,
) (*danmaku.Danmaku, error) {
	var row danmakuRow
	err := r.db.WithContext(ctx).
		Where(
			"video_id = ? AND student_id = ? AND client_msg_id = ?",
			videoID,
			studentID,
			clientMessageID,
		).
		Take(&row).Error
	if err != nil {
		return nil, normalizeDBError(err)
	}
	item := row.domain()
	return &item, nil
}

type videoRow struct {
	ID         uint64  `gorm:"column:id;primaryKey"`
	VideoKind  string  `gorm:"column:video_kind"`
	Status     string  `gorm:"column:status"`
	DurationMS *uint64 `gorm:"column:duration_ms"`
}

func (videoRow) TableName() string { return "course_video" }

// GetVideo 返回发布弹幕校验所需的课程视频最小快照。
func (r *Repository) GetVideo(ctx context.Context, videoID uint64) (*danmaku.VideoTarget, error) {
	var row videoRow
	if err := r.db.WithContext(ctx).Take(&row, "id = ?", videoID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, danmaku.ErrVideoNotFound
		}
		return nil, err
	}
	return &danmaku.VideoTarget{
		ID: row.ID, Kind: danmaku.VideoKind(row.VideoKind),
		Status: danmaku.VideoStatus(row.Status), DurationMS: row.DurationMS,
	}, nil
}

func normalizeDBError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return danmaku.ErrNotFound
	}
	var mysqlErr *mysqldriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return danmaku.ErrClientMessageExists
	}
	return err
}
