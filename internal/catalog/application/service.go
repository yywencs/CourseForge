package catalogapp

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "prizeforge/internal/catalog/domain"
)

var (
	ErrVideoStorageUnavailable = errors.New("视频存储服务暂时不可用")
	ErrVideoObjectInvalid      = errors.New("上传的视频文件校验失败")
)

type VideoPolicy struct {
	UploadURLTTL      time.Duration
	PlaybackURLTTL    time.Duration
	MaxVideoSizeBytes int64
}

type ServiceOption func(*Service)

func WithVideoStorage(storage ObjectStorage, policy VideoPolicy) ServiceOption {
	return func(service *Service) {
		service.objectStorage = storage
		service.videoPolicy = policy
	}
}

type CourseInput struct {
	CourseCode   string
	CourseName   string
	Credits      float64
	Introduction string
	Tags         []string
}

func (i CourseInput) details() domain.CourseDetails {
	return domain.CourseDetails{
		CourseCode: i.CourseCode, CourseName: i.CourseName, Credits: i.Credits,
		Introduction: i.Introduction, Tags: i.Tags,
	}
}

type TeachingClassInput struct {
	ClassCode        string
	TermID           uint64
	CourseID         uint64
	TeacherName      string
	Location         string
	Capacity         uint32
	MinimumGradeYear *uint16
	MaximumGradeYear *uint16
	Schedules        []domain.Schedule
}

func (i TeachingClassInput) plan() domain.TeachingClassPlan {
	return domain.TeachingClassPlan{
		ClassCode: i.ClassCode, TermID: i.TermID, CourseID: i.CourseID,
		TeacherName: i.TeacherName, Location: i.Location, Capacity: i.Capacity,
		MinimumGradeYear: i.MinimumGradeYear, MaximumGradeYear: i.MaximumGradeYear,
		Schedules: i.Schedules,
	}
}

type StudentCatalogQuery struct {
	RoundID uint64
	Keyword string
}

type Service struct {
	repository    Repository
	objectStorage ObjectStorage
	videoPolicy   VideoPolicy
	now           func() time.Time
	newObjectKey  func(uint64) (string, error)
}

func NewService(repository Repository, options ...ServiceOption) *Service {
	service := &Service{
		repository: repository,
		videoPolicy: VideoPolicy{
			UploadURLTTL: 15 * time.Minute, PlaybackURLTTL: 15 * time.Minute,
			MaxVideoSizeBytes: 500 * 1024 * 1024,
		},
		now:          time.Now,
		newObjectKey: newVideoObjectKey,
	}
	for _, option := range options {
		option(service)
	}
	return service
}

type StartVideoUploadInput struct {
	Kind        domain.CourseVideoKind
	Title       string
	FileName    string
	ContentType string
	FileSize    int64
	SortOrder   uint32
}

type VideoUploadTicket struct {
	Video     domain.CourseVideo
	UploadURL string
	ExpiresAt time.Time
}

type VideoPlaybackTicket struct {
	PlayURL   string
	ExpiresAt time.Time
}

// StartCourseVideoUpload 创建或重置课程视频记录，并返回供前端直传对象存储的临时 URL。
// 该方法不接收视频二进制；前端 PUT 上传成功后，还需要调用 CompleteCourseVideoUpload
// 让后端核验对象并将视频状态从 uploading 更新为 ready。
func (s *Service) StartCourseVideoUpload(ctx context.Context, courseID uint64, input StartVideoUploadInput) (*VideoUploadTicket, error) {
	// 对象存储关闭时，课程目录仍可使用，但不能申请视频上传。
	if s.objectStorage == nil {
		return nil, ErrVideoStorageUnavailable
	}
	// 这里只对客户端声明的元数据做前置校验；上传完成后还会以对象存储中的
	// 实际文件大小和 Content-Type 为准再次校验。
	if input.FileSize <= 0 || input.FileSize > s.videoPolicy.MaxVideoSizeBytes ||
		!strings.EqualFold(strings.TrimSpace(input.ContentType), "video/mp4") ||
		!strings.HasSuffix(strings.ToLower(strings.TrimSpace(input.FileName)), ".mp4") {
		return nil, domain.ErrInvalidCourseVideo
	}
	var video *domain.CourseVideo
	// 同一课程、视频类型和排序号代表同一个视频位。行锁可避免并发申请上传时
	// 为同一位置创建多条记录；重新上传则复用原对象键，数据库无需保存临时 URL。
	if err := s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		// 锁定课程行既能确认课程存在，也能把同一课程的并发上传申请串行化。
		if _, err := s.repository.GetCourseForUpdate(txCtx, courseID); err != nil {
			return err
		}
		// 数据库唯一位置由 course_id、video_kind 和 sort_order 共同确定。
		existing, err := s.repository.GetCourseVideoByPositionForUpdate(txCtx, courseID, input.Kind, input.SortOrder)
		if err == nil {
			// 已有未完成或失败的视频时复用记录和对象键，避免产生无主对象。
			expectedStatus := existing.Status
			if err := existing.RestartUpload(input.Title); err != nil {
				return err
			}
			if err := s.repository.SaveCourseVideo(txCtx, existing, expectedStatus); err != nil {
				return err
			}
			video = existing
			return nil
		}
		if !errors.Is(err, domain.ErrNotFound) {
			return err
		}
		// 新视频使用不可预测的随机对象键，不保存用户原始文件名。
		objectKey, err := s.newObjectKey(courseID)
		if err != nil {
			return fmt.Errorf("%w: generate object key", ErrVideoStorageUnavailable)
		}
		video, err = domain.NewCourseVideo(courseID, input.Kind, input.Title, objectKey, input.SortOrder)
		if err != nil {
			return err
		}
		return s.repository.InsertCourseVideo(txCtx, video)
	}); err != nil {
		return nil, err
	}
	// 预签名只依赖已提交的视频记录，放在事务外可避免对象存储调用延长数据库事务和行锁时间。
	uploadURL, err := s.objectStorage.PresignUpload(ctx, video.ObjectKey, s.videoPolicy.UploadURLTTL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVideoStorageUnavailable, err)
	}
	return &VideoUploadTicket{
		Video: *video, UploadURL: uploadURL, ExpiresAt: s.now().Add(s.videoPolicy.UploadURLTTL),
	}, nil
}

// CompleteCourseVideoUpload 核验已上传的对象，并将视频状态从 uploading 更新为 ready。
func (s *Service) CompleteCourseVideoUpload(ctx context.Context, videoID uint64, durationMS *uint64) (*domain.CourseVideo, error) {
	if s.objectStorage == nil {
		return nil, ErrVideoStorageUnavailable
	}
	video, err := s.repository.GetCourseVideo(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if video.Status != domain.CourseVideoStatusUploading {
		return nil, domain.ErrCourseVideoNotUploadable
	}
	// 以对象存储中的实际元数据为准，不直接信任客户端的完成声明。
	object, err := s.objectStorage.StatObject(ctx, video.ObjectKey)
	if errors.Is(err, ErrStoredObjectNotFound) {
		return nil, domain.ErrVideoUploadIncomplete
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVideoStorageUnavailable, err)
	}
	if object.Size <= 0 || object.Size > s.videoPolicy.MaxVideoSizeBytes ||
		!strings.EqualFold(strings.TrimSpace(object.ContentType), "video/mp4") {
		return nil, ErrVideoObjectInvalid
	}
	// 锁定记录并按旧状态更新，防止重复完成或并发覆盖。
	err = s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		current, err := s.repository.GetCourseVideoForUpdate(txCtx, videoID)
		if err != nil {
			return err
		}
		expectedStatus := current.Status
		if err := current.CompleteUpload(durationMS); err != nil {
			return err
		}
		if err := s.repository.SaveCourseVideo(txCtx, current, expectedStatus); err != nil {
			return err
		}
		video = current
		return nil
	})
	return video, err
}

func (s *Service) ListCourseVideos(ctx context.Context, courseID uint64) ([]domain.CourseVideo, error) {
	if _, err := s.repository.GetCourse(ctx, courseID); err != nil {
		return nil, err
	}
	return s.repository.ListCourseVideos(ctx, courseID)
}

func (s *Service) GetPreviewPlayback(ctx context.Context, videoID uint64) (*VideoPlaybackTicket, error) {
	if s.objectStorage == nil {
		return nil, ErrVideoStorageUnavailable
	}
	video, err := s.repository.GetCourseVideo(ctx, videoID)
	if err != nil {
		return nil, err
	}
	if err := video.EnsurePreviewPlayable(); err != nil {
		return nil, err
	}
	// 每次播放临时签发 URL；数据库只保存稳定对象键，便于以后切换 MinIO 或云 OSS。
	playURL, err := s.objectStorage.PresignPlayback(ctx, video.ObjectKey, s.videoPolicy.PlaybackURLTTL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVideoStorageUnavailable, err)
	}
	return &VideoPlaybackTicket{PlayURL: playURL, ExpiresAt: s.now().Add(s.videoPolicy.PlaybackURLTTL)}, nil
}

func newVideoObjectKey(courseID uint64) (string, error) {
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	// 随机键既避免同名文件互相覆盖，也不向对象存储暴露用户原始文件名。
	return fmt.Sprintf("course-videos/%d/%s.mp4", courseID, hex.EncodeToString(random)), nil
}

// 查询侧没有业务状态变更，可以直接使用只读端口。
func (s *Service) ListCourses(ctx context.Context, keyword string) ([]domain.Course, error) {
	return s.repository.ListCourses(ctx, keyword)
}

func (s *Service) GetCourse(ctx context.Context, id uint64) (*domain.Course, error) {
	return s.repository.GetCourse(ctx, id)
}

func (s *Service) CreateCourse(ctx context.Context, input CourseInput) (*domain.Course, error) {
	course, err := domain.NewCourse(input.details())
	if err != nil {
		return nil, err
	}
	if err := s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		return s.repository.InsertCourse(txCtx, course)
	}); err != nil {
		return nil, err
	}
	return course, nil
}

func (s *Service) UpdateCourse(ctx context.Context, id uint64, input CourseInput) (*domain.Course, error) {
	var course *domain.Course
	err := s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		current, err := s.repository.GetCourseForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		usage, err := s.repository.InspectCourseUsage(txCtx, id)
		if err != nil {
			return err
		}
		if err := current.Change(input.details(), usage); err != nil {
			return err
		}
		if err := s.repository.SaveCourse(txCtx, current); err != nil {
			return err
		}
		course = current
		return nil
	})
	return course, err
}

func (s *Service) DeleteCourse(ctx context.Context, id uint64) error {
	return s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		course, err := s.repository.GetCourseForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		usage, err := s.repository.InspectCourseUsage(txCtx, id)
		if err != nil {
			return err
		}
		if err := course.EnsureDeletable(usage); err != nil {
			return err
		}
		return s.repository.RemoveCourse(txCtx, id)
	})
}

func (s *Service) ListTeachingClasses(ctx context.Context, termID uint64, keyword string) ([]TeachingClassView, error) {
	return s.repository.ListTeachingClasses(ctx, termID, keyword)
}

func (s *Service) ListStudentCatalog(ctx context.Context, query StudentCatalogQuery) ([]TeachingClassView, error) {
	return s.repository.ListStudentCatalog(ctx, query)
}

func (s *Service) GetTeachingClass(ctx context.Context, id uint64) (*TeachingClassView, error) {
	return s.repository.GetTeachingClass(ctx, id)
}

func (s *Service) CreateTeachingClass(ctx context.Context, input TeachingClassInput) (*TeachingClassView, error) {
	class, err := domain.NewTeachingClass(input.plan())
	if err != nil {
		return nil, err
	}
	var created *TeachingClassView
	err = s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		// 锁住被引用课程，使“删除课程”和“创建教学班”不能并发穿透逻辑外键。
		if _, err := s.repository.GetCourseForUpdate(txCtx, class.CourseID); err != nil {
			return err
		}
		if err := s.repository.InsertTeachingClass(txCtx, class); err != nil {
			return err
		}
		// 在提交前读取完整投影；读取失败则整体回滚，避免“已创建但接口报错”。
		created, err = s.repository.GetTeachingClass(txCtx, class.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *Service) UpdateTeachingClass(ctx context.Context, id uint64, input TeachingClassInput) (*TeachingClassView, error) {
	var class *TeachingClassView
	err := s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		// 先锁目标课程，再锁教学班，保持逻辑外键操作的锁顺序一致。
		if _, err := s.repository.GetCourseForUpdate(txCtx, input.CourseID); err != nil {
			return err
		}
		current, err := s.repository.GetTeachingClassForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		usage, err := s.repository.InspectTeachingClassUsage(txCtx, id)
		if err != nil {
			return err
		}
		if err := current.ChangePlan(input.plan(), usage); err != nil {
			return err
		}
		if err := s.repository.SaveTeachingClass(txCtx, current); err != nil {
			return err
		}
		// 同一事务内刷新课程投影字段，读取失败时不提交半完成操作。
		class, err = s.repository.GetTeachingClass(txCtx, current.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return class, nil
}

func (s *Service) DeleteTeachingClass(ctx context.Context, id uint64) error {
	return s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		class, err := s.repository.GetTeachingClassForUpdate(txCtx, id)
		if err != nil {
			return err
		}
		usage, err := s.repository.InspectTeachingClassUsage(txCtx, id)
		if err != nil {
			return err
		}
		if err := class.EnsureDeletable(usage); err != nil {
			return err
		}
		return s.repository.RemoveTeachingClass(txCtx, id)
	})
}
