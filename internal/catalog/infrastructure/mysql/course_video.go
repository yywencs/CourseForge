package catalogrepo

import (
	"context"
	"time"

	domain "prizeforge/internal/catalog/domain"

	"gorm.io/gorm/clause"
)

type courseVideoRow struct {
	ID         uint64    `gorm:"column:id;primaryKey"`
	CourseID   uint64    `gorm:"column:course_id"`
	VideoKind  string    `gorm:"column:video_kind"`
	Title      string    `gorm:"column:title"`
	ObjectKey  string    `gorm:"column:object_key"`
	Status     string    `gorm:"column:status"`
	SortOrder  uint32    `gorm:"column:sort_order"`
	DurationMS *uint64   `gorm:"column:duration_ms"`
	CreateTime time.Time `gorm:"column:create_time;autoCreateTime"`
	UpdateTime time.Time `gorm:"column:update_time;autoUpdateTime"`
}

func (courseVideoRow) TableName() string { return "course_video" }

func (r courseVideoRow) domain() domain.CourseVideo {
	return domain.CourseVideo{
		ID: r.ID, CourseID: r.CourseID, Kind: domain.CourseVideoKind(r.VideoKind),
		Title: r.Title, ObjectKey: r.ObjectKey, Status: domain.CourseVideoStatus(r.Status),
		SortOrder: r.SortOrder, DurationMS: r.DurationMS,
		CreateTime: r.CreateTime, UpdateTime: r.UpdateTime,
	}
}

func (r *Repository) ListCourseVideos(ctx context.Context, courseID uint64) ([]domain.CourseVideo, error) {
	var rows []courseVideoRow
	if err := r.dbFor(ctx).Where("course_id = ?", courseID).
		Order("video_kind ASC, sort_order ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, normalizeDBError(err)
	}
	items := make([]domain.CourseVideo, 0, len(rows))
	for _, row := range rows {
		items = append(items, row.domain())
	}
	return items, nil
}

func (r *Repository) GetCourseVideo(ctx context.Context, id uint64) (*domain.CourseVideo, error) {
	return r.getCourseVideo(ctx, id, false)
}

func (r *Repository) GetCourseVideoForUpdate(ctx context.Context, id uint64) (*domain.CourseVideo, error) {
	return r.getCourseVideo(ctx, id, true)
}

func (r *Repository) GetCourseVideoByPositionForUpdate(ctx context.Context, courseID uint64, kind domain.CourseVideoKind, sortOrder uint32) (*domain.CourseVideo, error) {
	var row courseVideoRow
	err := r.dbFor(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("course_id = ? AND video_kind = ? AND sort_order = ?", courseID, string(kind), sortOrder).
		Take(&row).Error
	if err != nil {
		return nil, normalizeDBError(err)
	}
	item := row.domain()
	return &item, nil
}

func (r *Repository) getCourseVideo(ctx context.Context, id uint64, forUpdate bool) (*domain.CourseVideo, error) {
	query := r.dbFor(ctx)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row courseVideoRow
	if err := query.Take(&row, "id = ?", id).Error; err != nil {
		return nil, normalizeDBError(err)
	}
	item := row.domain()
	return &item, nil
}

func (r *Repository) InsertCourseVideo(ctx context.Context, video *domain.CourseVideo) error {
	row := courseVideoRow{
		CourseID: video.CourseID, VideoKind: string(video.Kind), Title: video.Title,
		ObjectKey: video.ObjectKey, Status: string(video.Status), SortOrder: video.SortOrder,
		DurationMS: video.DurationMS,
	}
	if err := r.dbFor(ctx).Create(&row).Error; err != nil {
		return normalizeDBError(err)
	}
	*video = row.domain()
	return nil
}

func (r *Repository) SaveCourseVideo(ctx context.Context, video *domain.CourseVideo, expectedStatus domain.CourseVideoStatus) error {
	// 将旧状态放入更新条件，实现轻量级乐观并发控制；受影响行数为 0 时统一返回冲突。
	result := r.dbFor(ctx).Model(&courseVideoRow{}).
		Where("id = ? AND status = ?", video.ID, string(expectedStatus)).
		Updates(map[string]interface{}{
			"title": video.Title, "object_key": video.ObjectKey, "status": string(video.Status),
			"sort_order": video.SortOrder, "duration_ms": video.DurationMS,
		})
	return requireConditionalWrite(result)
}
