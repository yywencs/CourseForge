package enrollmenthttp

import (
	enrollmentapp "prizeforge/internal/enrollment/application"

	"github.com/gin-gonic/gin"
)

// Routes owns the enrollment context's inbound HTTP adapter.
type Routes struct {
	enrollmentUsecase *enrollmentapp.EnrollmentUsecase
	dropUsecase       *enrollmentapp.DropEnrollmentUsecase
	waitlistUsecase   *enrollmentapp.WaitlistUsecase
	authMiddleware    gin.HandlerFunc
}

func NewRoutes(
	enrollmentUsecase *enrollmentapp.EnrollmentUsecase,
	dropUsecase *enrollmentapp.DropEnrollmentUsecase,
	waitlistUsecase *enrollmentapp.WaitlistUsecase,
	authMiddleware gin.HandlerFunc,
) *Routes {
	return &Routes{
		enrollmentUsecase: enrollmentUsecase,
		dropUsecase:       dropUsecase,
		waitlistUsecase:   waitlistUsecase,
		authMiddleware:    authMiddleware,
	}
}

func (s *Routes) RegisterAPIRoutes(root *gin.RouterGroup) {
	group := root.Group("/enrollments")
	if s.authMiddleware != nil {
		group.Use(s.authMiddleware)
	}
	group.POST("", s.SelectCourse)
	group.GET("/applications/:application_id", s.QueryApplication)
	group.GET("/me", s.ListMyEnrollments)
	group.DELETE("/:enrollment_id", s.DropEnrollment)
	group.POST("/waitlist", s.JoinWaitlist)
	group.GET("/waitlist/me", s.ListMyWaitlist)
	group.GET("/waitlist/:waitlist_id", s.QueryWaitlist)
	group.DELETE("/waitlist/:waitlist_id", s.CancelWaitlist)
}
