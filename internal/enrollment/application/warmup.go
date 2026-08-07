package enrollmentapp

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/yywencs/courseforge/internal/enrollment/domain"
)

const defaultWarmupPageSize = 500

var (
	ErrRoundWarmupRunning  = errors.New("轮次资格预热正在执行")
	ErrRoundWarmupNotReady = errors.New("轮次资格预热尚未完成")
)

type RoundWarmupState string

const (
	RoundWarmupStateQueued  RoundWarmupState = "queued"
	RoundWarmupStateRunning RoundWarmupState = "running"
	RoundWarmupStateReady   RoundWarmupState = "ready"
	RoundWarmupStateFailed  RoundWarmupState = "failed"
)

// RoundWarmupStatus 描述一个轮次当前生效的资格快照或最近一次预热结果。
type RoundWarmupStatus struct {
	RoundID       uint64           `json:"round_id"`
	Version       string           `json:"version"`
	State         RoundWarmupState `json:"state"`
	StudentCount  int              `json:"student_count"`
	EligibleCount int              `json:"eligible_count"`
	StartedAt     time.Time        `json:"started_at"`
	FinishedAt    *time.Time       `json:"finished_at,omitempty"`
	ErrorMessage  string           `json:"error_message,omitempty"`
}

// WarmupClass 是预热一次读取的教学班静态事实。
type WarmupClass struct {
	ID               uint64
	CourseID         uint64
	Credits          enrollment.Credit
	Capacity         uint32
	SelectedCount    uint32
	MinimumGradeYear *uint16
	MaximumGradeYear *uint16
	MajorScopes      []enrollment.MajorScope
	Prerequisites    []enrollment.PrerequisiteRequirement
	Schedules        []enrollment.ScheduleSlot
}

// WarmupEnrollment 是预热学生已有课程占用和课表占用所需的最小事实。
type WarmupEnrollment struct {
	ApplicationID   string
	CourseID        uint64
	TeachingClassID uint64
	Schedules       []enrollment.ScheduleSlot
}

// WarmupStudent 是按页读取的学生静态事实和当前额度镜像。
type WarmupStudent struct {
	Profile      enrollment.StudentProfile
	Quota        enrollment.StudentSelectionQuota
	Achievements []enrollment.CourseAchievement
	Enrollments  []WarmupEnrollment
}

// RoundWarmupSnapshot 是轮次级、只需读取一次的预热事实。
type RoundWarmupSnapshot struct {
	Round   enrollment.SelectionRound
	Classes []WarmupClass
}

// RoundWarmupSource 批量提供预热所需事实，禁止由服务逐学生、逐教学班查询。
type RoundWarmupSource interface {
	LoadRoundWarmupSnapshot(context.Context, uint64) (*RoundWarmupSnapshot, error)
	ListRoundWarmupStudents(context.Context, uint64, uint64, int) ([]WarmupStudent, error)
}

// EligibilityIndexStudent 是一次批量写入 Redis 的学生资格结果。
type EligibilityIndexStudent struct {
	StudentID        uint64
	EligibleClassIDs []uint64
	Quota            enrollment.StudentSelectionQuota
	Enrollments      []WarmupEnrollment
}

// EligibilityWarmupIndex 隐藏版本化 Redis Set、锁和原子版本切换细节。
type EligibilityWarmupIndex interface {
	TryLock(context.Context, uint64, string, time.Duration) (bool, error)
	RenewLock(context.Context, uint64, string, time.Duration) error
	ReleaseLock(context.Context, uint64, string) error
	MarkRunning(context.Context, RoundWarmupStatus, time.Duration) error
	MarkQueued(context.Context, RoundWarmupStatus, time.Duration) error
	WriteSnapshot(context.Context, *RoundWarmupSnapshot, string, time.Duration) error
	WriteStudents(context.Context, uint64, string, []EligibilityIndexStudent, time.Duration) error
	Activate(context.Context, RoundWarmupStatus, time.Duration, time.Duration) error
	MarkOpen(context.Context, uint64, string) error
	MarkFailed(context.Context, RoundWarmupStatus, time.Duration) error
	Status(context.Context, uint64) (*RoundWarmupStatus, error)
}

type WarmupVersionGenerator interface {
	NewID() (string, error)
}

// RoundWarmupService 生成并原子发布一个轮次的静态资格索引。
type RoundWarmupService struct {
	source   RoundWarmupSource
	index    EligibilityWarmupIndex
	versions WarmupVersionGenerator
	policy   enrollment.StaticEligibilityPolicy
	now      func() time.Time
	pageSize int
}

func NewRoundWarmupService(
	source RoundWarmupSource,
	index EligibilityWarmupIndex,
	versions WarmupVersionGenerator,
) *RoundWarmupService {
	return &RoundWarmupService{
		source: source, index: index, versions: versions,
		policy: enrollment.StaticEligibilityPolicy{}, now: time.Now,
		pageSize: defaultWarmupPageSize,
	}
}

// Warmup 计算新版本，全部写完后才切换 active_version；旧版本不会被半成品覆盖。
func (s *RoundWarmupService) Warmup(ctx context.Context, roundID uint64) (status *RoundWarmupStatus, err error) {
	if s == nil || s.source == nil || s.index == nil || s.versions == nil || roundID == 0 {
		return nil, enrollment.ErrInvalidParams
	}
	existing, err := s.index.Status(ctx, roundID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.State == RoundWarmupStateReady {
		return nil, enrollment.ErrRoundAlreadyWarmed
	}
	version, err := s.versions.NewID()
	if err != nil {
		return nil, fmt.Errorf("生成预热版本: %w", err)
	}
	const lockTTL = 2 * time.Minute
	locked, err := s.index.TryLock(ctx, roundID, version, lockTTL)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, ErrRoundWarmupRunning
	}
	defer func() { _ = s.index.ReleaseLock(context.Background(), roundID, version) }()

	startedAt := s.now()
	status = &RoundWarmupStatus{
		RoundID: roundID, Version: version, State: RoundWarmupStateRunning, StartedAt: startedAt,
	}
	const metadataTTL = 30 * 24 * time.Hour
	if err = s.index.MarkRunning(ctx, *status, metadataTTL); err != nil {
		finishedAt := s.now()
		status.State = RoundWarmupStateFailed
		status.FinishedAt = &finishedAt
		status.ErrorMessage = err.Error()
		_ = s.index.MarkFailed(context.Background(), *status, metadataTTL)
		return nil, err
	}
	defer func() {
		if err == nil {
			return
		}
		finishedAt := s.now()
		status.State = RoundWarmupStateFailed
		status.FinishedAt = &finishedAt
		status.ErrorMessage = err.Error()
		_ = s.index.MarkFailed(context.Background(), *status, metadataTTL)
	}()

	snapshot, err := s.source.LoadRoundWarmupSnapshot(ctx, roundID)
	if err != nil {
		return status, err
	}
	if snapshot == nil || snapshot.Round.ID != roundID ||
		(snapshot.Round.State != enrollment.SelectionRoundStatePlanned &&
			snapshot.Round.State != enrollment.SelectionRoundStateOpen) {
		return status, enrollment.ErrRoundNotEditable
	}
	if len(snapshot.Classes) == 0 {
		return status, enrollment.ErrInvalidParams
	}
	dataTTL := snapshot.Round.EndTime.Add(7 * 24 * time.Hour).Sub(s.now())
	if dataTTL < time.Hour {
		dataTTL = time.Hour
	}
	if err = s.index.WriteSnapshot(ctx, snapshot, version, dataTTL); err != nil {
		return status, err
	}

	var afterStudentID uint64
	for {
		students, queryErr := s.source.ListRoundWarmupStudents(ctx, roundID, afterStudentID, s.pageSize)
		if queryErr != nil {
			return status, queryErr
		}
		if len(students) == 0 {
			break
		}
		if err = s.index.RenewLock(ctx, roundID, version, lockTTL); err != nil {
			return status, err
		}
		batch := make([]EligibilityIndexStudent, 0, len(students))
		for _, student := range students {
			eligible := make([]uint64, 0, len(snapshot.Classes))
			for _, class := range snapshot.Classes {
				facts := &enrollment.EligibilitySnapshot{
					Student:          &student.Profile,
					MinimumGradeYear: class.MinimumGradeYear,
					MaximumGradeYear: class.MaximumGradeYear,
					MajorScopes:      class.MajorScopes,
					Prerequisites:    class.Prerequisites,
					Achievements:     student.Achievements,
				}
				if s.policy.Evaluate(facts) == nil {
					eligible = append(eligible, class.ID)
					status.EligibleCount++
				}
			}
			batch = append(batch, EligibilityIndexStudent{
				StudentID: student.Profile.ID, EligibleClassIDs: eligible, Quota: student.Quota,
				Enrollments: student.Enrollments,
			})
			afterStudentID = student.Profile.ID
			status.StudentCount++
		}
		if err = s.index.WriteStudents(ctx, roundID, version, batch, dataTTL); err != nil {
			return status, err
		}
		if err = s.index.RenewLock(ctx, roundID, version, lockTTL); err != nil {
			return status, err
		}
		if len(students) < s.pageSize {
			break
		}
	}
	if status.StudentCount == 0 {
		return status, enrollment.ErrRecordNotFound
	}
	finishedAt := s.now()
	status.State = RoundWarmupStateReady
	status.FinishedAt = &finishedAt
	readyStatusTTL := metadataTTL
	if dataTTL > readyStatusTTL {
		readyStatusTTL = dataTTL
	}
	if err = s.index.Activate(ctx, *status, dataTTL, readyStatusTTL); err != nil {
		return status, err
	}
	// Redis 丢失后允许从仍处于 open 的 MySQL 轮次重建快照；激活新版本后必须
	// 同步恢复开放门闩，否则重建完成的实例仍会 fail closed。
	if snapshot.Round.State == enrollment.SelectionRoundStateOpen {
		if err = s.index.MarkOpen(ctx, roundID, version); err != nil {
			return status, err
		}
	}
	return status, nil
}

func (s *RoundWarmupService) Status(ctx context.Context, roundID uint64) (*RoundWarmupStatus, error) {
	if s == nil || s.index == nil || roundID == 0 {
		return nil, enrollment.ErrInvalidParams
	}
	return s.index.Status(ctx, roundID)
}
