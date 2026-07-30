package api

// registerRoutes registers CourseForge API routes.
func (s *Server) registerRoutes() {
	selectionGroup := s.engine.Group("/api/v1/enrollments")
	{
		selectionGroup.POST("", s.SelectCourse)
	}
}
