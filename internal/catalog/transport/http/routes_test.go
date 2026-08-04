package cataloghttp

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestCatalogRoutesPreserveStudentAndAdminContracts(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	routes := NewCatalogRoutes(nil)
	routes.RegisterAPIRoutes(engine.Group("/api/v1"))
	routes.RegisterAdminRoutes(engine.Group("/admin/v1"))

	want := map[string]struct{}{
		http.MethodGet + " /api/v1/catalog/teaching-classes":                    {},
		http.MethodGet + " /admin/v1/courses":                                   {},
		http.MethodGet + " /admin/v1/courses/:course_id":                        {},
		http.MethodPost + " /admin/v1/courses":                                  {},
		http.MethodPut + " /admin/v1/courses/:course_id":                        {},
		http.MethodDelete + " /admin/v1/courses/:course_id":                     {},
		http.MethodGet + " /admin/v1/courses/:course_id/videos":                 {},
		http.MethodPost + " /admin/v1/courses/:course_id/videos/uploads":        {},
		http.MethodPost + " /admin/v1/course-video-uploads/:upload_id/complete": {},
		http.MethodGet + " /api/v1/course-videos/:video_id/playback":            {},
		http.MethodGet + " /admin/v1/teaching-classes":                          {},
		http.MethodGet + " /admin/v1/teaching-classes/:teaching_class_id":       {},
		http.MethodPost + " /admin/v1/teaching-classes":                         {},
		http.MethodPut + " /admin/v1/teaching-classes/:teaching_class_id":       {},
		http.MethodDelete + " /admin/v1/teaching-classes/:teaching_class_id":    {},
	}
	for _, route := range engine.Routes() {
		delete(want, route.Method+" "+route.Path)
	}
	if len(want) != 0 {
		t.Fatalf("missing catalog routes: %v", want)
	}
}
