package catalogrepo

import (
	"context"
	"time"

	domain "prizeforge/internal/catalog/domain"
)

type courseVideoUploadRow struct {
	ID            uint64    `gorm:"column:id;primaryKey"`
	CourseVideoID uint64    `gorm:"column:course_video_id"`
	ObjectKey     string    `gorm:"column:object_key"`
	Status        string    `gorm:"column:status"`
	ExpiresAt     time.Time `gorm:"column:expires_at"`
	CreateTime    time.Time `gorm:"column:create_time;autoCreateTime"`
	UpdateTime    time.Time `gorm:"column:update_time;autoUpdateTime"`
}

func (courseVideoUploadRow) TableName() string { return "course_video_upload" }

func (r courseVideoUploadRow) domain() domain.CourseVideoUpload {
	return domain.CourseVideoUpload{
		ID: r.ID, CourseVideoID: r.CourseVideoID, ObjectKey: r.ObjectKey,
		Status: domain.CourseVideoUploadStatus(r.Status), ExpiresAt: r.ExpiresAt,
		CreateTime: r.CreateTime, UpdateTime: r.UpdateTime,
	}
}

func (r *Repository) InsertCourseVideoUpload(ctx context.Context, upload *domain.CourseVideoUpload) error {
	row := courseVideoUploadRow{
		CourseVideoID: upload.CourseVideoID, ObjectKey: upload.ObjectKey,
		Status: string(upload.Status), ExpiresAt: upload.ExpiresAt,
	}
	if err := r.dbFor(ctx).Create(&row).Error; err != nil {
		return normalizeDBError(err)
	}
	*upload = row.domain()
	return nil
}
