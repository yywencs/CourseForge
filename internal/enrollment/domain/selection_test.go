package enrollment

import (
	"errors"
	"testing"
	"time"
)

func validSelectionRequest() *SelectionRequest {
	return &SelectionRequest{
		RequestID:       "request-001",
		RoundID:         101,
		TermID:          202601,
		StudentID:       10001,
		CourseID:        20001,
		TeachingClassID: 30001,
		Credits:         Credit(35),
		Source:          ApplicationSourceWeb,
	}
}

// TestCreditValueObject 验证学分使用十分之一为单位，避免后续额度扣减依赖浮点数。
func TestCreditValueObject(t *testing.T) {
	tests := []struct {
		tenths int32
		want   string
	}{
		{tenths: 10, want: "1"},
		{tenths: 25, want: "2.5"},
		{tenths: 35, want: "3.5"},
	}

	for _, tt := range tests {
		credit, err := NewCreditFromTenths(tt.tenths)
		if err != nil {
			t.Fatalf("NewCreditFromTenths(%d) error = %v", tt.tenths, err)
		}
		if got := credit.String(); got != tt.want {
			t.Fatalf("Credit(%d).String() = %q, want %q", tt.tenths, got, tt.want)
		}
	}

	if _, err := NewCreditFromTenths(0); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("NewCreditFromTenths(0) error = %v, want %v", err, ErrInvalidParams)
	}
}

// TestSelectionRequestValidate 验证主链路依赖的身份、业务ID、学分和来源字段不能为空。
func TestSelectionRequestValidate(t *testing.T) {
	valid := validSelectionRequest()
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid request error = %v, want nil", err)
	}

	tests := []struct {
		name   string
		mutate func(*SelectionRequest)
	}{
		{name: "empty request ID", mutate: func(r *SelectionRequest) { r.RequestID = "" }},
		{name: "empty round ID", mutate: func(r *SelectionRequest) { r.RoundID = 0 }},
		{name: "empty term ID", mutate: func(r *SelectionRequest) { r.TermID = 0 }},
		{name: "empty student ID", mutate: func(r *SelectionRequest) { r.StudentID = 0 }},
		{name: "empty course ID", mutate: func(r *SelectionRequest) { r.CourseID = 0 }},
		{name: "empty teaching class ID", mutate: func(r *SelectionRequest) { r.TeachingClassID = 0 }},
		{name: "invalid credits", mutate: func(r *SelectionRequest) { r.Credits = 0 }},
		{name: "invalid source", mutate: func(r *SelectionRequest) { r.Source = "unknown" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := *valid
			tt.mutate(&request)
			if err := request.Validate(); !errors.Is(err, ErrInvalidParams) {
				t.Fatalf("Validate() error = %v, want %v", err, ErrInvalidParams)
			}
		})
	}
}

// TestSelectionRoundAcceptingAt 验证轮次开始时间包含、结束时间不包含的边界语义。
func TestSelectionRoundAcceptingAt(t *testing.T) {
	start := time.Date(2026, time.September, 1, 8, 0, 0, 0, time.Local)
	round := &SelectionRound{
		ID:        1,
		TermID:    2,
		StartTime: start,
		EndTime:   start.Add(time.Hour),
		State:     SelectionRoundStateOpen,
	}

	if !round.AcceptingAt(start) {
		t.Fatal("round should accept at start time")
	}
	if !round.AcceptingAt(start.Add(time.Minute)) {
		t.Fatal("round should accept inside open interval")
	}
	if round.AcceptingAt(round.EndTime) {
		t.Fatal("round should not accept at end time")
	}
	round.State = SelectionRoundStateClosed
	if round.AcceptingAt(start.Add(time.Minute)) {
		t.Fatal("closed round should not accept selections")
	}
}

// TestTeachingClassValidateForSelection 验证教学班身份、状态和容量检查。
func TestTeachingClassValidateForSelection(t *testing.T) {
	request := validSelectionRequest()
	class := &TeachingClass{
		ID:            request.TeachingClassID,
		TermID:        request.TermID,
		CourseID:      request.CourseID,
		Credits:       request.Credits,
		Capacity:      100,
		SelectedCount: 99,
		State:         TeachingClassStateOpen,
	}

	if err := class.validateForSelection(request); err != nil {
		t.Fatalf("validateForSelection() error = %v, want nil", err)
	}

	class.SelectedCount = 100
	if err := class.validateForSelection(request); !errors.Is(err, ErrTeachingClassFull) {
		t.Fatalf("full class error = %v, want %v", err, ErrTeachingClassFull)
	}

	class.SelectedCount = 99
	class.State = TeachingClassStateClosed
	if err := class.validateForSelection(request); !errors.Is(err, ErrTeachingClassNotOpen) {
		t.Fatalf("closed class error = %v, want %v", err, ErrTeachingClassNotOpen)
	}

	class.State = TeachingClassStateOpen
	class.CourseID++
	if err := class.validateForSelection(request); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("mismatched class error = %v, want %v", err, ErrInvalidParams)
	}
}

// TestStudentSelectionQuotaValidateReservation 验证学分和课程门数两个额度维度。
func TestStudentSelectionQuotaValidateReservation(t *testing.T) {
	request := validSelectionRequest()
	quota := &StudentSelectionQuota{
		RoundID:             request.RoundID,
		TermID:              request.TermID,
		StudentID:           request.StudentID,
		CreditLimit:         Credit(200),
		SelectedCredits:     Credit(165),
		CourseLimit:         6,
		SelectedCourseCount: 5,
	}

	if err := quota.validateReservation(request); err != nil {
		t.Fatalf("validateReservation() error = %v, want nil", err)
	}

	quota.SelectedCredits = Credit(166)
	if err := quota.validateReservation(request); !errors.Is(err, ErrCreditQuotaExceeded) {
		t.Fatalf("credit quota error = %v, want %v", err, ErrCreditQuotaExceeded)
	}

	quota.SelectedCredits = Credit(100)
	quota.SelectedCourseCount = quota.CourseLimit
	if err := quota.validateReservation(request); !errors.Is(err, ErrCourseQuotaExceeded) {
		t.Fatalf("course quota error = %v, want %v", err, ErrCourseQuotaExceeded)
	}
}

// TestSelectionResultValidate 验证成功结果不能携带失败原因，失败结果必须携带原因。
func TestSelectionResultValidate(t *testing.T) {
	appliedAt := time.Date(2026, time.September, 1, 8, 0, 0, 0, time.Local)
	result := &SelectionResult{
		ApplicationID:   "application-001",
		RequestID:       "request-001",
		RoundID:         101,
		TermID:          202601,
		StudentID:       10001,
		CourseID:        20001,
		TeachingClassID: 30001,
		Credits:         Credit(35),
		Source:          ApplicationSourceWeb,
		State:           ApplicationStateSelected,
		AppliedAt:       appliedAt,
		CompletedAt:     appliedAt.Add(time.Second),
	}
	if err := result.Validate(); err != nil {
		t.Fatalf("selected result error = %v, want nil", err)
	}

	result.Failure = &FailureReason{Code: FailureCodeInternal, Message: "unexpected"}
	if err := result.Validate(); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("selected result with failure error = %v, want %v", err, ErrInvalidParams)
	}

	result.State = ApplicationStateRejected
	result.Failure = nil
	if err := result.Validate(); !errors.Is(err, ErrInvalidParams) {
		t.Fatalf("rejected result without failure error = %v, want %v", err, ErrInvalidParams)
	}
}
