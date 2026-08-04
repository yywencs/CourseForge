package catalogapp

import (
	"context"
	"errors"
	"testing"
	"time"

	domain "prizeforge/internal/catalog/domain"
)

func TestCleanupMissingObjectIsIdempotent(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	repository := cleanupRepository(now, domain.CourseVideoUploadStatusFailed)
	storage := &objectStorageStub{objectExists: false}
	service := NewService(repository, WithVideoStorage(storage, VideoPolicy{}))

	if err := service.CleanupCourseVideoUpload(context.Background(), 11, now.Add(-time.Hour)); err != nil {
		t.Fatalf("CleanupCourseVideoUpload() error = %v", err)
	}
	if repository.upload.Status != domain.CourseVideoUploadStatusCleaned {
		t.Fatalf("upload status = %q, want cleaned", repository.upload.Status)
	}
	if storage.deleteObjectCalls != 1 {
		t.Fatalf("DeleteObject() calls = %d, want 1", storage.deleteObjectCalls)
	}
	if err := service.CleanupCourseVideoUpload(context.Background(), 11, now.Add(-time.Hour)); err != nil {
		t.Fatalf("repeated CleanupCourseVideoUpload() error = %v", err)
	}
	if storage.deleteObjectCalls != 1 {
		t.Fatalf("DeleteObject() calls after retry = %d, want 1", storage.deleteObjectCalls)
	}
}

func TestCleanupDeleteFailureLeavesRecoverableFailedState(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	repository := cleanupRepository(now, domain.CourseVideoUploadStatusPending)
	wantErr := errors.New("object storage unavailable")
	storage := &objectStorageStub{objectExists: true, deleteErr: wantErr}
	service := NewService(repository, WithVideoStorage(storage, VideoPolicy{}))

	err := service.CleanupCourseVideoUpload(context.Background(), 11, now.Add(-time.Hour))
	if !errors.Is(err, wantErr) {
		t.Fatalf("CleanupCourseVideoUpload() error = %v, want %v", err, wantErr)
	}
	if repository.upload.Status != domain.CourseVideoUploadStatusFailed ||
		repository.video.Status != domain.CourseVideoStatusFailed {
		t.Fatalf("upload = %#v, video = %#v", repository.upload, repository.video)
	}

	storage.deleteErr = nil
	if err := service.CleanupCourseVideoUpload(context.Background(), 11, now.Add(-time.Hour)); err != nil {
		t.Fatalf("recovered CleanupCourseVideoUpload() error = %v", err)
	}
	if repository.upload.Status != domain.CourseVideoUploadStatusCleaned || storage.deleteObjectCalls != 2 {
		t.Fatalf("upload = %#v, delete calls = %d", repository.upload, storage.deleteObjectCalls)
	}
}

func TestCleanupAbortFailureLeavesRecoverableFailedState(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	repository := cleanupRepository(now, domain.CourseVideoUploadStatusPending)
	wantErr := errors.New("abort temporarily unavailable")
	storage := &objectStorageStub{abortMultipartErr: wantErr}
	service := NewService(repository, WithVideoStorage(storage, VideoPolicy{}))

	err := service.CleanupCourseVideoUpload(context.Background(), 11, now.Add(-time.Hour))
	if !errors.Is(err, wantErr) || repository.upload.Status != domain.CourseVideoUploadStatusFailed {
		t.Fatalf("first cleanup error = %v, upload = %#v", err, repository.upload)
	}
	if storage.deleteObjectCalls != 0 {
		t.Fatalf("DeleteObject() calls = %d, want 0", storage.deleteObjectCalls)
	}

	storage.abortMultipartErr = nil
	if err := service.CleanupCourseVideoUpload(context.Background(), 11, now.Add(-time.Hour)); err != nil {
		t.Fatalf("recovered cleanup error = %v", err)
	}
	if repository.upload.Status != domain.CourseVideoUploadStatusCleaned ||
		storage.abortMultipartCalls != 2 || storage.deleteObjectCalls != 1 {
		t.Fatalf("upload = %#v, abort calls = %d, delete calls = %d",
			repository.upload, storage.abortMultipartCalls, storage.deleteObjectCalls)
	}
}

func TestCleanupRecoversAfterDeleteBeforeDatabaseCommit(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	repository := cleanupRepository(now, domain.CourseVideoUploadStatusFailed)
	wantErr := errors.New("database interrupted after object delete")
	repository.saveCleanedErr = wantErr
	repository.saveCleanedFailures = 1
	storage := &objectStorageStub{objectExists: true}
	service := NewService(repository, WithVideoStorage(storage, VideoPolicy{}))

	err := service.CleanupCourseVideoUpload(context.Background(), 11, now.Add(-time.Hour))
	if !errors.Is(err, wantErr) || repository.upload.Status != domain.CourseVideoUploadStatusFailed {
		t.Fatalf("first cleanup error = %v, upload = %#v", err, repository.upload)
	}
	if err := service.CleanupCourseVideoUpload(context.Background(), 11, now.Add(-time.Hour)); err != nil {
		t.Fatalf("recovered cleanup error = %v", err)
	}
	if repository.upload.Status != domain.CourseVideoUploadStatusCleaned || storage.deleteObjectCalls != 2 {
		t.Fatalf("upload = %#v, delete calls = %d", repository.upload, storage.deleteObjectCalls)
	}
}

func TestCompletionAndCleanupRaceCleanupWins(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	repository := cleanupRepository(now, domain.CourseVideoUploadStatusPending)
	storage := &objectStorageStub{
		object:        StoredObject{Size: 1024, ContentType: "video/mp4"},
		uploadedParts: []UploadedPart{{PartNumber: 1, ETag: "etag-1", Size: 1024}},
		objectExists:  true,
		statStarted:   make(chan struct{}),
		statContinue:  make(chan struct{}),
	}
	service := NewService(repository, WithVideoStorage(storage, VideoPolicy{MaxVideoSizeBytes: 2048}))
	completeResult := make(chan error, 1)
	go func() {
		_, err := service.CompleteCourseVideoUpload(context.Background(), 11, nil)
		completeResult <- err
	}()

	<-storage.statStarted
	if err := service.CleanupCourseVideoUpload(context.Background(), 11, now.Add(-time.Hour)); err != nil {
		t.Fatalf("CleanupCourseVideoUpload() error = %v", err)
	}
	storage.statContinue <- struct{}{}
	if err := <-completeResult; !errors.Is(err, domain.ErrCourseVideoUploadNotCompletable) {
		t.Fatalf("CompleteCourseVideoUpload() error = %v, want %v", err, domain.ErrCourseVideoUploadNotCompletable)
	}
	if repository.upload.Status != domain.CourseVideoUploadStatusCleaned ||
		repository.video.Status != domain.CourseVideoStatusFailed {
		t.Fatalf("upload = %#v, video = %#v", repository.upload, repository.video)
	}
}

func TestCleanupSkipsPromotedUpload(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	repository := cleanupRepository(now, domain.CourseVideoUploadStatusPromoted)
	repository.video.Status = domain.CourseVideoStatusReady
	storage := &objectStorageStub{objectExists: true}
	service := NewService(repository, WithVideoStorage(storage, VideoPolicy{}))

	if err := service.CleanupCourseVideoUpload(context.Background(), 11, now.Add(-time.Hour)); err != nil {
		t.Fatalf("CleanupCourseVideoUpload() error = %v", err)
	}
	if storage.deleteObjectCalls != 0 || repository.upload.Status != domain.CourseVideoUploadStatusPromoted {
		t.Fatalf("delete calls = %d, upload = %#v", storage.deleteObjectCalls, repository.upload)
	}
}

func TestCleanupExpiredCourseVideoUploadsUsesBatchQuery(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	repository := cleanupRepository(now, domain.CourseVideoUploadStatusFailed)
	repository.expiredUploads = []domain.CourseVideoUpload{*repository.upload}
	storage := &objectStorageStub{}
	service := NewService(repository, WithVideoStorage(storage, VideoPolicy{}))
	cleanupBefore := now.Add(-time.Hour)

	if err := service.CleanupExpiredCourseVideoUploads(context.Background(), cleanupBefore, 25); err != nil {
		t.Fatalf("CleanupExpiredCourseVideoUploads() error = %v", err)
	}
	if !repository.listCleanupBefore.Equal(cleanupBefore) || repository.listCleanupLimit != 25 {
		t.Fatalf("cleanup before = %v, limit = %d", repository.listCleanupBefore, repository.listCleanupLimit)
	}
	if repository.upload.Status != domain.CourseVideoUploadStatusCleaned {
		t.Fatalf("upload status = %q, want cleaned", repository.upload.Status)
	}
}

func cleanupRepository(now time.Time, uploadStatus domain.CourseVideoUploadStatus) *videoRepositoryStub {
	const objectKey = "course-videos/1/cleanup.mp4"
	return &videoRepositoryStub{
		repositoryStub: repositoryStub{},
		video: &domain.CourseVideo{
			ID: 7, CourseID: 1, Kind: domain.CourseVideoKindPreview,
			ObjectKey: objectKey, Status: domain.CourseVideoStatusUploading,
		},
		upload: &domain.CourseVideoUpload{
			ID: 11, CourseVideoID: 7, ObjectKey: objectKey,
			MultipartUploadID: "multipart-cleanup", FileSize: 1024,
			Status: uploadStatus, ExpiresAt: now.Add(-2 * time.Hour),
		},
	}
}
