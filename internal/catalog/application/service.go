package catalogapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	domain "github.com/yywencs/courseforge/internal/catalog/domain"

	"github.com/google/uuid"
)

var (
	ErrVideoStorageUnavailable = errors.New("视频存储服务暂时不可用")
	ErrVideoObjectInvalid      = domain.ErrVideoObjectInvalid
)

const (
	VideoUploadPartSizeBytes = int64(8 * 1024 * 1024)
	maxPresignedPartsPerCall = 100
	maxMultipartPartNumber   = 10_000
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

// StudentEligibilityFilter 隔离 Catalog 与选课模块的具体缓存实现。
type StudentEligibilityFilter interface {
	ListEligibleClassIDs(context.Context, uint64, uint64) ([]uint64, bool, error)
}

func WithStudentEligibilityFilter(filter StudentEligibilityFilter) ServiceOption {
	return func(service *Service) { service.eligibilityFilter = filter }
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
	RoundID             uint64
	StudentID           uint64
	Keyword             string
	EligibleClassIDs    []uint64
	EligibilityFiltered bool
}

type Service struct {
	repository        Repository
	objectStorage     ObjectStorage
	videoPolicy       VideoPolicy
	now               func() time.Time
	newObjectKey      func(uint64) (string, error)
	eligibilityFilter StudentEligibilityFilter
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
	Video             domain.CourseVideo
	UploadID          uint64
	MultipartUploadID string
	PartSizeBytes     int64
	Parts             []VideoUploadPartTicket
	ExpiresAt         time.Time
}

type PresignVideoUploadPartsInput struct {
	MultipartUploadID string
	PartNumbers       []int
}

type VideoUploadPartTicket struct {
	PartNumber int
	UploadURL  string
}

type VideoPlaybackTicket struct {
	PlayURL   string
	ExpiresAt time.Time
}

// StartCourseVideoUpload 创建 OSS Multipart Upload 和对应的业务上传记录。
// Multipart 会话先于数据库事务创建；事务失败时立即补偿终止该会话。
func (s *Service) StartCourseVideoUpload(ctx context.Context, courseID uint64, input StartVideoUploadInput) (*VideoUploadTicket, error) {
	// 对象存储关闭时，课程目录仍可使用，但不能申请视频上传。
	if s.objectStorage == nil {
		return nil, ErrVideoStorageUnavailable
	}
	// 这里只对客户端声明的元数据做前置校验；上传完成后还会以对象存储中的
	// 实际文件大小和 Content-Type 为准再次校验。
	if err := s.domainVideoPolicy().ValidateDeclared(domain.VideoFileMetadata{
		FileName: input.FileName, ContentType: input.ContentType, Size: input.FileSize,
	}); err != nil {
		return nil, err
	}
	// 对象键不依赖数据库生成值，先生成可在失败时避免开启事务和持有课程行锁。
	objectKey, err := s.newObjectKey(courseID)
	if err != nil {
		return nil, fmt.Errorf("%w: generate object key", ErrVideoStorageUnavailable)
	}
	multipartUploadID, err := s.objectStorage.CreateMultipartUpload(ctx, objectKey, "video/mp4")
	if err != nil {
		return nil, fmt.Errorf("%w: create multipart upload: %v", ErrVideoStorageUnavailable, err)
	}
	partNumbers, err := allVideoPartNumbers(input.FileSize)
	if err != nil {
		s.abortMultipartUploadBestEffort(ctx, objectKey, multipartUploadID)
		return nil, err
	}
	parts, err := s.presignUploadParts(ctx, objectKey, multipartUploadID, partNumbers)
	if err != nil {
		s.abortMultipartUploadBestEffort(ctx, objectKey, multipartUploadID)
		return nil, err
	}
	var video *domain.CourseVideo
	var upload *domain.CourseVideoUpload
	expiresAt := s.now().Add(s.videoPolicy.UploadURLTTL)
	// 同一课程、视频类型和排序号代表同一个视频位。行锁可避免并发申请上传时
	// 为同一位置创建多条记录；每次尝试使用新对象键，避免旧签名 URL 覆盖新对象。
	if err := s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		// 锁定课程行既能确认课程存在，也能把同一课程的并发上传申请串行化。
		if _, err := s.repository.GetCourseForUpdate(txCtx, courseID); err != nil {
			return err
		}
		// 数据库唯一位置由 course_id、video_kind 和 sort_order 共同确定。
		existing, err := s.repository.GetCourseVideoByPositionForUpdate(txCtx, courseID, input.Kind, input.SortOrder)
		if err == nil {
			// 已有未完成或失败的视频时复用逻辑记录，但物理对象键属于本次新尝试。
			expectedStatus := existing.Status
			if err := existing.RestartUpload(input.Title, objectKey); err != nil {
				return err
			}
			if err := s.repository.SaveCourseVideo(txCtx, existing, expectedStatus); err != nil {
				return err
			}
			video = existing
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		} else {
			// 新视频使用不可预测的随机对象键，不保存用户原始文件名。
			video, err = domain.NewCourseVideo(courseID, input.Kind, input.Title, objectKey, input.SortOrder)
			if err != nil {
				return err
			}
			if err := s.repository.InsertCourseVideo(txCtx, video); err != nil {
				return err
			}
		}
		// 新尝试取代该逻辑视频尚未完成的旧尝试；与新任务插入一起提交，
		// 避免出现旧任务已失败但新任务没有创建成功的中间状态。
		previousUploads, err := s.repository.ListPendingCourseVideoUploadsForUpdate(txCtx, video.ID)
		if err != nil {
			return err
		}
		for index := range previousUploads {
			expectedStatus := previousUploads[index].Status
			if err := previousUploads[index].Fail(); err != nil {
				return err
			}
			if err := s.repository.SaveCourseVideoUpload(txCtx, &previousUploads[index], expectedStatus); err != nil {
				return err
			}
		}
		upload, err = domain.NewCourseVideoUpload(
			video.ID, objectKey, multipartUploadID, input.FileSize, expiresAt,
		)
		if err != nil {
			return err
		}
		return s.repository.InsertCourseVideoUpload(txCtx, upload)
	}); err != nil {
		s.abortMultipartUploadBestEffort(ctx, objectKey, multipartUploadID)
		return nil, err
	}
	return &VideoUploadTicket{
		Video: *video, UploadID: upload.ID, MultipartUploadID: multipartUploadID,
		PartSizeBytes: VideoUploadPartSizeBytes, Parts: parts, ExpiresAt: expiresAt,
	}, nil
}

// PresignCourseVideoUploadParts 为当前 pending 上传批量签发指定分片的 PUT URL。
func (s *Service) PresignCourseVideoUploadParts(
	ctx context.Context,
	uploadID uint64,
	input PresignVideoUploadPartsInput,
) ([]VideoUploadPartTicket, error) {
	if s.objectStorage == nil {
		return nil, ErrVideoStorageUnavailable
	}
	upload, err := s.repository.GetCourseVideoUpload(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	if err := upload.EnsureActiveSession(input.MultipartUploadID, s.now()); err != nil {
		return nil, err
	}
	if len(input.PartNumbers) == 0 || len(input.PartNumbers) > maxPresignedPartsPerCall {
		return nil, domain.ErrCourseVideoUploadNotCompletable
	}
	allPartNumbers, err := allVideoPartNumbers(upload.FileSize)
	if err != nil {
		return nil, err
	}
	maxPartNumber := len(allPartNumbers)
	seen := make(map[int]struct{}, len(input.PartNumbers))
	for _, partNumber := range input.PartNumbers {
		if partNumber <= 0 || partNumber > maxPartNumber {
			return nil, domain.ErrInvalidCourseVideo
		}
		if _, exists := seen[partNumber]; exists {
			return nil, domain.ErrInvalidCourseVideo
		}
		seen[partNumber] = struct{}{}
	}
	return s.presignUploadParts(ctx, upload.ObjectKey, upload.MultipartUploadID, input.PartNumbers)
}

// ListCourseVideoUploadParts 以 OSS Multipart 会话为准返回已经持久化成功的分片。
func (s *Service) ListCourseVideoUploadParts(
	ctx context.Context,
	uploadID uint64,
) ([]UploadedPart, error) {
	if s.objectStorage == nil {
		return nil, ErrVideoStorageUnavailable
	}
	upload, err := s.repository.GetCourseVideoUpload(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	if err := upload.EnsureActive(s.now()); err != nil {
		return nil, err
	}
	parts, err := s.objectStorage.ListUploadedParts(
		ctx, upload.ObjectKey, upload.MultipartUploadID,
	)
	if errors.Is(err, ErrMultipartUploadNotFound) {
		return nil, domain.ErrCourseVideoUploadNotCompletable
	}
	if err != nil {
		return nil, fmt.Errorf("%w: list multipart upload parts: %v", ErrVideoStorageUnavailable, err)
	}
	if !validUploadedVideoPartSubset(parts, upload.FileSize) {
		return nil, ErrVideoObjectInvalid
	}
	return parts, nil
}

func (s *Service) presignUploadParts(
	ctx context.Context,
	objectKey string,
	multipartUploadID string,
	partNumbers []int,
) ([]VideoUploadPartTicket, error) {
	tickets := make([]VideoUploadPartTicket, 0, len(partNumbers))
	for _, partNumber := range partNumbers {
		uploadURL, err := s.objectStorage.PresignUploadPart(
			ctx, objectKey, multipartUploadID, partNumber, s.videoPolicy.UploadURLTTL,
		)
		if err != nil {
			return nil, fmt.Errorf("%w: presign upload part %d: %v", ErrVideoStorageUnavailable, partNumber, err)
		}
		tickets = append(tickets, VideoUploadPartTicket{PartNumber: partNumber, UploadURL: uploadURL})
	}
	return tickets, nil
}

func allVideoPartNumbers(fileSize int64) ([]int, error) {
	if fileSize <= 0 {
		return nil, domain.ErrInvalidCourseVideo
	}
	partCount := int((fileSize-1)/VideoUploadPartSizeBytes + 1)
	if partCount <= 0 || partCount > maxMultipartPartNumber {
		return nil, domain.ErrInvalidCourseVideo
	}
	partNumbers := make([]int, partCount)
	for index := range partNumbers {
		partNumbers[index] = index + 1
	}
	return partNumbers, nil
}

func (s *Service) abortMultipartUploadBestEffort(ctx context.Context, objectKey, multipartUploadID string) {
	abortCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = s.objectStorage.AbortMultipartUpload(abortCtx, objectKey, multipartUploadID)
}

// CompleteCourseVideoUpload 根据上传任务核验对象，并原子地将上传任务更新为 promoted、
// 课程视频更新为 ready。已经 promoted 的任务直接返回当前视频，实现完成请求幂等。
func (s *Service) CompleteCourseVideoUpload(ctx context.Context, uploadID uint64, durationMS *uint64) (*domain.CourseVideo, error) {
	if s.objectStorage == nil {
		return nil, ErrVideoStorageUnavailable
	}
	upload, err := s.repository.GetCourseVideoUpload(ctx, uploadID)
	if err != nil {
		return nil, err
	}
	if upload.IsPromoted() {
		return s.repository.GetCourseVideo(ctx, upload.CourseVideoID)
	}
	if err := upload.EnsurePending(); err != nil {
		return nil, err
	}
	// 服务端从 OSS 读取分片和 ETag 后完成合并，不信任客户端声明的分片列表。
	object, err := s.completeMultipartObject(ctx, *upload)
	if errors.Is(err, ErrStoredObjectNotFound) {
		return nil, domain.ErrVideoUploadIncomplete
	}
	if errors.Is(err, domain.ErrVideoUploadIncomplete) {
		return nil, err
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrVideoStorageUnavailable, err)
	}
	if err := s.domainVideoPolicy().ValidateStored(domain.VideoFileMetadata{
		ContentType: object.ContentType, Size: object.Size,
	}); err != nil {
		return nil, err
	}
	var video *domain.CourseVideo
	// OSS 校验期间任务可能被另一次上传取代，因此事务内锁定并重新检查状态。
	err = s.repository.WithinTransaction(ctx, func(txCtx context.Context) error {
		// 与申请上传保持一致的 video → upload 加锁顺序，避免并发申请和完成互相等待。
		currentVideo, err := s.repository.GetCourseVideoForUpdate(txCtx, upload.CourseVideoID)
		if err != nil {
			return err
		}
		currentUpload, err := s.repository.GetCourseVideoUploadForUpdate(txCtx, uploadID)
		if err != nil {
			return err
		}
		if currentUpload.IsPromoted() {
			video = currentVideo
			return nil
		}
		if err := currentUpload.EnsureCompletes(*currentVideo); err != nil {
			return err
		}
		expectedVideoStatus := currentVideo.Status
		if err := currentVideo.CompleteUpload(durationMS); err != nil {
			return err
		}
		if err := s.repository.SaveCourseVideo(txCtx, currentVideo, expectedVideoStatus); err != nil {
			return err
		}
		expectedUploadStatus := currentUpload.Status
		if err := currentUpload.Promote(); err != nil {
			return err
		}
		if err := s.repository.SaveCourseVideoUpload(txCtx, currentUpload, expectedUploadStatus); err != nil {
			return err
		}
		video = currentVideo
		return nil
	})
	return video, err
}

func (s *Service) domainVideoPolicy() domain.VideoUploadPolicy {
	return domain.VideoUploadPolicy{MaxSizeBytes: s.videoPolicy.MaxVideoSizeBytes}
}

func (s *Service) completeMultipartObject(
	ctx context.Context,
	upload domain.CourseVideoUpload,
) (StoredObject, error) {
	parts, err := s.objectStorage.ListUploadedParts(ctx, upload.ObjectKey, upload.MultipartUploadID)
	if errors.Is(err, ErrMultipartUploadNotFound) {
		// OSS 已合并但进程在数据库提交前中断时，会话已经不存在而最终对象已经存在。
		return s.objectStorage.StatObject(ctx, upload.ObjectKey)
	}
	if err != nil {
		return StoredObject{}, fmt.Errorf("list multipart upload parts: %w", err)
	}
	if !validUploadedVideoParts(parts, upload.FileSize) {
		return StoredObject{}, domain.ErrVideoUploadIncomplete
	}
	if err := s.objectStorage.CompleteMultipartUpload(
		ctx, upload.ObjectKey, upload.MultipartUploadID, parts,
	); err != nil && !errors.Is(err, ErrMultipartUploadNotFound) {
		return StoredObject{}, fmt.Errorf("complete multipart upload: %w", err)
	}
	return s.objectStorage.StatObject(ctx, upload.ObjectKey)
}

func validUploadedVideoParts(parts []UploadedPart, fileSize int64) bool {
	if fileSize <= 0 {
		return false
	}
	wantPartCount := int((fileSize + VideoUploadPartSizeBytes - 1) / VideoUploadPartSizeBytes)
	return len(parts) == wantPartCount && validUploadedVideoPartSubset(parts, fileSize)
}

func validUploadedVideoPartSubset(parts []UploadedPart, fileSize int64) bool {
	if fileSize <= 0 {
		return false
	}
	wantPartCount := int((fileSize-1)/VideoUploadPartSizeBytes + 1)
	previousPartNumber := 0
	for _, part := range parts {
		if part.PartNumber <= previousPartNumber || part.PartNumber > wantPartCount {
			return false
		}
		wantSize := VideoUploadPartSizeBytes
		if part.PartNumber == wantPartCount {
			wantSize = fileSize - int64(part.PartNumber-1)*VideoUploadPartSizeBytes
		}
		if strings.TrimSpace(part.ETag) == "" || part.Size != wantSize {
			return false
		}
		previousPartNumber = part.PartNumber
	}
	return true
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
	id, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	// UUID v4 既避免同名文件互相覆盖，也不向对象存储暴露用户原始文件名。
	return fmt.Sprintf("course-videos/%d/%s.mp4", courseID, id.String()), nil
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
	if s.eligibilityFilter != nil && query.StudentID != 0 {
		classIDs, ready, err := s.eligibilityFilter.ListEligibleClassIDs(ctx, query.RoundID, query.StudentID)
		if err != nil {
			return nil, err
		}
		if ready {
			if len(classIDs) == 0 {
				return []TeachingClassView{}, nil
			}
			query.EligibleClassIDs = classIDs
			query.EligibilityFiltered = true
		}
	}
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
