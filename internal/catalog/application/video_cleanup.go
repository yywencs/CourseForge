package catalogapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "prizeforge/internal/catalog/domain"
)

var ErrVideoObjectStillInUse = errors.New("视频对象仍被可播放记录引用")

const managedVideoObjectPrefix = "course-videos/"

// CleanupExpiredCourseVideoUploads 批量扫描候选记录；每条记录独立提交，单条失败不会阻塞其余对象。
func (s *Service) CleanupExpiredCourseVideoUploads(ctx context.Context, cleanupBefore time.Time, limit int) error {
	if s.objectStorage == nil {
		return ErrVideoStorageUnavailable
	}
	if limit <= 0 {
		return nil
	}
	uploads, err := s.repository.ListExpiredCourseVideoUploads(ctx, cleanupBefore, limit)
	if err != nil {
		return err
	}
	var cleanupErrors []error
	for _, upload := range uploads {
		if err := s.CleanupCourseVideoUpload(ctx, upload.ID, cleanupBefore); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("cleanup video upload %d: %w", upload.ID, err))
		}
	}
	return errors.Join(cleanupErrors...)
}

// CleanupCourseVideoUpload 幂等清理一条过期上传。数据库先撤销完成资格，OSS 删除成功后
// 再标记 cleaned；任一步中断都会留下可由下一轮安全重试的 pending/failed 状态。
func (s *Service) CleanupCourseVideoUpload(ctx context.Context, uploadID uint64, cleanupBefore time.Time) error {
	if s.objectStorage == nil {
		return ErrVideoStorageUnavailable
	}
	upload, err := s.repository.GetCourseVideoUpload(ctx, uploadID)
	if err != nil {
		return err
	}
	if !courseVideoUploadCanBeCleaned(*upload, cleanupBefore) {
		return nil
	}
	if !isManagedVideoObjectKey(upload.ObjectKey) {
		return fmt.Errorf("refuse to delete unmanaged object key %q", upload.ObjectKey)
	}

	var objectKey string
	shouldDelete := false
	err = s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		// 与申请、完成上传保持 video → upload 的加锁顺序。
		video, videoErr := s.repository.GetCourseVideoForUpdate(txCtx, upload.CourseVideoID)
		if videoErr != nil && !errors.Is(videoErr, domain.ErrNotFound) {
			return videoErr
		}
		current, err := s.repository.GetCourseVideoUploadForUpdate(txCtx, uploadID)
		if err != nil {
			return err
		}
		if !courseVideoUploadCanBeCleaned(*current, cleanupBefore) {
			return nil
		}
		if !isManagedVideoObjectKey(current.ObjectKey) {
			return fmt.Errorf("refuse to delete unmanaged object key %q", current.ObjectKey)
		}
		if videoErr == nil && video.ObjectKey == current.ObjectKey {
			switch video.Status {
			case domain.CourseVideoStatusUploading:
				expectedStatus := video.Status
				if err := video.FailUpload(); err != nil {
					return err
				}
				if err := s.repository.SaveCourseVideo(txCtx, video, expectedStatus); err != nil {
					return err
				}
			case domain.CourseVideoStatusReady:
				return ErrVideoObjectStillInUse
			}
		}
		expectedStatus := current.Status
		if err := current.Fail(); err != nil {
			return err
		}
		if current.Status != expectedStatus {
			if err := s.repository.SaveCourseVideoUpload(txCtx, current, expectedStatus); err != nil {
				return err
			}
		}
		objectKey = current.ObjectKey
		shouldDelete = true
		return nil
	})
	if err != nil || !shouldDelete {
		return err
	}

	if err := s.objectStorage.DeleteObject(ctx, objectKey); err != nil {
		return fmt.Errorf("delete object %q: %w", objectKey, err)
	}
	return s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		current, err := s.repository.GetCourseVideoUploadForUpdate(txCtx, uploadID)
		if err != nil {
			return err
		}
		if current.Status == domain.CourseVideoUploadStatusCleaned {
			return nil
		}
		if current.Status != domain.CourseVideoUploadStatusFailed || current.ObjectKey != objectKey {
			return domain.ErrCourseVideoUploadNotCompletable
		}
		expectedStatus := current.Status
		if err := current.Clean(); err != nil {
			return err
		}
		return s.repository.SaveCourseVideoUpload(txCtx, current, expectedStatus)
	})
}

func courseVideoUploadCanBeCleaned(upload domain.CourseVideoUpload, cleanupBefore time.Time) bool {
	if upload.ExpiresAt.After(cleanupBefore) {
		return false
	}
	return upload.Status == domain.CourseVideoUploadStatusPending ||
		upload.Status == domain.CourseVideoUploadStatusFailed
}

func isManagedVideoObjectKey(objectKey string) bool {
	objectKey = strings.TrimSpace(objectKey)
	return strings.HasPrefix(objectKey, managedVideoObjectPrefix) && len(objectKey) > len(managedVideoObjectPrefix)
}
