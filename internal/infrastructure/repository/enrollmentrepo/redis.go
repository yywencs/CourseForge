package enrollmentrepo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"prizeforge/internal/domain/enrollment"
	"prizeforge/pkg/cache"
)

const (
	selectionRequestResultTTL    = 7 * 24 * time.Hour
	selectionResultRecoveryBatch = 100
)

const initializeSelectionResourcesScript = `
	redis.call('SETNX', KEYS[1], ARGV[1])
	redis.call('SETNX', KEYS[2], ARGV[2])
	redis.call('SETNX', KEYS[3], ARGV[3])
	redis.call('PERSIST', KEYS[1])
	redis.call('PERSIST', KEYS[2])
	redis.call('PERSIST', KEYS[3])
	return 1
`

const reserveSelectionScript = `
	local credit_key = KEYS[1]
	local course_quota_key = KEYS[2]
	local seat_key = KEYS[3]
	local pending_key = KEYS[4]
		local result_key = KEYS[5]
		local course_guard_key = KEYS[6]
		local application_key = KEYS[7]

	local request_id = ARGV[1]
	local application_id = ARGV[2]
	local credits = tonumber(ARGV[3])
	local application_json = ARGV[4]

	local completed = redis.call('GET', result_key)
	if completed then
		return {3, completed}
	end

	local pending = redis.call('GET', pending_key)
	if pending then
		local ok, application = pcall(cjson.decode, pending)
		if not ok or not application.request_id or not application.application_id then
			return {-6, pending}
		end
		if application.request_id == request_id then
			return {1, pending}
		end
		return {2, pending}
	end

	if redis.call('EXISTS', course_guard_key) == 1 then
		return {-4, ''}
	end
	if redis.call('EXISTS', credit_key) == 0 or
	   redis.call('EXISTS', course_quota_key) == 0 or
	   redis.call('EXISTS', seat_key) == 0 then
		return {-1, ''}
	end

	local credit_remaining = tonumber(redis.call('GET', credit_key))
	local course_remaining = tonumber(redis.call('GET', course_quota_key))
	local seat_remaining = tonumber(redis.call('GET', seat_key))
	if not credit_remaining or not course_remaining or not seat_remaining or
	   not credits or credits <= 0 then
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

	local ok, application = pcall(cjson.decode, application_json)
	if not ok or application.request_id ~= request_id or
	   application.application_id ~= application_id then
		return {-6, ''}
	end

	application.state = 'reserved'
	local stored_application = cjson.encode(application)

	redis.call('DECRBY', credit_key, credits)
	redis.call('DECR', course_quota_key)
	redis.call('DECR', seat_key)
		redis.call('SET', course_guard_key, application_id)
		redis.call('SET', pending_key, stored_application)
		redis.call('SET', application_key, stored_application, 'EX', ARGV[5])
		return {0, stored_application}
`

const completeSelectionScript = `
	local pending = redis.call('GET', KEYS[1])
	if not pending then
		return {-1, ''}
	end
	local pending_ok, application = pcall(cjson.decode, pending)
	if not pending_ok or application.application_id ~= ARGV[1] or
	   application.request_id ~= ARGV[2] then
		return {-2, ''}
	end

	local existing = redis.call('GET', KEYS[2])
	if existing then
		return {1, existing}
	end
	if application.state ~= 'reserved' then
		return {-3, ''}
	end

	local result_ok, result = pcall(cjson.decode, ARGV[3])
	if not result_ok or result.application_id ~= ARGV[1] or
	   result.request_id ~= ARGV[2] then
		return {-4, ''}
	end
	if result.state ~= 'selected' and result.state ~= 'rejected' and
	   result.state ~= 'cancelled' then
		return {-4, ''}
	end

	if result.state == 'rejected' or result.state == 'cancelled' then
		local credits = tonumber(result.credits)
		if not credits or credits <= 0 then
			return {-4, ''}
		end
		redis.call('INCRBY', KEYS[4], credits)
		redis.call('INCR', KEYS[5])
		redis.call('INCR', KEYS[6])
		if redis.call('GET', KEYS[7]) == ARGV[1] then
			redis.call('DEL', KEYS[7])
		end
	end

	local stream_id = redis.call('XADD', KEYS[3], '*', 'event', ARGV[3])
	local publication = {
		stream_id = stream_id,
		broker_confirmed = false,
		result = result
	}
	local stored_publication = cjson.encode(publication)

	application.state = result.state
	application.failure = result.failure
	application.completed_at = result.completed_at
		redis.call('SET', KEYS[1], cjson.encode(application))
		redis.call('SET', KEYS[2], stored_publication, 'EX', ARGV[4])
		redis.call('SET', KEYS[8], stored_publication, 'EX', ARGV[4])
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
		   redis.call('EXISTS', KEYS[4]) == 0 then
			return -1
		end
		local credits = tonumber(ARGV[1])
		if not credits or credits <= 0 then
			return -2
		end
		redis.call('INCRBY', KEYS[2], credits)
		redis.call('INCR', KEYS[3])
		redis.call('INCR', KEYS[4])
		if redis.call('GET', KEYS[5]) == ARGV[2] then
			redis.call('DEL', KEYS[5])
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
	Failure         *enrollment.FailureReason    `json:"failure,omitempty"`
	AppliedAt       time.Time                    `json:"applied_at"`
	CompletedAt     *time.Time                   `json:"completed_at,omitempty"`
}

func newSelectionApplicationPayload(
	application *enrollment.SelectionApplication,
) *selectionApplicationPayload {
	payload := &selectionApplicationPayload{
		ApplicationID:   application.ApplicationID,
		RequestID:       application.RequestID,
		RoundID:         application.RoundID,
		TermID:          application.TermID,
		StudentID:       application.StudentID,
		CourseID:        application.CourseID,
		TeachingClassID: application.TeachingClassID,
		Credits:         application.Credits,
		Source:          application.Source,
		State:           application.State,
		Failure:         application.Failure,
		AppliedAt:       application.AppliedAt,
		CompletedAt:     application.CompletedAt,
	}
	return payload
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
		Failure:         p.Failure,
		AppliedAt:       p.AppliedAt,
		CompletedAt:     p.CompletedAt,
	}
	return application
}

func (r *Repository) ReserveSelection(
	ctx context.Context,
	application *enrollment.SelectionApplication,
) (*enrollment.SelectionReservation, error) {
	if application == nil || application.State != enrollment.ApplicationStateCreated {
		return nil, enrollment.ErrInvalidParams
	}
	payload, err := json.Marshal(newSelectionApplicationPayload(application))
	if err != nil {
		return nil, fmt.Errorf("序列化选课申请: %w", err)
	}
	keys := []string{
		studentCreditKey(application.RoundID, application.StudentID),
		studentCourseQuotaKey(application.RoundID, application.StudentID),
		teachingClassSeatKey(application.TeachingClassID),
		pendingApplicationKey(application.RoundID, application.StudentID),
		requestResultKey(application.RoundID, application.StudentID, application.RequestID),
		selectedCourseGuardKey(application.TermID, application.StudentID, application.CourseID),
		applicationLookupKey(application.ApplicationID),
	}

	for attempt := 0; attempt < 2; attempt++ {
		raw, err := r.redis.Eval(
			ctx,
			reserveSelectionScript,
			keys,
			application.RequestID,
			application.ApplicationID,
			int64(application.Credits),
			string(payload),
			int64(selectionRequestResultTTL/time.Second),
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
			var decoded selectionApplicationPayload
			if err := json.Unmarshal([]byte(stored), &decoded); err != nil {
				return nil, fmt.Errorf("解析Redis选课申请: %w", err)
			}
			reservationStatus := enrollment.ReservationStatusAcquired
			if status == 1 {
				reservationStatus = enrollment.ReservationStatusReused
			}
			return &enrollment.SelectionReservation{
				Status:      reservationStatus,
				Application: decoded.toEntity(),
			}, nil
		case 3:
			publication, err := decodePublication(stored)
			if err != nil {
				return nil, err
			}
			return &enrollment.SelectionReservation{
				Status:      enrollment.ReservationStatusCompleted,
				Application: applicationFromResult(publication.Result),
				Publication: publication,
			}, nil
		case 2:
			return nil, enrollment.ErrApplicationInProgress
		case -1:
			if attempt == 1 {
				return nil, errors.New("选课Redis额度或名额未初始化")
			}
			if err := r.initializeSelectionResources(ctx, application); err != nil {
				return nil, err
			}
		case -2:
			return nil, enrollment.ErrCreditQuotaExceeded
		case -3:
			return nil, enrollment.ErrCourseQuotaExceeded
		case -4:
			return nil, enrollment.ErrDuplicateSelection
		case -5:
			return nil, enrollment.ErrTeachingClassFull
		case -6:
			return nil, errors.New("Redis选课申请数据非法")
		default:
			return nil, fmt.Errorf("未知Redis选课预占状态: %d", status)
		}
	}
	return nil, errors.New("选课资源初始化重试耗尽")
}

func (r *Repository) querySelectionByRequestFromRedis(
	ctx context.Context,
	roundID uint64,
	studentID uint64,
	requestID string,
) (*enrollment.SelectionRequestRecord, error) {
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
		// pending 按学生和轮次串行化，可能属于另一笔请求；这种情况继续回退 MySQL
		// 查询当前 request_id，而不是把不同幂等键误判为冲突。
		if payload.RequestID != requestID {
			return nil, nil
		}
		return &enrollment.SelectionRequestRecord{
			Application: payload.toEntity(),
		}, nil
	case 2:
		publication, err := decodePublication(stored)
		if err != nil {
			return nil, err
		}
		return &enrollment.SelectionRequestRecord{
			Application: applicationFromResult(publication.Result),
			Publication: publication,
		}, nil
	default:
		return nil, fmt.Errorf("未知Redis选课幂等查询状态: %d", status)
	}
}

func (r *Repository) querySelectionApplicationFromRedis(
	ctx context.Context,
	applicationID string,
	studentID uint64,
) (*enrollment.SelectionApplicationRecord, error) {
	var raw []byte
	err := r.redis.GetSkippingLocalCache(ctx, applicationLookupKey(applicationID), &raw)
	if err != nil {
		if errors.Is(err, cache.ErrCacheMiss) {
			return nil, nil
		}
		return nil, err
	}

	var publication enrollment.SelectionResultPublication
	if err := json.Unmarshal(raw, &publication); err == nil && publication.Result != nil {
		if publication.Result.ApplicationID != applicationID ||
			publication.Result.StudentID != studentID {
			return nil, nil
		}
		if err := publication.Validate(); err != nil {
			return nil, err
		}
		return &enrollment.SelectionApplicationRecord{
			Application:     applicationFromResult(publication.Result),
			BrokerConfirmed: publication.BrokerConfirmed,
			MySQLPersisted:  false,
		}, nil
	}

	var payload selectionApplicationPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("解析Redis申请状态: %w", err)
	}
	if payload.ApplicationID != applicationID || payload.StudentID != studentID {
		return nil, nil
	}
	return &enrollment.SelectionApplicationRecord{
		Application: payload.toEntity(),
	}, nil
}

func (r *Repository) initializeSelectionResources(
	ctx context.Context,
	application *enrollment.SelectionApplication,
) error {
	exists, err := r.HasExistingEnrollment(
		ctx,
		application.TermID,
		application.StudentID,
		application.CourseID,
	)
	if err != nil {
		return err
	}
	if exists {
		return enrollment.ErrDuplicateSelection
	}
	quota, err := r.QueryStudentSelectionQuota(ctx, application.RoundID, application.StudentID)
	if err != nil {
		return err
	}
	if quota == nil {
		return enrollment.ErrRecordNotFound
	}
	class, err := r.QueryTeachingClass(ctx, application.RoundID, application.TeachingClassID)
	if err != nil {
		return err
	}
	if class == nil {
		return enrollment.ErrRecordNotFound
	}
	creditRemaining := quota.CreditLimit - quota.SelectedCredits
	courseRemaining := int64(quota.CourseLimit) - int64(quota.SelectedCourseCount)
	seatRemaining := int64(class.Capacity) - int64(class.SelectedCount)
	if creditRemaining < 0 || courseRemaining < 0 || seatRemaining < 0 {
		return errors.New("MySQL选课额度或名额快照非法")
	}
	_, err = r.redis.Eval(
		ctx,
		initializeSelectionResourcesScript,
		[]string{
			studentCreditKey(application.RoundID, application.StudentID),
			studentCourseQuotaKey(application.RoundID, application.StudentID),
			teachingClassSeatKey(application.TeachingClassID),
		},
		int64(creditRemaining),
		courseRemaining,
		seatRemaining,
	)
	return err
}

func (r *Repository) CompleteSelection(
	ctx context.Context,
	result *enrollment.SelectionResult,
) (*enrollment.SelectionResultPublication, error) {
	if err := result.Validate(); err != nil {
		return nil, err
	}
	payload, err := json.Marshal(result)
	if err != nil {
		return nil, fmt.Errorf("序列化选课结果: %w", err)
	}
	raw, err := r.redis.Eval(
		ctx,
		completeSelectionScript,
		[]string{
			pendingApplicationKey(result.RoundID, result.StudentID),
			requestResultKey(result.RoundID, result.StudentID, result.RequestID),
			selectionResultStreamKey,
			studentCreditKey(result.RoundID, result.StudentID),
			studentCourseQuotaKey(result.RoundID, result.StudentID),
			teachingClassSeatKey(result.TeachingClassID),
			selectedCourseGuardKey(result.TermID, result.StudentID, result.CourseID),
			applicationLookupKey(result.ApplicationID),
		},
		result.ApplicationID,
		result.RequestID,
		string(payload),
		int64(selectionRequestResultTTL/time.Second),
	)
	if err != nil {
		return nil, err
	}
	status, stored, err := parseScriptPair(raw)
	if err != nil {
		return nil, err
	}
	if status < 0 {
		return nil, fmt.Errorf("完成选课申请被拒绝: status=%d", status)
	}
	return decodePublication(stored)
}

func (r *Repository) QueryPendingSelectionResults(
	ctx context.Context,
	limit int64,
) ([]*enrollment.SelectionResultPublication, error) {
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
	publications := make([]*enrollment.SelectionResultPublication, 0, len(entries))
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
		var result enrollment.SelectionResult
		if err := json.Unmarshal([]byte(eventJSON), &result); err != nil {
			return nil, fmt.Errorf("解析选课Stream结果: %w", err)
		}
		publication := &enrollment.SelectionResultPublication{
			StreamID:        streamID,
			BrokerConfirmed: false,
			Result:          &result,
		}
		if err := publication.Validate(); err != nil {
			return nil, err
		}
		publications = append(publications, publication)
	}
	return publications, nil
}

func (r *Repository) MarkSelectionResultPublished(
	ctx context.Context,
	publication *enrollment.SelectionResultPublication,
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
		publication.StreamID,
	)
	if err != nil {
		return err
	}
	status, ok := raw.(int64)
	if !ok || status < 0 {
		return fmt.Errorf("标记选课结果已发布失败: status=%v", raw)
	}
	publication.BrokerConfirmed = true
	return nil
}

func (r *Repository) clearPersistedSelection(
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

func (r *Repository) ReleaseDroppedEnrollment(
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
			selectedCourseGuardKey(target.TermID, target.StudentID, target.CourseID),
		},
		int64(target.Credits),
		target.ApplicationID,
		int64(30*24*time.Hour/time.Second),
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

func decodePublication(raw string) (*enrollment.SelectionResultPublication, error) {
	if raw == "" {
		return nil, enrollment.ErrRecordNotFound
	}
	var publication enrollment.SelectionResultPublication
	if err := json.Unmarshal([]byte(raw), &publication); err != nil {
		return nil, fmt.Errorf("解析选课结果投递信封: %w", err)
	}
	if err := publication.Validate(); err != nil {
		return nil, err
	}
	return &publication, nil
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
