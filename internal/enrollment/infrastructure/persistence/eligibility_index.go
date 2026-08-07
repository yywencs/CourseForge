package enrollmentrepo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	redislib "github.com/redis/go-redis/v9"
	enrollmentapp "github.com/yywencs/courseforge/internal/enrollment/application"
	"github.com/yywencs/courseforge/internal/enrollment/domain"
	"github.com/yywencs/courseforge/internal/platform/cache"
)

const emptyEligibilitySetSentinel = "0"

const renewWarmupLockScript = `
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 0 end
return redis.call('PEXPIRE', KEYS[1], ARGV[2])
`

const releaseWarmupLockScript = `
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return 0 end
return redis.call('DEL', KEYS[1])
`

const activateWarmupVersionScript = `
redis.call('SET', KEYS[1], ARGV[1], 'PX', ARGV[3])
redis.call('SET', KEYS[2], ARGV[2], 'PX', ARGV[4])
return 1
`

const openRoundVersionScript = `
if redis.call('GET', KEYS[1]) ~= ARGV[1] then return -1 end
local ttl = redis.call('PTTL', KEYS[1])
if ttl <= 0 then return -2 end
redis.call('SET', KEYS[2], ARGV[1], 'PX', ttl)
return 1
`

const querySelectionAdmissionScript = `
local active_version = redis.call('GET', KEYS[1])
if not active_version or active_version ~= ARGV[1] or
   redis.call('GET', KEYS[2]) ~= active_version then
	return {-1}
end

local round = redis.call('HMGET', KEYS[3], 'term_id', 'start_ms', 'end_ms')
local term_id = tonumber(round[1])
local start_ms = tonumber(round[2])
local end_ms = tonumber(round[3])
local now_ms = tonumber(ARGV[3])
if not term_id or not start_ms or not end_ms or not now_ms or
   now_ms < start_ms or now_ms >= end_ms then
	return {-1}
end

local class = redis.call('HMGET', KEYS[4], 'course_id', 'credits', 'capacity')
local course_id = tonumber(class[1])
local credits = tonumber(class[2])
local capacity = tonumber(class[3])
if not course_id or not credits or not capacity then
	return {-2}
end
if redis.call('EXISTS', KEYS[5]) == 0 then
	return {-3}
end

local credit_remaining = tonumber(redis.call('GET', KEYS[6]))
local course_remaining = tonumber(redis.call('GET', KEYS[7]))
local seat_remaining = tonumber(redis.call('GET', KEYS[8]))
if not credit_remaining or not course_remaining or not seat_remaining or
   redis.call('EXISTS', KEYS[10]) == 0 or redis.call('EXISTS', KEYS[11]) == 0 then
	return {-1}
end

local conflict = 0
local slots = redis.call('SMEMBERS', KEYS[10])
for _, slot in ipairs(slots) do
	if slot ~= '0' and tonumber(redis.call('HGET', KEYS[11], slot) or '0') > 0 then
		conflict = 1
		break
	end
end

return {
	0, term_id, start_ms, end_ms, course_id, credits, capacity, seat_remaining,
	redis.call('SISMEMBER', KEYS[5], ARGV[2]),
	redis.call('HEXISTS', KEYS[9], tostring(course_id)), conflict, credit_remaining, course_remaining
}
`

// EligibilityIndex 使用版本化 Redis Set 存储学生可选教学班。
type EligibilityIndex struct {
	redis *cache.Cache
}

func NewEligibilityIndex(redis *cache.Cache) *EligibilityIndex {
	return &EligibilityIndex{redis: redis}
}

func warmupLockKey(roundID uint64) string {
	return fmt.Sprintf("courseforge:selection:warmup:%d:lock", roundID)
}

func warmupStatusKey(roundID uint64) string {
	return fmt.Sprintf("courseforge:selection:warmup:%d:status", roundID)
}

func activeEligibilityVersionKey(roundID uint64) string {
	return fmt.Sprintf("courseforge:selection:warmup:%d:active_version", roundID)
}

func studentEligibilityKey(roundID uint64, version string, studentID uint64) string {
	return fmt.Sprintf("courseforge:selection:eligible:%d:%s:%d", roundID, version, studentID)
}

func (r *EligibilityIndex) TryLock(
	ctx context.Context, roundID uint64, token string, ttl time.Duration,
) (bool, error) {
	if r == nil || r.redis == nil || roundID == 0 || token == "" || ttl <= 0 {
		return false, enrollmentapp.ErrRoundWarmupNotReady
	}
	return r.redis.SetNX(ctx, warmupLockKey(roundID), token, ttl)
}

func (r *EligibilityIndex) RenewLock(
	ctx context.Context, roundID uint64, token string, ttl time.Duration,
) error {
	result, err := r.redis.Eval(ctx, renewWarmupLockScript, []string{warmupLockKey(roundID)}, token, ttl.Milliseconds())
	if err != nil {
		return err
	}
	if value, ok := result.(int64); !ok || value != 1 {
		return enrollmentapp.ErrRoundWarmupRunning
	}
	return nil
}

func (r *EligibilityIndex) ReleaseLock(ctx context.Context, roundID uint64, token string) error {
	_, err := r.redis.Eval(ctx, releaseWarmupLockScript, []string{warmupLockKey(roundID)}, token)
	return err
}

func (r *EligibilityIndex) MarkRunning(
	ctx context.Context, status enrollmentapp.RoundWarmupStatus, ttl time.Duration,
) error {
	return r.setStatus(ctx, status, ttl)
}

func (r *EligibilityIndex) MarkQueued(
	ctx context.Context, status enrollmentapp.RoundWarmupStatus, ttl time.Duration,
) error {
	return r.setStatus(ctx, status, ttl)
}

func (r *EligibilityIndex) MarkFailed(
	ctx context.Context, status enrollmentapp.RoundWarmupStatus, ttl time.Duration,
) error {
	return r.setStatus(ctx, status, ttl)
}

func (r *EligibilityIndex) setStatus(
	ctx context.Context, status enrollmentapp.RoundWarmupStatus, ttl time.Duration,
) error {
	return r.redis.Set(&cache.Item{
		Ctx: ctx, Key: warmupStatusKey(status.RoundID), Value: status, TTL: ttl, SkipLocalCache: true,
	})
}

func (r *EligibilityIndex) WriteSnapshot(
	ctx context.Context,
	snapshot *enrollmentapp.RoundWarmupSnapshot,
	version string,
	ttl time.Duration,
) error {
	if snapshot == nil || snapshot.Round.ID == 0 || version == "" || ttl <= 0 {
		return enrollmentapp.ErrRoundWarmupNotReady
	}
	classCells := make(map[uint64][]string, len(snapshot.Classes))
	for _, class := range snapshot.Classes {
		cells, err := scheduleCells(class.Schedules)
		if err != nil {
			return fmt.Errorf("教学班 %d 课表非法: %w", class.ID, err)
		}
		classCells[class.ID] = cells
	}
	pipe := r.redis.Pipeline()
	if pipe == nil {
		return cache.ErrCacheMiss
	}
	roundKey := roundSnapshotKey(snapshot.Round.ID, version)
	pipe.HSet(ctx, roundKey,
		"term_id", snapshot.Round.TermID,
		"start_ms", snapshot.Round.StartTime.UnixMilli(),
		"end_ms", snapshot.Round.EndTime.UnixMilli(),
	)
	pipe.Expire(ctx, roundKey, ttl)
	for _, class := range snapshot.Classes {
		remaining := int64(class.Capacity) - int64(class.SelectedCount)
		if remaining < 0 {
			return fmt.Errorf("教学班 %d 名额快照非法", class.ID)
		}
		classKey := teachingClassSnapshotKey(snapshot.Round.ID, version, class.ID)
		pipe.HSet(ctx, classKey,
			"course_id", class.CourseID,
			"credits", int64(class.Credits),
			"capacity", class.Capacity,
		)
		pipe.Expire(ctx, classKey, ttl)
		pipe.Set(ctx, teachingClassSeatKey(class.ID), remaining, 0)
		scheduleKey := teachingClassScheduleKey(class.ID)
		pipe.Del(ctx, scheduleKey)
		members := make([]interface{}, 0, len(classCells[class.ID])+1)
		members = append(members, emptyEligibilitySetSentinel)
		for _, cell := range classCells[class.ID] {
			members = append(members, cell)
		}
		pipe.SAdd(ctx, scheduleKey, members...)
		pipe.Persist(ctx, scheduleKey)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (r *EligibilityIndex) WriteStudents(
	ctx context.Context,
	roundID uint64,
	version string,
	students []enrollmentapp.EligibilityIndexStudent,
	ttl time.Duration,
) error {
	if roundID == 0 || version == "" || ttl <= 0 {
		return enrollmentapp.ErrRoundWarmupNotReady
	}
	pipe := r.redis.Pipeline()
	if pipe == nil {
		return cache.ErrCacheMiss
	}
	for _, student := range students {
		enrollmentCells := make(map[string][]string, len(student.Enrollments))
		for _, existing := range student.Enrollments {
			cells, err := scheduleCells(existing.Schedules)
			if err != nil {
				return fmt.Errorf("学生 %d 已选课表非法: %w", student.StudentID, err)
			}
			enrollmentCells[existing.ApplicationID] = cells
		}
		eligibleKey := studentEligibilityKey(roundID, version, student.StudentID)
		pipe.Del(ctx, eligibleKey)
		members := make([]interface{}, 0, len(student.EligibleClassIDs)+1)
		members = append(members, emptyEligibilitySetSentinel)
		for _, classID := range student.EligibleClassIDs {
			members = append(members, strconv.FormatUint(classID, 10))
		}
		pipe.SAdd(ctx, eligibleKey, members...)
		pipe.Expire(ctx, eligibleKey, ttl)

		creditRemaining := student.Quota.CreditLimit - student.Quota.SelectedCredits
		courseRemaining := int64(student.Quota.CourseLimit) - int64(student.Quota.SelectedCourseCount)
		if creditRemaining < 0 || courseRemaining < 0 {
			return fmt.Errorf("学生 %d 额度快照非法", student.StudentID)
		}
		creditKey := studentCreditKey(roundID, student.StudentID)
		courseKey := studentCourseQuotaKey(roundID, student.StudentID)
		pipe.Set(ctx, creditKey, int64(creditRemaining), 0)
		pipe.Set(ctx, courseKey, courseRemaining, 0)

		scheduleKey := studentScheduleKey(roundID, student.StudentID)
		coursesKey := studentCourseSelectionKey(roundID, student.StudentID)
		pipe.Del(ctx, scheduleKey)
		pipe.Del(ctx, coursesKey)
		pipe.HSet(ctx, scheduleKey, emptyEligibilitySetSentinel, 0)
		pipe.HSet(ctx, coursesKey, emptyEligibilitySetSentinel, emptyEligibilitySetSentinel)
		pipe.Persist(ctx, scheduleKey)
		pipe.Persist(ctx, coursesKey)
		for _, existing := range student.Enrollments {
			pipe.HSet(ctx, coursesKey, strconv.FormatUint(existing.CourseID, 10), existing.ApplicationID)
			for _, cell := range enrollmentCells[existing.ApplicationID] {
				pipe.HIncrBy(ctx, scheduleKey, cell, 1)
			}
		}
	}
	_, err := pipe.Exec(ctx)
	return err
}

// MarkOpen 在数据库轮次开放后激活同一预热版本的 Redis 开放门闩。
func (r *EligibilityIndex) MarkOpen(ctx context.Context, roundID uint64, version string) error {
	if r == nil || r.redis == nil || roundID == 0 || version == "" {
		return enrollmentapp.ErrRoundWarmupNotReady
	}
	result, err := r.redis.Eval(
		ctx,
		openRoundVersionScript,
		[]string{activeEligibilityVersionKey(roundID), roundOpenVersionKey(roundID)},
		version,
	)
	if err != nil {
		return err
	}
	value, ok := result.(int64)
	if !ok || value != 1 {
		return enrollmentapp.ErrRoundWarmupNotReady
	}
	return nil
}

func (r *EligibilityIndex) Activate(
	ctx context.Context,
	status enrollmentapp.RoundWarmupStatus,
	activeTTL time.Duration,
	statusTTL time.Duration,
) error {
	encoded, err := r.redis.Marshal(status)
	if err != nil {
		return err
	}
	_, err = r.redis.Eval(
		ctx,
		activateWarmupVersionScript,
		[]string{activeEligibilityVersionKey(status.RoundID), warmupStatusKey(status.RoundID)},
		status.Version,
		encoded,
		activeTTL.Milliseconds(),
		statusTTL.Milliseconds(),
	)
	return err
}

func (r *EligibilityIndex) Status(ctx context.Context, roundID uint64) (*enrollmentapp.RoundWarmupStatus, error) {
	var status enrollmentapp.RoundWarmupStatus
	err := r.redis.GetSkippingLocalCache(ctx, warmupStatusKey(roundID), &status)
	if errors.Is(err, cache.ErrCacheMiss) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if status.State == enrollmentapp.RoundWarmupStateReady {
		version, active, versionErr := r.activeVersion(ctx, roundID)
		if versionErr != nil {
			return nil, versionErr
		}
		if !active || version != status.Version {
			status.State = enrollmentapp.RoundWarmupStateFailed
			status.ErrorMessage = "资格索引已过期，请重新预热"
		}
	}
	return &status, nil
}

// IsEligible 判断 active_version 下学生是否具有目标教学班的静态资格。
// ready=false 表示尚无可用预热版本，调用方可以选择兼容性回源。
func (r *EligibilityIndex) IsEligible(
	ctx context.Context, roundID, studentID, teachingClassID uint64,
) (eligible bool, ready bool, err error) {
	version, ready, err := r.activeVersion(ctx, roundID)
	if err != nil || !ready {
		return false, ready, err
	}
	result, err := r.redis.Eval(
		ctx,
		"return redis.call('SISMEMBER', KEYS[1], ARGV[1])",
		[]string{studentEligibilityKey(roundID, version, studentID)},
		strconv.FormatUint(teachingClassID, 10),
	)
	if err != nil {
		return false, true, err
	}
	eligibleValue, ok := result.(int64)
	return ok && eligibleValue == 1, true, nil
}

// QuerySelectionAdmission 从当前开放版本读取一次选课准入快照。
// 所有事实都来自 Redis；版本、开放门闩或关键资源缺失时直接 fail closed。
func (r *EligibilityIndex) QuerySelectionAdmission(
	ctx context.Context,
	roundID uint64,
	studentID uint64,
	teachingClassID uint64,
	now time.Time,
) (*enrollmentapp.SelectionAdmissionSnapshot, error) {
	if r == nil || r.redis == nil || roundID == 0 || studentID == 0 ||
		teachingClassID == 0 || now.IsZero() {
		return nil, enrollment.ErrInvalidParams
	}
	version, ready, err := r.activeVersion(ctx, roundID)
	if err != nil {
		return nil, err
	}
	if !ready {
		return nil, enrollment.ErrRoundNotOpen
	}
	raw, err := r.redis.Eval(
		ctx,
		querySelectionAdmissionScript,
		[]string{
			activeEligibilityVersionKey(roundID),
			roundOpenVersionKey(roundID),
			roundSnapshotKey(roundID, version),
			teachingClassSnapshotKey(roundID, version, teachingClassID),
			studentEligibilityKey(roundID, version, studentID),
			studentCreditKey(roundID, studentID),
			studentCourseQuotaKey(roundID, studentID),
			teachingClassSeatKey(teachingClassID),
			studentCourseSelectionKey(roundID, studentID),
			teachingClassScheduleKey(teachingClassID),
			studentScheduleKey(roundID, studentID),
		},
		version,
		strconv.FormatUint(teachingClassID, 10),
		now.UnixMilli(),
	)
	if err != nil {
		return nil, err
	}
	values, ok := raw.([]interface{})
	if !ok || len(values) == 0 {
		return nil, fmt.Errorf("非法Redis准入响应: %#v", raw)
	}
	status, ok := values[0].(int64)
	if !ok {
		return nil, fmt.Errorf("非法Redis准入状态: %#v", values[0])
	}
	switch status {
	case -1:
		return nil, enrollment.ErrRoundNotOpen
	case -2:
		return nil, enrollment.ErrTeachingClassNotOpen
	case -3:
		return nil, enrollment.ErrEligibilityNotMet
	case 0:
	default:
		return nil, fmt.Errorf("未知Redis准入状态: %d", status)
	}
	if len(values) != 13 {
		return nil, fmt.Errorf("非法Redis准入字段: %#v", values)
	}
	termID, err := redisInt64(values[1])
	if err != nil {
		return nil, err
	}
	startMillis, err := redisInt64(values[2])
	if err != nil {
		return nil, err
	}
	endMillis, err := redisInt64(values[3])
	if err != nil {
		return nil, err
	}
	courseID, err := redisInt64(values[4])
	if err != nil {
		return nil, err
	}
	credits, err := redisInt64(values[5])
	if err != nil {
		return nil, err
	}
	capacity, err := redisInt64(values[6])
	if err != nil {
		return nil, err
	}
	seatRemaining, err := redisInt64(values[7])
	if err != nil {
		return nil, err
	}
	eligible, err := redisInt64(values[8])
	if err != nil {
		return nil, err
	}
	existing, err := redisInt64(values[9])
	if err != nil {
		return nil, err
	}
	conflict, err := redisInt64(values[10])
	if err != nil {
		return nil, err
	}
	creditRemaining, err := redisInt64(values[11])
	if err != nil {
		return nil, err
	}
	courseRemaining, err := redisInt64(values[12])
	if err != nil {
		return nil, err
	}
	if termID <= 0 || courseID <= 0 || credits <= 0 || capacity < 0 ||
		seatRemaining < 0 || seatRemaining > capacity || creditRemaining < 0 {
		return nil, enrollment.ErrRoundNotOpen
	}
	selectedCount := capacity - seatRemaining
	return &enrollmentapp.SelectionAdmissionSnapshot{
		Round: &enrollment.SelectionRound{
			ID: roundID, TermID: uint64(termID), StartTime: time.UnixMilli(startMillis),
			EndTime: time.UnixMilli(endMillis), State: enrollment.SelectionRoundStateOpen,
		},
		Class: &enrollment.TeachingClass{
			ID: teachingClassID, TermID: uint64(termID), CourseID: uint64(courseID),
			Credits: enrollment.Credit(credits), Capacity: uint32(capacity),
			SelectedCount: uint32(selectedCount), State: enrollment.TeachingClassStateOpen,
		},
		Eligible: eligible == 1, ExistingEnrollment: existing == 1,
		ScheduleConflict: conflict == 1, CreditRemaining: enrollment.Credit(creditRemaining),
		CourseQuotaRemaining: courseRemaining,
	}, nil
}

// ListEligibleClassIDs 返回 active_version 下学生可见的全部教学班 ID。
func (r *EligibilityIndex) ListEligibleClassIDs(
	ctx context.Context, roundID, studentID uint64,
) ([]uint64, bool, error) {
	version, ready, err := r.activeVersion(ctx, roundID)
	if err != nil || !ready {
		return nil, ready, err
	}
	result, err := r.redis.Eval(
		ctx,
		"return redis.call('SMEMBERS', KEYS[1])",
		[]string{studentEligibilityKey(roundID, version, studentID)},
	)
	if err != nil {
		return nil, true, err
	}
	values, ok := result.([]interface{})
	if !ok {
		return nil, true, nil
	}
	ids := make([]uint64, 0, len(values))
	for _, value := range values {
		raw, ok := value.(string)
		if !ok {
			continue
		}
		if raw == emptyEligibilitySetSentinel {
			continue
		}
		id, parseErr := strconv.ParseUint(raw, 10, 64)
		if parseErr != nil {
			return nil, true, parseErr
		}
		ids = append(ids, id)
	}
	// active_version 已存在时必须 fail closed。学生集合缺失通常表示没有轮次额度，
	// 也可能是缓存被淘汰；两种情况都不能回退成“显示全部教学班”。
	return ids, true, nil
}

func scheduleCells(slots []enrollment.ScheduleSlot) ([]string, error) {
	cells := make(map[string]struct{})
	for _, slot := range slots {
		if slot.DayOfWeek < 1 || slot.DayOfWeek > 7 || slot.StartWeek < 1 ||
			slot.EndWeek < slot.StartWeek || slot.StartSection < 1 ||
			slot.EndSection < slot.StartSection {
			return nil, enrollment.ErrInvalidParams
		}
		for week := int(slot.StartWeek); week <= int(slot.EndWeek); week++ {
			for section := int(slot.StartSection); section <= int(slot.EndSection); section++ {
				cells[fmt.Sprintf("%d:%d:%d", week, slot.DayOfWeek, section)] = struct{}{}
			}
		}
	}
	result := make([]string, 0, len(cells))
	for cell := range cells {
		result = append(result, cell)
	}
	return result, nil
}

func redisInt64(value interface{}) (int64, error) {
	parsed, ok := value.(int64)
	if !ok {
		return 0, fmt.Errorf("非法Redis整数: %#v", value)
	}
	return parsed, nil
}

func (r *EligibilityIndex) activeVersion(ctx context.Context, roundID uint64) (string, bool, error) {
	result, err := r.redis.Eval(ctx, "return redis.call('GET', KEYS[1])", []string{activeEligibilityVersionKey(roundID)})
	if errors.Is(err, redislib.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if result == nil {
		return "", false, nil
	}
	version, ok := result.(string)
	return version, ok && version != "", nil
}
