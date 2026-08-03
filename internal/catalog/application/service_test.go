package catalogapp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	domain "prizeforge/internal/catalog/domain"

	"github.com/google/uuid"
)

func TestCourseVideoUploadCompletesAfterObjectVerification(t *testing.T) {
	repository := &videoRepositoryStub{repositoryStub: repositoryStub{
		course: &domain.Course{ID: 1, CourseCode: "CS-101", CourseName: "程序设计", Credits: 3},
	}}
	storage := &objectStorageStub{object: StoredObject{Size: 1024, ContentType: "video/mp4"}}
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
	if ticket.UploadURL != "https://objects.example/upload" || repository.video == nil {
		t.Fatalf("ticket = %#v, video = %#v", ticket, repository.video)
	}
	if repository.upload == nil || repository.upload.CourseVideoID != ticket.Video.ID ||
		repository.upload.ObjectKey != ticket.Video.ObjectKey ||
		repository.upload.Status != domain.CourseVideoUploadStatusPending ||
		!repository.upload.ExpiresAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("upload = %#v", repository.upload)
	}
	if !repository.insertUploadInTransaction {
		t.Fatal("course video upload was not inserted inside the video transaction")
	}
	duration := uint64(12_000)
	video, err := service.CompleteCourseVideoUpload(context.Background(), ticket.Video.ID, &duration)
	if err != nil {
		t.Fatalf("CompleteCourseVideoUpload() error = %v", err)
	}
	if video.Status != domain.CourseVideoStatusReady || video.DurationMS == nil || *video.DurationMS != duration {
		t.Fatalf("video = %#v", video)
	}
}

func TestCourseVideoUploadRejectsMissingObject(t *testing.T) {
	repository := &videoRepositoryStub{repositoryStub: repositoryStub{}, video: &domain.CourseVideo{
		ID: 7, CourseID: 1, Kind: domain.CourseVideoKindPreview,
		ObjectKey: "course-videos/1/missing.mp4", Status: domain.CourseVideoStatusUploading,
	}}
	storage := &objectStorageStub{statErr: ErrStoredObjectNotFound}
	service := NewService(repository, WithVideoStorage(storage, VideoPolicy{
		UploadURLTTL: time.Minute, PlaybackURLTTL: time.Minute, MaxVideoSizeBytes: 2048,
	}))

	_, err := service.CompleteCourseVideoUpload(context.Background(), 7, nil)
	if !errors.Is(err, domain.ErrVideoUploadIncomplete) {
		t.Fatalf("CompleteCourseVideoUpload() error = %v, want %v", err, domain.ErrVideoUploadIncomplete)
	}
}

func TestCourseVideoUploadAttemptFailureStopsBeforePresigning(t *testing.T) {
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
	if storage.presignUploadCalls != 0 {
		t.Fatalf("PresignUpload() calls = %d, want 0", storage.presignUploadCalls)
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
}

type videoRepositoryStub struct {
	repositoryStub
	video                     *domain.CourseVideo
	upload                    *domain.CourseVideoUpload
	insertUploadInTransaction bool
	insertUploadErr           error
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

type objectStorageStub struct {
	object             StoredObject
	statErr            error
	presignUploadCalls int
}

func (s *objectStorageStub) PresignUpload(context.Context, string, time.Duration) (string, error) {
	s.presignUploadCalls++
	return "https://objects.example/upload", nil
}

func (s *objectStorageStub) StatObject(context.Context, string) (StoredObject, error) {
	return s.object, s.statErr
}

func (s *objectStorageStub) PresignPlayback(context.Context, string, time.Duration) (string, error) {
	return "https://objects.example/play", nil
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
func (r *repositoryStub) InsertCourseVideoUpload(context.Context, *domain.CourseVideoUpload) error {
	return nil
}

func (r *repositoryStub) ListTeachingClasses(context.Context, uint64, string) ([]TeachingClassView, error) {
	return nil, nil
}
func (r *repositoryStub) ListStudentCatalog(context.Context, StudentCatalogQuery) ([]TeachingClassView, error) {
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
