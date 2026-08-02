package common

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestMetricsRouteExposesPrometheusMetrics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	RegisterMetricsRoute(engine)

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	engine.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}
	if !strings.Contains(response.Body.String(), "go_goroutines") {
		t.Fatal("metrics response does not contain Go runtime metrics")
	}
}
