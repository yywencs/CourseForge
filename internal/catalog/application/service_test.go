package catalogapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	domain "github.com/yywencs/courseforge/internal/catalog/domain"

	"github.com/google/uuid"
)

type studentEligibilityFilterStub struct {
	classIDs []uint64
	ready    bool
}

func (s studentEligibilityFilterStub) ListEligibleClassIDs(
	context.Context, uint64, uint64,
) ([]uint64, bool, error) {
	return s.classIDs, s.ready, nil
}

func TestListStudentCatalogAppliesReadyEligibilityIndex(t *testing.T) {
	repository := &repositoryStub{}
	service := NewService(repository, WithStudentEligibilityFilter(studentEligibilityFilterStub{
		classIDs: []uint64{11, 12}, ready: true,
	}))
	if _, err := service.ListStudentCatalog(context.Background(), StudentCatalogQuery{
		RoundID: 3, StudentID: 7,
	}); err != nil {
		t.Fatalf("ListStudentCatalog() error = %v", err)
	}
	query := repository.studentCatalogQuery
	if !query.EligibilityFiltered || len(query.EligibleClassIDs) != 2 || query.EligibleClassIDs[0] != 11 {
		t.Fatalf("repository query = %+v", query)
	}
}

func TestCourseVideoUploadCompletesAfterObjectVerification(t *testing.T) {
	repository := &videoRepositoryStub{repositoryStub: repositoryStub{
		course: &domain.Course{ID: 1, CourseCode: "CS-101", CourseName: "程序设计", Credits: 3},
	}}
	storage := &objectStorageStub{
		object:        StoredObject{Size: 1024, ContentType: "video/mp4"},
		uploadedParts: []UploadedPart{{PartNumber: 1, ETag: "etag-1", Size: 1024}},
	}
	service := NewService(repository, WithVideoStorage(storage, VideoPolicy{
		UploadURLTTL: time.Minute, PlaybackURLTTL: time.Minute, MaxVideoSizeBytes: 2048,
	}))
	now := time.Date(2026, 8, 3, 10, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return now }
	service.newObjectKey = func(uint64) (string, error) { return "course-videos/1/test.mp4", nil }

	ticket, err := service.StartCourseVideoUpload(context.Background(), 1, StartVideoUploadInput{
		Kind: domain.CourseVideoKindPreview, Title: "课程预览", FileName: "preview.mp4",
		ContentType: "video/mp4", FileSize: 1024,
	})
	if err != nil {
		t.Fatalf("StartCourseVideoUpload() error = %v", err)
	}
	if ticket.MultipartUploadID != "multipart-1" || ticket.PartSizeBytes != VideoUploadPartSizeBytes ||
		len(ticket.Parts) != 1 || ticket.Parts[0].PartNumber != 1 || repository.video == nil {
		t.Fatalf("ticket = %#v, video = %#v", ticket, repository.video)
	}
	if ticket.UploadID != 11 {
		t.Fatalf("upload ID = %d, want 11", ticket.UploadID)
	}
	if repository.upload == nil || repository.upload.CourseVideoID != ticket.Video.ID ||
		repository.upload.ObjectKey != ticket.Video.ObjectKey ||
		repository.upload.MultipartUploadID != ticket.MultipartUploadID ||
		repository.upload.FileSize != 1024 ||
		repository.upload.Status != domain.CourseVideoUploadStatusPending ||
		!repository.upload.ExpiresAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("upload = %#v", repository.upload)
	}
	if !repository.insertUploadInTransaction {
		t.Fatal("course video upload was not inserted inside the video transaction")
	}
	if !repository.failPendingInTransaction || repository.failedPendingCourseVideoID != ticket.Video.ID {
		t.Fatalf("fail pending in transaction = %v, course video ID = %d",
			repository.failPendingInTransaction, repository.failedPendingCourseVideoID)
	}
	duration := uint64(12_000)
	video, err := service.CompleteCourseVideoUpload(context.Background(), ticket.UploadID, &duration)
	if err != nil {
		t.Fatalf("CompleteCourseVideoUpload() error = %v", err)
	}
	if video.Status != domain.CourseVideoStatusReady || video.DurationMS == nil || *video.DurationMS != duration {
		t.Fatalf("video = %#v", video)
	}
	if repository.upload.Status != domain.CourseVideoUploadStatusPromoted {
		t.Fatalf("upload status = %q, want promoted", repository.upload.Status)
	}
	// 客户端未收到第一次响应而重试时，直接返回已经完成的视频，不再访问 OSS。
	repeated, err := service.CompleteCourseVideoUpload(context.Background(), ticket.UploadID, nil)
	if err != nil {
		t.Fatalf("repeated CompleteCourseVideoUpload() error = %v", err)
	}
	if repeated.ID != video.ID || repeated.Status != domain.CourseVideoStatusReady {
		t.Fatalf("repeated video = %#v", repeated)
	}
	if storage.statObjectCalls != 1 || storage.completeMultipartCalls != 1 {
		t.Fatalf("StatObject() calls = %d, complete calls = %d", storage.statObjectCalls, storage.completeMultipartCalls)
	}
}

func TestNewCourseVideoUploadFailsPreviousPendingAttempt(t *testing.T) {
	repository := &videoRepositoryStub{
		repositoryStub: repositoryStub{
			course: &domain.Course{ID: 1, CourseCode: "CS-101", CourseName: "程序设计", Credits: 3},
		},
		video: &domain.CourseVideo{
			ID: 7, CourseID: 1, Kind: domain.CourseVideoKindPreview,
			Title: "旧预览", ObjectKey: "course-videos/1/old.mp4",
			Status: domain.CourseVideoStatusUploading,
		},
		previousUpload: &domain.CourseVideoUpload{
			ID: 10, CourseVideoID: 7, ObjectKey: "course-videos/1/old.mp4",
			Status: domain.CourseVideoUploadStatusPending,
		},
	}
	service := NewService(repository, WithVideoStorage(&objectStorageStub{}, VideoPolicy{
		UploadURLTTL: time.Minute, PlaybackURLTTL: time.Minute, MaxVideoSizeBytes: 2048,
	}))
	service.newObjectKey = func(uint64) (string, error) { return "course-videos/1/new.mp4", nil }

	ticket, err := service.StartCourseVideoUpload(context.Background(), 1, StartVideoUploadInput{
		Kind: domain.CourseVideoKindPreview, Title: "新预览", FileName: "preview.mp4",
		ContentType: "video/mp4", FileSize: 1024,
	})
	if err != nil {
		t.Fatalf("StartCourseVideoUpload() error = %v", err)
	}
	if repository.previousUpload.Status != domain.CourseVideoUploadStatusFailed {
		t.Fatalf("previous upload status = %q", repository.previousUpload.Status)
	}
	if ticket.UploadID != 11 || repository.upload == nil ||
		repository.upload.Status != domain.CourseVideoUploadStatusPending ||
		repository.upload.ObjectKey != "course-videos/1/new.mp4" {
		t.Fatalf("ticket = %#v, new upload = %#v", ticket, repository.upload)
	}
	if repository.video.ObjectKey != "course-videos/1/new.mp4" {
		t.Fatalf("course video object key = %q", repository.video.ObjectKey)
	}
}

func TestCourseVideoUploadRejectsMissingObject(t *testing.T) {
	repository := &videoRepositoryStub{
		repositoryStub: repositoryStub{},
		video: &domain.CourseVideo{
			ID: 7, CourseID: 1, Kind: domain.CourseVideoKindPreview,
			ObjectKey: "course-videos/1/missing.mp4", Status: domain.CourseVideoStatusUploading,
		},
		upload: &domain.CourseVideoUpload{
			ID: 9, CourseVideoID: 7, ObjectKey: "course-videos/1/missing.mp4",
			MultipartUploadID: "multipart-missing", FileSize: 1024,
			Status: domain.CourseVideoUploadStatusPending,
		},
	}
	storage := &objectStorageStub{
		listPartsErr: ErrMultipartUploadNotFound,
		statErr:      ErrStoredObjectNotFound,
	}
	service := NewService(repository, WithVideoStorage(storage, VideoPolicy{
		UploadURLTTL: time.Minute, PlaybackURLTTL: time.Minute, MaxVideoSizeBytes: 2048,
	}))

	_, err := service.CompleteCourseVideoUpload(context.Background(), 9, nil)
	if !errors.Is(err, domain.ErrVideoUploadIncomplete) {
		t.Fatalf("CompleteCourseVideoUpload() error = %v, want %v", err, domain.ErrVideoUploadIncomplete)
	}
}

func TestCourseVideoUploadRecoversAfterMultipartWasAlreadyCompleted(t *testing.T) {
	repository := &videoRepositoryStub{
		video: &domain.CourseVideo{
			ID: 7, CourseID: 1, Kind: domain.CourseVideoKindPreview,
			ObjectKey: "course-videos/1/completed.mp4", Status: domain.CourseVideoStatusUploading,
		},
		upload: &domain.CourseVideoUpload{
			ID: 9, CourseVideoID: 7, ObjectKey: "course-videos/1/completed.mp4",
			MultipartUploadID: "multipart-completed", FileSize: 1024,
			Status: domain.CourseVideoUploadStatusPending,
		},
	}
	storage := &objectStorageStub{
		listPartsErr: ErrMultipartUploadNotFound,
		object:       StoredObject{Size: 1024, ContentType: "video/mp4"},
	}
	service := NewService(repository, WithVideoStorage(storage, VideoPolicy{MaxVideoSizeBytes: 2048}))

	video, err := service.CompleteCourseVideoUpload(context.Background(), 9, nil)
	if err != nil || video.Status != domain.CourseVideoStatusReady ||
		repository.upload.Status != domain.CourseVideoUploadStatusPromoted {
		t.Fatalf("video = %#v, upload = %#v, error = %v", video, repository.upload, err)
	}
	if storage.completeMultipartCalls != 0 || storage.statObjectCalls != 1 {
		t.Fatalf("complete calls = %d, stat calls = %d", storage.completeMultipartCalls, storage.statObjectCalls)
	}
}

func TestFailedCourseVideoUploadCannotComplete(t *testing.T) {
	repository := &videoRepositoryStub{
		repositoryStub: repositoryStub{},
		video: &domain.CourseVideo{
			ID: 7, CourseID: 1, Kind: domain.CourseVideoKindPreview,
			ObjectKey: "course-videos/1/new.mp4", Status: domain.CourseVideoStatusUploading,
		},
		upload: &domain.CourseVideoUpload{
			ID: 9, CourseVideoID: 7, ObjectKey: "course-videos/1/old.mp4",
			Status: domain.CourseVideoUploadStatusFailed,
		},
	}
	storage := &objectStorageStub{}
	service := NewService(repository, WithVideoStorage(storage, VideoPolicy{
		UploadURLTTL: time.Minute, PlaybackURLTTL: time.Minute, MaxVideoSizeBytes: 2048,
	}))

	_, err := service.CompleteCourseVideoUpload(context.Background(), 9, nil)
	if !errors.Is(err, domain.ErrCourseVideoUploadNotCompletable) {
		t.Fatalf("CompleteCourseVideoUpload() error = %v, want %v", err, domain.ErrCourseVideoUploadNotCompletable)
	}
	if storage.statObjectCalls != 0 || repository.transactions != 0 {
		t.Fatalf("StatObject() calls = %d, transactions = %d", storage.statObjectCalls, repository.transactions)
	}
}

func TestCourseVideoUploadAttemptFailureAbortsMultipartUpload(t *testing.T) {
	wantErr := errors.New("insert upload failed")
	repository := &videoRepositoryStub{
		repositoryStub: repositoryStub{
			course: &domain.Course{ID: 1, CourseCode: "CS-101", CourseName: "程序设计", Credits: 3},
		},
		insertUploadErr: wantErr,
	}
	storage := &objectStorageStub{}
	service := NewService(repository, WithVideoStorage(storage, VideoPolicy{
		UploadURLTTL: time.Minute, PlaybackURLTTL: time.Minute, MaxVideoSizeBytes: 2048,
	}))
	service.newObjectKey = func(uint64) (string, error) { return "course-videos/1/test.mp4", nil }

	_, err := service.StartCourseVideoUpload(context.Background(), 1, StartVideoUploadInput{
		Kind: domain.CourseVideoKindPreview, Title: "课程预览", FileName: "preview.mp4",
		ContentType: "video/mp4", FileSize: 1024,
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("StartCourseVideoUpload() error = %v, want %v", err, wantErr)
	}
	if storage.createMultipartCalls != 1 || storage.presignPartCalls != 1 || storage.abortMultipartCalls != 1 {
		t.Fatalf("create calls = %d, presign calls = %d, abort calls = %d",
			storage.createMultipartCalls, storage.presignPartCalls, storage.abortMultipartCalls)
	}
}

func TestCourseVideoUploadPresignFailureAbortsBeforeTransaction(t *testing.T) {
	repository := &videoRepositoryStub{repositoryStub: repositoryStub{
		course: &domain.Course{ID: 1, CourseCode: "CS-101", CourseName: "程序设计", Credits: 3},
	}}
	wantErr := errors.New("presign unavailable")
	storage := &objectStorageStub{presignPartErr: wantErr}
	service := NewService(repository, WithVideoStorage(storage, VideoPolicy{
		UploadURLTTL: time.Minute, MaxVideoSizeBytes: 2048,
	}))
	service.newObjectKey = func(uint64) (string, error) { return "course-videos/1/test.mp4", nil }

	_, err := service.StartCourseVideoUpload(context.Background(), 1, StartVideoUploadInput{
		Kind: domain.CourseVideoKindPreview, Title: "课程预览", FileName: "preview.mp4",
		ContentType: "video/mp4", FileSize: 1024,
	})
	if !errors.Is(err, ErrVideoStorageUnavailable) || repository.transactions != 0 {
		t.Fatalf("error = %v, transactions = %d", err, repository.transactions)
	}
	if storage.createMultipartCalls != 1 || storage.presignPartCalls != 1 || storage.abortMultipartCalls != 1 {
		t.Fatalf("create calls = %d, presign calls = %d, abort calls = %d",
			storage.createMultipartCalls, storage.presignPartCalls, storage.abortMultipartCalls)
	}
}

func TestCourseVideoObjectKeyFailureStopsBeforeTransaction(t *testing.T) {
	repository := &videoRepositoryStub{repositoryStub: repositoryStub{
		course: &domain.Course{ID: 1, CourseCode: "CS-101", CourseName: "程序设计", Credits: 3},
	}}
	service := NewService(repository, WithVideoStorage(&objectStorageStub{}, VideoPolicy{
		UploadURLTTL: time.Minute, PlaybackURLTTL: time.Minute, MaxVideoSizeBytes: 2048,
	}))
	service.newObjectKey = func(uint64) (string, error) { return "", errors.New("random source unavailable") }

	_, err := service.StartCourseVideoUpload(context.Background(), 1, StartVideoUploadInput{
		Kind: domain.CourseVideoKindPreview, Title: "课程预览", FileName: "preview.mp4",
		ContentType: "video/mp4", FileSize: 1024,
	})
	if !errors.Is(err, ErrVideoStorageUnavailable) {
		t.Fatalf("StartCourseVideoUpload() error = %v, want %v", err, ErrVideoStorageUnavailable)
	}
	if repository.transactions != 0 {
		t.Fatalf("transactions = %d, want 0", repository.transactions)
	}
	storage := service.objectStorage.(*objectStorageStub)
	if storage.createMultipartCalls != 0 {
		t.Fatalf("CreateMultipartUpload() calls = %d, want 0", storage.createMultipartCalls)
	}
}

func TestPresignCourseVideoUploadPartsValidatesMultipartUpload(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	repository := &videoRepositoryStub{upload: &domain.CourseVideoUpload{
		ID: 11, CourseVideoID: 7, ObjectKey: "course-videos/1/test.mp4",
		MultipartUploadID: "multipart-1", FileSize: VideoUploadPartSizeBytes + 1,
		Status: domain.CourseVideoUploadStatusPending, ExpiresAt: now.Add(time.Minute),
	}}
	storage := &objectStorageStub{}
	service := NewService(repository, WithVideoStorage(storage, VideoPolicy{UploadURLTTL: time.Minute}))
	service.now = func() time.Time { return now }

	parts, err := service.PresignCourseVideoUploadParts(context.Background(), 11, PresignVideoUploadPartsInput{
		MultipartUploadID: "multipart-1", PartNumbers: []int{1, 2},
	})
	if err != nil || len(parts) != 2 || parts[1].PartNumber != 2 || storage.presignPartCalls != 2 {
		t.Fatalf("parts = %#v, calls = %d, error = %v", parts, storage.presignPartCalls, err)
	}
	_, err = service.PresignCourseVideoUploadParts(context.Background(), 11, PresignVideoUploadPartsInput{
		MultipartUploadID: "other-upload", PartNumbers: []int{1},
	})
	if !errors.Is(err, domain.ErrCourseVideoUploadNotCompletable) {
		t.Fatalf("mismatched multipart ID error = %v", err)
	}
}

func TestListCourseVideoUploadPartsReturnsOSSState(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	fileSize := 2*VideoUploadPartSizeBytes + 1
	repository := &videoRepositoryStub{upload: &domain.CourseVideoUpload{
		ID: 11, CourseVideoID: 7, ObjectKey: "course-videos/1/test.mp4",
		MultipartUploadID: "multipart-1", FileSize: fileSize,
		Status: domain.CourseVideoUploadStatusPending, ExpiresAt: now.Add(time.Minute),
	}}
	storage := &objectStorageStub{uploadedParts: []UploadedPart{
		{PartNumber: 1, ETag: "etag-1", Size: VideoUploadPartSizeBytes},
		{PartNumber: 3, ETag: "etag-3", Size: 1},
	}}
	service := NewService(repository, WithVideoStorage(storage, VideoPolicy{}))
	service.now = func() time.Time { return now }

	parts, err := service.ListCourseVideoUploadParts(context.Background(), 11)
	if err != nil || len(parts) != 2 || parts[0].PartNumber != 1 || parts[1].PartNumber != 3 {
		t.Fatalf("parts = %#v, error = %v", parts, err)
	}
	if storage.listPartsCalls != 1 {
		t.Fatalf("ListUploadedParts() calls = %d, want 1", storage.listPartsCalls)
	}
}

func TestListCourseVideoUploadPartsRejectsExpiredUpload(t *testing.T) {
	now := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	repository := &videoRepositoryStub{upload: &domain.CourseVideoUpload{
		ID: 11, CourseVideoID: 7, ObjectKey: "course-videos/1/test.mp4",
		MultipartUploadID: "multipart-1", FileSize: 1024,
		Status: domain.CourseVideoUploadStatusPending, ExpiresAt: now.Add(-time.Second),
	}}
	storage := &objectStorageStub{}
	service := NewService(repository, WithVideoStorage(storage, VideoPolicy{}))
	service.now = func() time.Time { return now }

	_, err := service.ListCourseVideoUploadParts(context.Background(), 11)
	if !errors.Is(err, domain.ErrCourseVideoUploadNotCompletable) || storage.listPartsCalls != 0 {
		t.Fatalf("error = %v, ListUploadedParts() calls = %d", err, storage.listPartsCalls)
	}
}

func TestNewVideoObjectKeyUsesUUID(t *testing.T) {
	key, err := newVideoObjectKey(12)
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "course-videos/12/"
	if !strings.HasPrefix(key, prefix) || !strings.HasSuffix(key, ".mp4") {
		t.Fatalf("object key = %q", key)
	}
	idText := strings.TrimSuffix(strings.TrimPrefix(key, prefix), ".mp4")
	id, err := uuid.Parse(idText)
	if err != nil || id.Version() != 4 {
		t.Fatalf("object key UUID = %q, error = %v", idText, err)
	}
}

func TestUpdateCourseStopsBeforeSaveWhenDomainRejectsCoreChange(t *testing.T) {
	repository := &repositoryStub{
		course:      &domain.Course{ID: 1, CourseCode: "CS-101", CourseName: "程序设计", Credits: 3},
		courseUsage: domain.CourseUsage{NonPlannedTeachingClassCount: 1},
	}
	service := NewService(repository)

	_, err := service.UpdateCourse(context.Background(), 1, CourseInput{
		CourseCode: "CS-102", CourseName: "程序设计", Credits: 3,
	})
	if !errors.Is(err, domain.ErrCourseCoreLocked) {
		t.Fatalf("UpdateCourse() error = %v, want %v", err, domain.ErrCourseCoreLocked)
	}
	if repository.saveCourseCalled {
		t.Fatal("SaveCourse() called after domain rejected change")
	}
	if repository.transactions != 1 {
		t.Fatalf("transactions = %d, want 1", repository.transactions)
	}
}

func TestUpdateCoursePersistsDomainChangeInsideTransaction(t *testing.T) {
	repository := &repositoryStub{
		course: &domain.Course{ID: 1, CourseCode: "CS-101", CourseName: "程序设计", Credits: 3},
	}
	service := NewService(repository)

	updated, err := service.UpdateCourse(context.Background(), 1, CourseInput{
		CourseCode: "CS-101", CourseName: "程序设计", Credits: 3,
		Introduction: "新的课程简介",
	})
	if err != nil {
		t.Fatalf("UpdateCourse() error = %v", err)
	}
	if !repository.saveCourseCalled || updated.Introduction != "新的课程简介" {
		t.Fatalf("saved = %v, updated = %#v", repository.saveCourseCalled, updated)
	}
}

func TestUpdateTeachingClassLoadsFactsThenUsesDomainBehavior(t *testing.T) {
	repository := &repositoryStub{
		course: &domain.Course{ID: 1},
		class: &domain.TeachingClass{
			ID: 2, ClassCode: "CS-101-01", TermID: 202601, CourseID: 1,
			Capacity: 30, State: domain.TeachingClassStateOpen,
		},
	}
	service := NewService(repository)

	_, err := service.UpdateTeachingClass(context.Background(), 2, validTeachingClassInput())
	if !errors.Is(err, domain.ErrTeachingClassNotEditable) {
		t.Fatalf("UpdateTeachingClass() error = %v, want %v", err, domain.ErrTeachingClassNotEditable)
	}
	wantEvents := []string{"course-lock", "class-lock", "class-usage"}
	if !equalStrings(repository.events, wantEvents) {
		t.Fatalf("events = %v, want %v", repository.events, wantEvents)
	}
	if repository.saveClassCalled {
		t.Fatal("SaveTeachingClass() called after domain rejected change")
	}
}

func TestCreateTeachingClassValidatesBeforeOpeningTransaction(t *testing.T) {
	repository := &repositoryStub{}
	service := NewService(repository)
	input := validTeachingClassInput()
	input.Capacity = 0

	_, err := service.CreateTeachingClass(context.Background(), input)
	if !errors.Is(err, domain.ErrInvalidTeachingClass) {
		t.Fatalf("CreateTeachingClass() error = %v, want %v", err, domain.ErrInvalidTeachingClass)
	}
	if repository.transactions != 0 {
		t.Fatalf("transactions = %d, want 0", repository.transactions)
	}
}

func validTeachingClassInput() TeachingClassInput {
	return TeachingClassInput{
		ClassCode: "CS-101-01", TermID: 202601, CourseID: 1, Capacity: 30,
		Schedules: []domain.Schedule{{DayOfWeek: 1, StartWeek: 1, EndWeek: 16, StartSection: 1, EndSection: 2}},
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

type repositoryStub struct {
	transactions     int
	transactionDepth int
	events           []string

	course      *domain.Course
	class       *domain.TeachingClass
	courseUsage domain.CourseUsage
	classUsage  domain.TeachingClassUsage

	saveCourseCalled bool
	saveClassCalled  bool

	expiredUploads      []domain.CourseVideoUpload
	listCleanupBefore   time.Time
	listCleanupLimit    int
	studentCatalogQuery StudentCatalogQuery
}

type videoRepositoryStub struct {
	repositoryStub
	video                      *domain.CourseVideo
	upload                     *domain.CourseVideoUpload
	previousUpload             *domain.CourseVideoUpload
	insertUploadInTransaction  bool
	insertUploadErr            error
	failPendingInTransaction   bool
	failedPendingCourseVideoID uint64
	saveCleanedErr             error
	saveCleanedFailures        int
}

func (r *videoRepositoryStub) GetCourseVideo(context.Context, uint64) (*domain.CourseVideo, error) {
	if r.video == nil {
		return nil, domain.ErrNotFound
	}
	copy := *r.video
	return &copy, nil
}

func (r *videoRepositoryStub) GetCourseVideoForUpdate(ctx context.Context, id uint64) (*domain.CourseVideo, error) {
	return r.GetCourseVideo(ctx, id)
}

func (r *videoRepositoryStub) GetCourseVideoByPositionForUpdate(context.Context, uint64, domain.CourseVideoKind, uint32) (*domain.CourseVideo, error) {
	if r.video == nil {
		return nil, domain.ErrNotFound
	}
	copy := *r.video
	return &copy, nil
}

func (r *videoRepositoryStub) InsertCourseVideo(_ context.Context, video *domain.CourseVideo) error {
	video.ID = 7
	copy := *video
	r.video = &copy
	return nil
}

func (r *videoRepositoryStub) SaveCourseVideo(_ context.Context, video *domain.CourseVideo, _ domain.CourseVideoStatus) error {
	copy := *video
	r.video = &copy
	return nil
}

func (r *videoRepositoryStub) ListPendingCourseVideoUploadsForUpdate(_ context.Context, courseVideoID uint64) ([]domain.CourseVideoUpload, error) {
	r.failPendingInTransaction = r.transactionDepth > 0
	r.failedPendingCourseVideoID = courseVideoID
	if r.previousUpload != nil && r.previousUpload.CourseVideoID == courseVideoID &&
		r.previousUpload.Status == domain.CourseVideoUploadStatusPending {
		return []domain.CourseVideoUpload{*r.previousUpload}, nil
	}
	return nil, nil
}

func (r *videoRepositoryStub) InsertCourseVideoUpload(_ context.Context, upload *domain.CourseVideoUpload) error {
	if r.insertUploadErr != nil {
		return r.insertUploadErr
	}
	upload.ID = 11
	copy := *upload
	r.upload = &copy
	r.insertUploadInTransaction = r.transactionDepth > 0
	return nil
}

func (r *videoRepositoryStub) GetCourseVideoUpload(context.Context, uint64) (*domain.CourseVideoUpload, error) {
	if r.upload == nil {
		return nil, domain.ErrNotFound
	}
	copy := *r.upload
	return &copy, nil
}

func (r *videoRepositoryStub) GetCourseVideoUploadForUpdate(ctx context.Context, id uint64) (*domain.CourseVideoUpload, error) {
	return r.GetCourseVideoUpload(ctx, id)
}

func (r *videoRepositoryStub) SaveCourseVideoUpload(_ context.Context, upload *domain.CourseVideoUpload, _ domain.CourseVideoUploadStatus) error {
	if upload.Status == domain.CourseVideoUploadStatusCleaned && r.saveCleanedFailures > 0 {
		r.saveCleanedFailures--
		return r.saveCleanedErr
	}
	copy := *upload
	if r.previousUpload != nil && upload.ID == r.previousUpload.ID {
		r.previousUpload = &copy
		return nil
	}
	r.upload = &copy
	return nil
}

type objectStorageStub struct {
	object                 StoredObject
	statErr                error
	createMultipartCalls   int
	presignPartCalls       int
	completeMultipartCalls int
	abortMultipartCalls    int
	listPartsCalls         int
	uploadedParts          []UploadedPart
	createMultipartErr     error
	presignPartErr         error
	listPartsErr           error
	completeMultipartErr   error
	abortMultipartErr      error
	statObjectCalls        int
	deleteObjectCalls      int
	deletedObjectKeys      []string
	deleteErr              error
	objectExists           bool
	statStarted            chan struct{}
	statContinue           chan struct{}
}

func (s *objectStorageStub) CreateMultipartUpload(context.Context, string, string) (string, error) {
	s.createMultipartCalls++
	if s.createMultipartErr != nil {
		return "", s.createMultipartErr
	}
	return "multipart-1", nil
}

func (s *objectStorageStub) PresignUploadPart(_ context.Context, _ string, _ string, partNumber int, _ time.Duration) (string, error) {
	s.presignPartCalls++
	if s.presignPartErr != nil {
		return "", s.presignPartErr
	}
	return fmt.Sprintf("https://objects.example/upload?partNumber=%d", partNumber), nil
}

func (s *objectStorageStub) ListUploadedParts(context.Context, string, string) ([]UploadedPart, error) {
	s.listPartsCalls++
	return s.uploadedParts, s.listPartsErr
}

func (s *objectStorageStub) CompleteMultipartUpload(context.Context, string, string, []UploadedPart) error {
	s.completeMultipartCalls++
	return s.completeMultipartErr
}

func (s *objectStorageStub) AbortMultipartUpload(context.Context, string, string) error {
	s.abortMultipartCalls++
	return s.abortMultipartErr
}

func (s *objectStorageStub) StatObject(context.Context, string) (StoredObject, error) {
	s.statObjectCalls++
	if s.statStarted != nil {
		s.statStarted <- struct{}{}
		<-s.statContinue
	}
	return s.object, s.statErr
}

func (s *objectStorageStub) PresignPlayback(context.Context, string, time.Duration) (string, error) {
	return "https://objects.example/play", nil
}

func (s *objectStorageStub) DeleteObject(_ context.Context, objectKey string) error {
	s.deleteObjectCalls++
	s.deletedObjectKeys = append(s.deletedObjectKeys, objectKey)
	if s.deleteErr != nil {
		return s.deleteErr
	}
	s.objectExists = false
	return nil
}

func (r *repositoryStub) WithinTransaction(ctx context.Context, fn func(context.Context) error) error {
	r.transactions++
	r.transactionDepth++
	defer func() { r.transactionDepth-- }()
	return fn(ctx)
}

func (r *repositoryStub) ListCourses(context.Context, string) ([]domain.Course, error) {
	return nil, nil
}
func (r *repositoryStub) GetCourse(context.Context, uint64) (*domain.Course, error) {
	return r.course, nil
}
func (r *repositoryStub) GetCourseForUpdate(context.Context, uint64) (*domain.Course, error) {
	r.events = append(r.events, "course-lock")
	if r.course == nil {
		return nil, domain.ErrNotFound
	}
	return r.course, nil
}
func (r *repositoryStub) InsertCourse(context.Context, *domain.Course) error { return nil }
func (r *repositoryStub) SaveCourse(context.Context, *domain.Course) error {
	r.saveCourseCalled = true
	return nil
}
func (r *repositoryStub) RemoveCourse(context.Context, uint64) error { return nil }
func (r *repositoryStub) InspectCourseUsage(context.Context, uint64) (domain.CourseUsage, error) {
	r.events = append(r.events, "course-usage")
	return r.courseUsage, nil
}
func (r *repositoryStub) ListCourseVideos(context.Context, uint64) ([]domain.CourseVideo, error) {
	return nil, nil
}
func (r *repositoryStub) GetCourseVideo(context.Context, uint64) (*domain.CourseVideo, error) {
	return nil, domain.ErrNotFound
}
func (r *repositoryStub) GetCourseVideoForUpdate(context.Context, uint64) (*domain.CourseVideo, error) {
	return nil, domain.ErrNotFound
}
func (r *repositoryStub) GetCourseVideoByPositionForUpdate(context.Context, uint64, domain.CourseVideoKind, uint32) (*domain.CourseVideo, error) {
	return nil, domain.ErrNotFound
}
func (r *repositoryStub) InsertCourseVideo(context.Context, *domain.CourseVideo) error { return nil }
func (r *repositoryStub) SaveCourseVideo(context.Context, *domain.CourseVideo, domain.CourseVideoStatus) error {
	return nil
}
func (r *repositoryStub) ListPendingCourseVideoUploadsForUpdate(context.Context, uint64) ([]domain.CourseVideoUpload, error) {
	return nil, nil
}
func (r *repositoryStub) InsertCourseVideoUpload(context.Context, *domain.CourseVideoUpload) error {
	return nil
}
func (r *repositoryStub) GetCourseVideoUpload(context.Context, uint64) (*domain.CourseVideoUpload, error) {
	return nil, domain.ErrNotFound
}
func (r *repositoryStub) GetCourseVideoUploadForUpdate(context.Context, uint64) (*domain.CourseVideoUpload, error) {
	return nil, domain.ErrNotFound
}
func (r *repositoryStub) SaveCourseVideoUpload(context.Context, *domain.CourseVideoUpload, domain.CourseVideoUploadStatus) error {
	return nil
}
func (r *repositoryStub) ListExpiredCourseVideoUploads(_ context.Context, cleanupBefore time.Time, limit int) ([]domain.CourseVideoUpload, error) {
	r.listCleanupBefore = cleanupBefore
	r.listCleanupLimit = limit
	result := make([]domain.CourseVideoUpload, len(r.expiredUploads))
	copy(result, r.expiredUploads)
	return result, nil
}

func (r *repositoryStub) ListTeachingClasses(context.Context, uint64, string) ([]TeachingClassView, error) {
	return nil, nil
}
func (r *repositoryStub) ListStudentCatalog(_ context.Context, query StudentCatalogQuery) ([]TeachingClassView, error) {
	r.studentCatalogQuery = query
	return nil, nil
}
func (r *repositoryStub) GetTeachingClass(context.Context, uint64) (*TeachingClassView, error) {
	if r.class == nil {
		return nil, domain.ErrNotFound
	}
	return &TeachingClassView{
		ID: r.class.ID, ClassCode: r.class.ClassCode, TermID: r.class.TermID,
		CourseID: r.class.CourseID, Capacity: r.class.Capacity, State: r.class.State,
	}, nil
}
func (r *repositoryStub) GetTeachingClassForUpdate(context.Context, uint64) (*domain.TeachingClass, error) {
	r.events = append(r.events, "class-lock")
	if r.class == nil {
		return nil, domain.ErrNotFound
	}
	return r.class, nil
}
func (r *repositoryStub) InsertTeachingClass(context.Context, *domain.TeachingClass) error {
	return nil
}
func (r *repositoryStub) SaveTeachingClass(context.Context, *domain.TeachingClass) error {
	r.saveClassCalled = true
	return nil
}
func (r *repositoryStub) RemoveTeachingClass(context.Context, uint64) error { return nil }
func (r *repositoryStub) InspectTeachingClassUsage(context.Context, uint64) (domain.TeachingClassUsage, error) {
	r.events = append(r.events, "class-usage")
	return r.classUsage, nil
}
