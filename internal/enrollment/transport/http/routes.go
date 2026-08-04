package enrollmenthttp

import (
	enrollmentapp "github.com/yywencs/courseforge/internal/enrollment/application"

	"github.com/gin-gonic/gin"
)

// Routes owns the enrollment context's inbound HTTP adapter.
type Routes struct {
	enrollmentUsecase *enrollmentapp.EnrollmentUsecase
	dropUsecase       *enrollmentapp.DropEnrollmentUsecase
	waitlistUsecase   *enrollmentapp.WaitlistUsecase
	authMiddleware    gin.HandlerFunc
	selectionLimiter  gin.HandlerFunc
}

func NewRoutes(
	enrollmentUsecase *enrollmentapp.EnrollmentUsecase,
	dropUsecase *enrollmentapp.DropEnrollmentUsecase,
	waitlistUsecase *enrollmentapp.WaitlistUsecase,
	authMiddleware gin.HandlerFunc,
	selectionLimiters ...gin.HandlerFunc,
) *Routes {
	routes := &Routes{
		enrollmentUsecase: enrollmentUsecase,
		dropUsecase:       dropUsecase,
		waitlistUsecase:   waitlistUsecase,
		authMiddleware:    authMiddleware,
	}
	if len(selectionLimiters) > 0 {
		routes.selectionLimiter = selectionLimiters[0]
	}
	return routes
}

func (s *Routes) RegisterAPIRoutes(root *gin.RouterGroup) {
	group := root.Group("/enrollments")
	if s.authMiddleware != nil {
		group.Use(s.authMiddleware)
	}
	selectHandlers := make([]gin.HandlerFunc, 0, 2)
	if s.selectionLimiter != nil {
		selectHandlers = append(selectHandlers, s.selectionLimiter)
	}
	selectHandlers = append(selectHandlers, s.SelectCourse)
	group.POST("", selectHandlers...)
	group.GET("/applications/:application_id", s.QueryApplication)
	group.GET("/me", s.ListMyEnrollments)
	group.DELETE("/:enrollment_id", s.DropEnrollment)
	group.POST("/waitlist", s.JoinWaitlist)
	group.GET("/waitlist/me", s.ListMyWaitlist)
	group.GET("/waitlist/:waitlist_id", s.QueryWaitlist)
	group.DELETE("/waitlist/:waitlist_id", s.CancelWaitlist)
}
