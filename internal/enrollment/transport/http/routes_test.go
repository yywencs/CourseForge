package enrollmenthttp

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestEnrollmentRoutesPreserveAPIContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewRoutes(nil, nil, nil, nil).RegisterAPIRoutes(engine.Group("/api/v1"))

	want := map[string]string{
		http.MethodPost + " /api/v1/enrollments":                             "",
		http.MethodGet + " /api/v1/enrollments/applications/:application_id": "",
		http.MethodGet + " /api/v1/enrollments/me":                           "",
		http.MethodDelete + " /api/v1/enrollments/:enrollment_id":            "",
		http.MethodPost + " /api/v1/enrollments/waitlist":                    "",
		http.MethodGet + " /api/v1/enrollments/waitlist/me":                  "",
		http.MethodGet + " /api/v1/enrollments/waitlist/:waitlist_id":        "",
		http.MethodDelete + " /api/v1/enrollments/waitlist/:waitlist_id":     "",
	}
	for _, route := range engine.Routes() {
		delete(want, route.Method+" "+route.Path)
	}
	if len(want) != 0 {
		t.Fatalf("missing enrollment routes: %v", want)
	}
}

func TestRoundAdminRoutesPreserveAPIContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	NewRoundAdminRoutes(nil).RegisterAdminRoutes(engine.Group("/admin/v1"))

	want := map[string]struct{}{
		http.MethodGet + " /admin/v1/selection-rounds":                                                  {},
		http.MethodPost + " /admin/v1/selection-rounds":                                                 {},
		http.MethodPut + " /admin/v1/selection-rounds/:round_id":                                        {},
		http.MethodDelete + " /admin/v1/selection-rounds/:round_id":                                     {},
		http.MethodGet + " /admin/v1/selection-rounds/:round_id/teaching-classes":                       {},
		http.MethodPost + " /admin/v1/selection-rounds/:round_id/teaching-classes/:teaching_class_id":   {},
		http.MethodDelete + " /admin/v1/selection-rounds/:round_id/teaching-classes/:teaching_class_id": {},
	}
	for _, route := range engine.Routes() {
		delete(want, route.Method+" "+route.Path)
	}
	if len(want) != 0 {
		t.Fatalf("missing round admin routes: %v", want)
	}
}
