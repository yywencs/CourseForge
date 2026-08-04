package catalogrepo

import (
	"context"
	"time"

	domain "prizeforge/internal/catalog/domain"

	"gorm.io/gorm/clause"
)

type courseVideoUploadRow struct {
	ID                uint64    `gorm:"column:id;primaryKey"`
	CourseVideoID     uint64    `gorm:"column:course_video_id"`
	ObjectKey         string    `gorm:"column:object_key"`
	MultipartUploadID string    `gorm:"column:multipart_upload_id"`
	FileSize          int64     `gorm:"column:file_size"`
	Status            string    `gorm:"column:status"`
	ExpiresAt         time.Time `gorm:"column:expires_at"`
	CreateTime        time.Time `gorm:"column:create_time;autoCreateTime"`
	UpdateTime        time.Time `gorm:"column:update_time;autoUpdateTime"`
}

func (courseVideoUploadRow) TableName() string { return "course_video_upload" }

func (r courseVideoUploadRow) domain() domain.CourseVideoUpload {
	return domain.CourseVideoUpload{
		ID: r.ID, CourseVideoID: r.CourseVideoID, ObjectKey: r.ObjectKey,
		MultipartUploadID: r.MultipartUploadID, FileSize: r.FileSize,
		Status: domain.CourseVideoUploadStatus(r.Status), ExpiresAt: r.ExpiresAt,
		CreateTime: r.CreateTime, UpdateTime: r.UpdateTime,
	}
}

func (r *Repository) ListPendingCourseVideoUploadsForUpdate(
	ctx context.Context,
	courseVideoID uint64,
) ([]domain.CourseVideoUpload, error) {
	var rows []courseVideoUploadRow
	err := r.dbFor(ctx).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("course_video_id = ? AND status = ?", courseVideoID, string(domain.CourseVideoUploadStatusPending)).
		Order("id ASC").
		Find(&rows).Error
	if err != nil {
		return nil, normalizeDBError(err)
	}
	uploads := make([]domain.CourseVideoUpload, 0, len(rows))
	for _, row := range rows {
		uploads = append(uploads, row.domain())
	}
	return uploads, nil
}

func (r *Repository) InsertCourseVideoUpload(ctx context.Context, upload *domain.CourseVideoUpload) error {
	row := courseVideoUploadRow{
		CourseVideoID: upload.CourseVideoID, ObjectKey: upload.ObjectKey,
		MultipartUploadID: upload.MultipartUploadID, FileSize: upload.FileSize,
		Status: string(upload.Status), ExpiresAt: upload.ExpiresAt,
	}
	if err := r.dbFor(ctx).Create(&row).Error; err != nil {
		return normalizeDBError(err)
	}
	*upload = row.domain()
	return nil
}

func (r *Repository) GetCourseVideoUpload(ctx context.Context, id uint64) (*domain.CourseVideoUpload, error) {
	return r.getCourseVideoUpload(ctx, id, false)
}

func (r *Repository) GetCourseVideoUploadForUpdate(ctx context.Context, id uint64) (*domain.CourseVideoUpload, error) {
	return r.getCourseVideoUpload(ctx, id, true)
}

func (r *Repository) getCourseVideoUpload(ctx context.Context, id uint64, forUpdate bool) (*domain.CourseVideoUpload, error) {
	query := r.dbFor(ctx)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var row courseVideoUploadRow
	if err := query.Take(&row, "id = ?", id).Error; err != nil {
		return nil, normalizeDBError(err)
	}
	upload := row.domain()
	return &upload, nil
}

func (r *Repository) SaveCourseVideoUpload(ctx context.Context, upload *domain.CourseVideoUpload, expectedStatus domain.CourseVideoUploadStatus) error {
	result := r.dbFor(ctx).Model(&courseVideoUploadRow{}).
		Where("id = ? AND status = ?", upload.ID, string(expectedStatus)).
		Update("status", string(upload.Status))
	return requireConditionalWrite(result)
}

func (r *Repository) ListExpiredCourseVideoUploads(ctx context.Context, cleanupBefore time.Time, limit int) ([]domain.CourseVideoUpload, error) {
	if limit <= 0 {
		return []domain.CourseVideoUpload{}, nil
	}
	var rows []courseVideoUploadRow
	err := r.dbFor(ctx).
		Where("status IN ? AND expires_at <= ?", []string{
			string(domain.CourseVideoUploadStatusPending),
			string(domain.CourseVideoUploadStatusFailed),
		}, cleanupBefore).
		Order("expires_at ASC, id ASC").
		Limit(limit).
		Find(&rows).Error
	if err != nil {
		return nil, normalizeDBError(err)
	}
	uploads := make([]domain.CourseVideoUpload, 0, len(rows))
	for _, row := range rows {
		uploads = append(uploads, row.domain())
	}
	return uploads, nil
}
