package api

// registerRoutes registers CourseForge API routes.
func (s *Server) registerRoutes() {
	selectionGroup := s.engine.Group("/api/v1/enrollments")
	if s.authMiddleware != nil {
		selectionGroup.Use(s.authMiddleware)
	}
	{
		selectionGroup.POST("", s.SelectCourse)
		selectionGroup.GET("/applications/:application_id", s.QueryApplication)
		selectionGroup.GET("/me", s.ListMyEnrollments)
		selectionGroup.DELETE("/:enrollment_id", s.DropEnrollment)
		selectionGroup.POST("/waitlist", s.JoinWaitlist)
		selectionGroup.GET("/waitlist/me", s.ListMyWaitlist)
		selectionGroup.GET("/waitlist/:waitlist_id", s.QueryWaitlist)
		selectionGroup.DELETE("/waitlist/:waitlist_id", s.CancelWaitlist)
	}
}
