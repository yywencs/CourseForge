package api

// registerRoutes registers CourseForge API routes.
func (s *Server) registerRoutes() {
	group := s.engine.Group("/api/v1")
	for _, registrar := range s.registrars {
		if registrar != nil {
			registrar.RegisterAPIRoutes(group)
		}
	}
}
