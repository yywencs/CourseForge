package enrollmenthttp

import (
	"strconv"
	"time"

	applicationapi "github.com/yywencs/courseforge/internal/enrollment/application"
	"github.com/yywencs/courseforge/internal/enrollment/domain"
	"github.com/yywencs/courseforge/server/http/common"

	"github.com/gin-gonic/gin"
)

type joinWaitlistRequest struct {
	RequestID       string `json:"request_id"`
	RoundID         uint64 `json:"round_id"`
	TeachingClassID uint64 `json:"teaching_class_id"`
}

type waitlistResponse struct {
	WaitlistID      string                   `json:"waitlist_id"`
	RequestID       string                   `json:"request_id"`
	RoundID         uint64                   `json:"round_id"`
	TermID          uint64                   `json:"term_id"`
	CourseID        uint64                   `json:"course_id"`
	TeachingClassID uint64                   `json:"teaching_class_id"`
	Credits         string                   `json:"credits"`
	State           enrollment.WaitlistState `json:"state"`
	Failure         *failureReasonResponse   `json:"failure,omitempty"`
	Position        uint64                   `json:"position"`
	JoinedAt        time.Time                `json:"joined_at"`
	PromotedAt      *time.Time               `json:"promoted_at,omitempty"`
	CancelledAt     *time.Time               `json:"cancelled_at,omitempty"`
}

// JoinWaitlist 处理学生加入已满教学班候补队列的请求。
func (s *Routes) JoinWaitlist(c *gin.Context) {
	if s.waitlistUsecase == nil {
		common.Error(c, 503, "waitlist service is not configured")
		return
	}
	var req joinWaitlistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.Error(c, 400, "invalid request body: "+err.Error())
		return
	}
	studentID, ok := authenticatedStudentID(c)
	if !ok {
		return
	}
	entry, err := s.waitlistUsecase.Join(c.Request.Context(), &applicationapi.JoinWaitlistCommand{
		RequestID:       req.RequestID,
		RoundID:         req.RoundID,
		StudentID:       studentID,
		TeachingClassID: req.TeachingClassID,
	})
	if err != nil {
		handleSelectionError(c, studentID, req.RoundID, req.TeachingClassID, err)
		return
	}
	common.Success(c, toWaitlistResponse(entry))
}

func (s *Routes) QueryWaitlist(c *gin.Context) {
	studentID, ok := authenticatedStudentID(c)
	if !ok {
		return
	}
	entry, err := s.waitlistUsecase.Query(c.Request.Context(), studentID, c.Param("waitlist_id"))
	if err != nil {
		handleSelectionError(c, studentID, 0, 0, err)
		return
	}
	common.Success(c, toWaitlistResponse(entry))
}

func (s *Routes) ListMyWaitlist(c *gin.Context) {
	studentID, ok := authenticatedStudentID(c)
	if !ok {
		return
	}
	termID, err := strconv.ParseUint(c.Query("term_id"), 10, 64)
	if err != nil || termID == 0 {
		common.Error(c, 400, "invalid term_id")
		return
	}
	page, err := s.waitlistUsecase.List(
		c.Request.Context(),
		studentID,
		termID,
		queryIntOrDefault(c, "limit", 20),
		queryIntOrDefault(c, "offset", 0),
	)
	if err != nil {
		handleSelectionError(c, studentID, 0, 0, err)
		return
	}
	items := make([]waitlistResponse, 0, len(page.Items))
	for _, entry := range page.Items {
		items = append(items, toWaitlistResponse(entry))
	}
	common.Success(c, gin.H{
		"items":  items,
		"limit":  page.Limit,
		"offset": page.Offset,
		"total":  page.Total,
	})
}

func (s *Routes) CancelWaitlist(c *gin.Context) {
	studentID, ok := authenticatedStudentID(c)
	if !ok {
		return
	}
	entry, err := s.waitlistUsecase.Cancel(c.Request.Context(), studentID, c.Param("waitlist_id"))
	if err != nil {
		handleSelectionError(c, studentID, 0, 0, err)
		return
	}
	common.Success(c, toWaitlistResponse(entry))
}

func toWaitlistResponse(entry *enrollment.WaitlistEntry) waitlistResponse {
	return waitlistResponse{
		WaitlistID:      entry.WaitlistID,
		RequestID:       entry.RequestID,
		RoundID:         entry.RoundID,
		TermID:          entry.TermID,
		CourseID:        entry.CourseID,
		TeachingClassID: entry.TeachingClassID,
		Credits:         entry.Credits.String(),
		State:           entry.State,
		Failure:         toFailureReasonResponse(entry.Failure),
		Position:        entry.Position,
		JoinedAt:        entry.JoinedAt,
		PromotedAt:      entry.PromotedAt,
		CancelledAt:     entry.CancelledAt,
	}
}
