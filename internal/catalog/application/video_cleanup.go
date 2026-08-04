package catalogapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "prizeforge/internal/catalog/domain"
)

var ErrVideoObjectStillInUse = domain.ErrVideoObjectStillInUse

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
	if !upload.EligibleForCleanup(cleanupBefore) {
		return nil
	}
	if !isManagedVideoObjectKey(upload.ObjectKey) {
		return fmt.Errorf("refuse to delete unmanaged object key %q", upload.ObjectKey)
	}

	var objectKey, multipartUploadID string
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
		if !current.EligibleForCleanup(cleanupBefore) {
			return nil
		}
		if !isManagedVideoObjectKey(current.ObjectKey) {
			return fmt.Errorf("refuse to delete unmanaged object key %q", current.ObjectKey)
		}
		expectedUploadStatus := current.Status
		var expectedVideoStatus domain.CourseVideoStatus
		if videoErr == nil {
			expectedVideoStatus = video.Status
		}
		shouldClean, err := domain.PrepareCourseVideoUploadCleanup(current, video, cleanupBefore)
		if err != nil {
			return err
		}
		if !shouldClean {
			return nil
		}
		if videoErr == nil && video.Status != expectedVideoStatus {
			if err := s.repository.SaveCourseVideo(txCtx, video, expectedVideoStatus); err != nil {
				return err
			}
		}
		if current.Status != expectedUploadStatus {
			if err := s.repository.SaveCourseVideoUpload(txCtx, current, expectedUploadStatus); err != nil {
				return err
			}
		}
		objectKey = current.ObjectKey
		multipartUploadID = current.MultipartUploadID
		shouldDelete = true
		return nil
	})
	if err != nil || !shouldDelete {
		return err
	}

	if multipartUploadID != "" {
		if err := s.objectStorage.AbortMultipartUpload(ctx, objectKey, multipartUploadID); err != nil {
			return fmt.Errorf("abort multipart upload for object %q: %w", objectKey, err)
		}
	}
	if err := s.objectStorage.DeleteObject(ctx, objectKey); err != nil {
		return fmt.Errorf("delete object %q: %w", objectKey, err)
	}
	return s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		current, err := s.repository.GetCourseVideoUploadForUpdate(txCtx, uploadID)
		if err != nil {
			return err
		}
		expectedStatus := current.Status
		if err := current.FinalizeCleanup(objectKey); err != nil {
			return err
		}
		if current.Status == expectedStatus {
			return nil
		}
		return s.repository.SaveCourseVideoUpload(txCtx, current, expectedStatus)
	})
}

func isManagedVideoObjectKey(objectKey string) bool {
	objectKey = strings.TrimSpace(objectKey)
	return strings.HasPrefix(objectKey, managedVideoObjectPrefix) && len(objectKey) > len(managedVideoObjectPrefix)
}
