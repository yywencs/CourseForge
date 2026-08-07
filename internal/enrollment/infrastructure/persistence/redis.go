package enrollmentrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	redislib "github.com/redis/go-redis/v9"
	applicationapi "github.com/yywencs/courseforge/internal/enrollment/application"
	"github.com/yywencs/courseforge/internal/enrollment/domain"
	"github.com/yywencs/courseforge/internal/platform/cache"
)

const (
	selectionRequestResultTTL    = 7 * 24 * time.Hour
	selectionResultRecoveryBatch = 100
)

const commitSelectionScript = `
	local active_version_key = KEYS[1]
	local open_version_key = KEYS[2]
	local round_key = KEYS[3]
	local class_key = KEYS[4]
	local eligibility_key = KEYS[5]
	local credit_key = KEYS[6]
	local course_quota_key = KEYS[7]
	local seat_key = KEYS[8]
	local pending_key = KEYS[9]
	local result_key = KEYS[10]
	local selected_courses_key = KEYS[11]
	local application_key = KEYS[12]
	local stream_key = KEYS[13]
	local class_schedule_key = KEYS[14]
	local student_schedule_key = KEYS[15]

	local application_id = ARGV[1]
	local request_id = ARGV[2]
	local result_json = ARGV[3]
	local result_ttl = ARGV[4]
	local expected_version = ARGV[5]
	local now_ms = tonumber(ARGV[6])
	local teaching_class_id = ARGV[7]

	local completed = redis.call('GET', result_key)
	if completed then
		return {1, completed}
	end

	local result_ok, result = pcall(cjson.decode, result_json)
	if not result_ok or result.application_id ~= application_id or
	   result.request_id ~= request_id or result.state ~= 'selected' or
	   tostring(result.teaching_class_id) ~= teaching_class_id then
		return {-6, ''}
	end
	local credits = tonumber(result.credits)
	if not credits or credits <= 0 then
		return {-6, ''}
	end

	local pending = redis.call('GET', pending_key)
	if pending then
		local ok, application = pcall(cjson.decode, pending)
		if not ok or application.request_id ~= request_id then
			return {2, pending}
		end
		return {-6, pending}
	end

	if redis.call('GET', active_version_key) ~= expected_version or
	   redis.call('GET', open_version_key) ~= expected_version then
		return {-8, ''}
	end
	local round = redis.call('HMGET', round_key, 'term_id', 'start_ms', 'end_ms')
	local term_id = tonumber(round[1])
	local start_ms = tonumber(round[2])
	local end_ms = tonumber(round[3])
	if not term_id or not start_ms or not end_ms or not now_ms or
	   now_ms < start_ms or now_ms >= end_ms or tonumber(result.term_id) ~= term_id then
		return {-8, ''}
	end
	local class = redis.call('HMGET', class_key, 'course_id', 'credits')
	local course_id = tonumber(class[1])
	local class_credits = tonumber(class[2])
	if not course_id or not class_credits or tonumber(result.course_id) ~= course_id or
	   credits ~= class_credits then
		return {-9, ''}
	end
	if redis.call('EXISTS', eligibility_key) == 0 or
	   redis.call('SISMEMBER', eligibility_key, teaching_class_id) ~= 1 then
		return {-10, ''}
	end
	if redis.call('HEXISTS', selected_courses_key, tostring(course_id)) == 1 then
		return {-4, ''}
	end
	if redis.call('EXISTS', credit_key) == 0 or
	   redis.call('EXISTS', course_quota_key) == 0 or
	   redis.call('EXISTS', seat_key) == 0 or
	   redis.call('EXISTS', class_schedule_key) == 0 or
	   redis.call('EXISTS', student_schedule_key) == 0 or
	   redis.call('EXISTS', selected_courses_key) == 0 then
		return {-1, ''}
	end

	local credit_remaining = tonumber(redis.call('GET', credit_key))
	local course_remaining = tonumber(redis.call('GET', course_quota_key))
	local seat_remaining = tonumber(redis.call('GET', seat_key))
	if not credit_remaining or not course_remaining or not seat_remaining then
		return {-6, ''}
	end
	if credit_remaining < credits then
		return {-2, ''}
	end
	if course_remaining <= 0 then
		return {-3, ''}
	end
	if seat_remaining <= 0 then
		return {-5, ''}
	end

	local slots = redis.call('SMEMBERS', class_schedule_key)
	for _, slot in ipairs(slots) do
		if slot ~= '0' and tonumber(redis.call('HGET', student_schedule_key, slot) or '0') > 0 then
			return {-7, ''}
		end
	end

	redis.call('DECRBY', credit_key, credits)
	redis.call('DECR', course_quota_key)
	redis.call('DECR', seat_key)
	redis.call('HSET', selected_courses_key, tostring(course_id), application_id)
	for _, slot in ipairs(slots) do
		if slot ~= '0' then
			redis.call('HINCRBY', student_schedule_key, slot, 1)
		end
	end

	local stream_id = redis.call('XADD', stream_key, '*', 'event', result_json)
	local publication = {
		stream_id = stream_id,
		broker_confirmed = false,
		result = result
	}
	local stored_publication = cjson.encode(publication)

	redis.call('SET', pending_key, result_json)
	redis.call('SET', result_key, stored_publication, 'EX', result_ttl)
	redis.call('SET', application_key, stored_publication, 'EX', result_ttl)
	return {0, stored_publication}
`

const markSelectionResultPublishedScript = `
	local raw = redis.call('GET', KEYS[1])
	if not raw then
		return -1
	end
	local ok, publication = pcall(cjson.decode, raw)
	if not ok or not publication.result or
	   publication.result.application_id ~= ARGV[1] or
	   publication.stream_id ~= ARGV[2] then
		return -2
	end
		publication.broker_confirmed = true
		local stored = cjson.encode(publication)
		redis.call('SET', KEYS[1], stored, 'KEEPTTL')
		redis.call('SET', KEYS[3], stored, 'KEEPTTL')
		redis.call('XDEL', KEYS[2], ARGV[2])
	return 1
`

const querySelectionResultStreamScript = `
	return redis.call('XRANGE', KEYS[1], '-', '+', 'COUNT', ARGV[1])
`

const querySelectionByRequestScript = `
	local result = redis.call('GET', KEYS[1])
	if result then
		return {2, result}
	end
	local pending = redis.call('GET', KEYS[2])
	if pending then
		return {1, pending}
	end
	return {0, ''}
`

const clearPersistedSelectionScript = `
	local pending = redis.call('GET', KEYS[1])
	if not pending then
		return 0
	end
	local ok, application = pcall(cjson.decode, pending)
	if not ok or application.application_id ~= ARGV[1] then
		return -1
	end
	redis.call('DEL', KEYS[1])
	return 1
`

const releaseDroppedEnrollmentScript = `
		if redis.call('EXISTS', KEYS[1]) == 1 then
			return 0
		end
		if redis.call('EXISTS', KEYS[2]) == 0 or
		   redis.call('EXISTS', KEYS[3]) == 0 or
		   redis.call('EXISTS', KEYS[4]) == 0 or
		   redis.call('EXISTS', KEYS[5]) == 0 or
		   redis.call('EXISTS', KEYS[6]) == 0 or
		   redis.call('EXISTS', KEYS[7]) == 0 then
			return -1
		end
		local credits = tonumber(ARGV[1])
		if not credits or credits <= 0 then
			return -2
		end
		redis.call('INCRBY', KEYS[2], credits)
		redis.call('INCR', KEYS[3])
		redis.call('INCR', KEYS[4])
		if redis.call('HGET', KEYS[5], ARGV[4]) == ARGV[2] then
			redis.call('HDEL', KEYS[5], ARGV[4])
		end
		local slots = redis.call('SMEMBERS', KEYS[6])
		for _, slot in ipairs(slots) do
			if slot ~= '0' then
				local remaining = redis.call('HINCRBY', KEYS[7], slot, -1)
				if remaining <= 0 then
					redis.call('HDEL', KEYS[7], slot)
				end
			end
		end
		redis.call('SET', KEYS[1], '1', 'EX', ARGV[3])
		return 1
`

type selectionApplicationPayload struct {
	ApplicationID   string                       `json:"application_id"`
	RequestID       string                       `json:"request_id"`
	RoundID         uint64                       `json:"round_id"`
	TermID          uint64                       `json:"term_id"`
	StudentID       uint64                       `json:"student_id"`
	CourseID        uint64                       `json:"course_id"`
	TeachingClassID uint64                       `json:"teaching_class_id"`
	Credits         enrollment.Credit            `json:"credits"`
	Source          enrollment.ApplicationSource `json:"source"`
	State           enrollment.ApplicationState  `json:"state"`
	Failure         *failureReasonPayload        `json:"failure,omitempty"`
	AppliedAt       time.Time                    `json:"applied_at"`
	CompletedAt     *time.Time                   `json:"completed_at,omitempty"`
}

func (p *selectionApplicationPayload) toEntity() *enrollment.SelectionApplication {
	application := &enrollment.SelectionApplication{
		ApplicationID:   p.ApplicationID,
		RequestID:       p.RequestID,
		RoundID:         p.RoundID,
		TermID:          p.TermID,
		StudentID:       p.StudentID,
		CourseID:        p.CourseID,
		TeachingClassID: p.TeachingClassID,
		Credits:         p.Credits,
		Source:          p.Source,
		State:           p.State,
		Failure:         p.Failure.toDomain(),
		AppliedAt:       p.AppliedAt,
		CompletedAt:     p.CompletedAt,
	}
	return application
}

// CommitSelection 以单次 Lua 原子完成额度和名额扣减、课程占用、最终结果保存以及
// Redis Stream 出站记录写入，避免在资源预占与结果落盘之间暴露中间状态。
func (r *SelectionStore) CommitSelection(
	ctx context.Context,
	result *enrollment.SelectionResult,
) (*applicationapi.SelectionResultPublication, error) {
	if err := result.Validate(); err != nil {
		return nil, err
	}
	if result.State != enrollment.ApplicationStateSelected {
		return nil, enrollment.ErrInvalidApplicationState
	}
	payload, err := json.Marshal(newSelectionResultPayload(result))
	if err != nil {
		return nil, fmt.Errorf("序列化选课结果: %w", err)
	}
	version, ready, err := r.activeVersion(ctx, result.RoundID)
	if err != nil {
		return nil, err
	}
	if !ready {
		return nil, enrollment.ErrRoundNotOpen
	}
	raw, err := r.redis.Eval(
		ctx,
		commitSelectionScript,
		[]string{
			activeEligibilityVersionKey(result.RoundID),
			roundOpenVersionKey(result.RoundID),
			roundSnapshotKey(result.RoundID, version),
			teachingClassSnapshotKey(result.RoundID, version, result.TeachingClassID),
			studentEligibilityKey(result.RoundID, version, result.StudentID),
			studentCreditKey(result.RoundID, result.StudentID),
			studentCourseQuotaKey(result.RoundID, result.StudentID),
			teachingClassSeatKey(result.TeachingClassID),
			pendingApplicationKey(result.RoundID, result.StudentID),
			requestResultKey(result.RoundID, result.StudentID, result.RequestID),
			studentCourseSelectionKey(result.RoundID, result.StudentID),
			applicationLookupKey(result.ApplicationID),
			selectionResultStreamKey,
			teachingClassScheduleKey(result.TeachingClassID),
			studentScheduleKey(result.RoundID, result.StudentID),
		},
		result.ApplicationID,
		result.RequestID,
		string(payload),
		int64(selectionRequestResultTTL/time.Second),
		version,
		result.CompletedAt.UnixMilli(),
		strconv.FormatUint(result.TeachingClassID, 10),
	)
	if err != nil {
		return nil, err
	}
	status, stored, err := parseScriptPair(raw)
	if err != nil {
		return nil, err
	}
	switch status {
	case 0, 1:
		publication, err := decodePublication(stored)
		if err != nil {
			return nil, err
		}
		if !sameSelectionFingerprint(publication.Result, result) {
			return nil, enrollment.ErrIdempotencyConflict
		}
		return publication, nil
	case 2:
		return nil, enrollment.ErrApplicationInProgress
	case -1, -8:
		return nil, enrollment.ErrRoundNotOpen
	case -2:
		return nil, enrollment.ErrCreditQuotaExceeded
	case -3:
		return nil, enrollment.ErrCourseQuotaExceeded
	case -4:
		return nil, enrollment.ErrDuplicateSelection
	case -5:
		return nil, enrollment.ErrTeachingClassFull
	case -7:
		return nil, enrollment.ErrScheduleConflict
	case -9:
		return nil, enrollment.ErrTeachingClassNotOpen
	case -10:
		return nil, enrollment.ErrEligibilityNotMet
	case -6:
		return nil, errors.New("Redis选课提交数据非法")
	default:
		return nil, fmt.Errorf("未知Redis选课提交状态: %d", status)
	}
}

func sameSelectionFingerprint(actual, expected *enrollment.SelectionResult) bool {
	return actual != nil && expected != nil &&
		actual.RequestID == expected.RequestID &&
		actual.RoundID == expected.RoundID &&
		actual.TermID == expected.TermID &&
		actual.StudentID == expected.StudentID &&
		actual.CourseID == expected.CourseID &&
		actual.TeachingClassID == expected.TeachingClassID &&
		actual.Credits == expected.Credits &&
		actual.Source == expected.Source
}

func (r *SelectionStore) activeVersion(ctx context.Context, roundID uint64) (string, bool, error) {
	result, err := r.redis.Eval(
		ctx,
		"return redis.call('GET', KEYS[1])",
		[]string{activeEligibilityVersionKey(roundID)},
	)
	if errors.Is(err, redislib.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	version, ok := result.(string)
	return version, ok && version != "", nil
}

func (r *QueryStore) querySelectionByRequestFromRedis(
	ctx context.Context,
	roundID uint64,
	studentID uint64,
	requestID string,
) (*applicationapi.SelectionRequestRecord, error) {
	raw, err := r.redis.Eval(
		ctx,
		querySelectionByRequestScript,
		[]string{
			requestResultKey(roundID, studentID, requestID),
			pendingApplicationKey(roundID, studentID),
		},
	)
	if err != nil {
		return nil, err
	}
	status, stored, err := parseScriptPair(raw)
	if err != nil {
		return nil, err
	}
	switch status {
	case 0:
		return nil, nil
	case 1:
		var payload selectionApplicationPayload
		if err := json.Unmarshal([]byte(stored), &payload); err != nil {
			return nil, fmt.Errorf("解析Redis选课pending: %w", err)
		}
		// pending 按学生和轮次串行化，可能属于另一笔请求；不同 request_id
		// 不能被误认为当前请求的幂等结果。
		if payload.RequestID != requestID {
			return nil, nil
		}
		return &applicationapi.SelectionRequestRecord{
			Application: payload.toEntity(),
		}, nil
	case 2:
		publication, err := decodePublication(stored)
		if err != nil {
			return nil, err
		}
		return &applicationapi.SelectionRequestRecord{
			Application: applicationFromResult(publication.Result),
			Publication: publication,
		}, nil
	default:
		return nil, fmt.Errorf("未知Redis选课幂等查询状态: %d", status)
	}
}

func (r *QueryStore) querySelectionApplicationFromRedis(
	ctx context.Context,
	applicationID string,
	studentID uint64,
) (*applicationapi.SelectionApplicationRecord, error) {
	var raw []byte
	err := r.redis.GetSkippingLocalCache(ctx, applicationLookupKey(applicationID), &raw)
	if err != nil {
		if errors.Is(err, cache.ErrCacheMiss) {
			return nil, nil
		}
		return nil, err
	}

	var publicationPayload selectionResultPublicationPayload
	if err := json.Unmarshal(raw, &publicationPayload); err == nil && publicationPayload.Result != nil {
		publication := publicationPayload.toApplication()
		if publication.Result.ApplicationID != applicationID ||
			publication.Result.StudentID != studentID {
			return nil, nil
		}
		if err := publication.Validate(); err != nil {
			return nil, err
		}
		return &applicationapi.SelectionApplicationRecord{
			Application:       applicationFromResult(publication.Result),
			DeliveryConfirmed: publication.DeliveryConfirmed,
			DurablyPersisted:  false,
		}, nil
	}

	var payload selectionApplicationPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("解析Redis申请状态: %w", err)
	}
	if payload.ApplicationID != applicationID || payload.StudentID != studentID {
		return nil, nil
	}
	return &applicationapi.SelectionApplicationRecord{
		Application: payload.toEntity(),
	}, nil
}

func (r *SelectionStore) QueryPendingSelectionResults(
	ctx context.Context,
	limit int64,
) ([]*applicationapi.SelectionResultPublication, error) {
	if limit <= 0 || limit > selectionResultRecoveryBatch {
		limit = selectionResultRecoveryBatch
	}
	raw, err := r.redis.Eval(
		ctx,
		querySelectionResultStreamScript,
		[]string{selectionResultStreamKey},
		limit,
	)
	if err != nil {
		return nil, err
	}
	entries, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("未知选课结果Stream响应: %#v", raw)
	}
	publications := make([]*applicationapi.SelectionResultPublication, 0, len(entries))
	for _, rawEntry := range entries {
		entry, ok := rawEntry.([]interface{})
		if !ok || len(entry) != 2 {
			return nil, fmt.Errorf("非法选课Stream记录: %#v", rawEntry)
		}
		streamID, ok := entry[0].(string)
		if !ok {
			return nil, fmt.Errorf("非法选课Stream ID: %#v", entry[0])
		}
		fields, ok := entry[1].([]interface{})
		if !ok || len(fields) != 2 || fields[0] != "event" {
			return nil, fmt.Errorf("非法选课Stream字段: %#v", entry[1])
		}
		eventJSON, ok := fields[1].(string)
		if !ok {
			return nil, fmt.Errorf("非法选课Stream载荷: %#v", fields[1])
		}
		var payload selectionResultPayload
		if err := json.Unmarshal([]byte(eventJSON), &payload); err != nil {
			return nil, fmt.Errorf("解析选课Stream结果: %w", err)
		}
		result := payload.toDomain()
		publication := &applicationapi.SelectionResultPublication{
			DeliveryCursor:    streamID,
			DeliveryConfirmed: false,
			Result:            result,
		}
		if err := publication.Validate(); err != nil {
			return nil, err
		}
		publications = append(publications, publication)
	}
	return publications, nil
}

func (r *SelectionStore) MarkSelectionResultPublished(
	ctx context.Context,
	publication *applicationapi.SelectionResultPublication,
) error {
	if err := publication.Validate(); err != nil {
		return err
	}
	result := publication.Result
	raw, err := r.redis.Eval(
		ctx,
		markSelectionResultPublishedScript,
		[]string{
			requestResultKey(result.RoundID, result.StudentID, result.RequestID),
			selectionResultStreamKey,
			applicationLookupKey(result.ApplicationID),
		},
		result.ApplicationID,
		publication.DeliveryCursor,
	)
	if err != nil {
		return err
	}
	status, ok := raw.(int64)
	if !ok || status < 0 {
		return fmt.Errorf("标记选课结果已发布失败: status=%v", raw)
	}
	publication.DeliveryConfirmed = true
	return nil
}

func (r *ResultStore) clearPersistedSelection(
	ctx context.Context,
	result *enrollment.SelectionResult,
) error {
	raw, err := r.redis.Eval(
		ctx,
		clearPersistedSelectionScript,
		[]string{pendingApplicationKey(result.RoundID, result.StudentID)},
		result.ApplicationID,
	)
	if err != nil {
		return err
	}
	status, ok := raw.(int64)
	if !ok {
		return fmt.Errorf("清理选课pending响应非法: %#v", raw)
	}
	// -1 表示该学生已经有更新的申请，不能清理新申请。
	if status == -1 {
		return nil
	}
	if status < -1 {
		return fmt.Errorf("清理选课pending失败: status=%d", status)
	}
	return nil
}

func (r *ProjectionStore) ReleaseDroppedEnrollment(
	ctx context.Context,
	target *enrollment.StudentEnrollment,
) error {
	if target == nil || target.State != enrollment.EnrollmentStateDropped ||
		target.DroppedAt == nil {
		return enrollment.ErrInvalidParams
	}
	raw, err := r.redis.Eval(
		ctx,
		releaseDroppedEnrollmentScript,
		[]string{
			droppedEnrollmentKey(target.EnrollmentID),
			studentCreditKey(target.RoundID, target.StudentID),
			studentCourseQuotaKey(target.RoundID, target.StudentID),
			teachingClassSeatKey(target.TeachingClassID),
			studentCourseSelectionKey(target.RoundID, target.StudentID),
			teachingClassScheduleKey(target.TeachingClassID),
			studentScheduleKey(target.RoundID, target.StudentID),
		},
		int64(target.Credits),
		target.ApplicationID,
		int64(30*24*time.Hour/time.Second),
		strconv.FormatUint(target.CourseID, 10),
	)
	if err != nil {
		return err
	}
	status, ok := raw.(int64)
	if !ok {
		return fmt.Errorf("退课Redis投影响应非法: %#v", raw)
	}
	if status < 0 {
		return fmt.Errorf("退课Redis投影失败: status=%d", status)
	}
	return nil
}

func parseScriptPair(raw interface{}) (int64, string, error) {
	values, ok := raw.([]interface{})
	if !ok || len(values) != 2 {
		return 0, "", fmt.Errorf("未知Redis Lua响应: %#v", raw)
	}
	status, ok := values[0].(int64)
	if !ok {
		return 0, "", fmt.Errorf("未知Redis Lua状态: %#v", values[0])
	}
	payload, _ := values[1].(string)
	return status, payload, nil
}

func decodePublication(raw string) (*applicationapi.SelectionResultPublication, error) {
	if raw == "" {
		return nil, enrollment.ErrRecordNotFound
	}
	var payload selectionResultPublicationPayload
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, fmt.Errorf("解析选课结果投递信封: %w", err)
	}
	publication := payload.toApplication()
	if err := publication.Validate(); err != nil {
		return nil, err
	}
	return publication, nil
}

func applicationFromResult(result *enrollment.SelectionResult) *enrollment.SelectionApplication {
	if result == nil {
		return nil
	}
	completedAt := result.CompletedAt
	return &enrollment.SelectionApplication{
		ApplicationID:   result.ApplicationID,
		RequestID:       result.RequestID,
		RoundID:         result.RoundID,
		TermID:          result.TermID,
		StudentID:       result.StudentID,
		CourseID:        result.CourseID,
		TeachingClassID: result.TeachingClassID,
		Credits:         result.Credits,
		Source:          result.Source,
		State:           result.State,
		Failure:         result.Failure,
		AppliedAt:       result.AppliedAt,
		CompletedAt:     &completedAt,
	}
}
