package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"prizeforge/internal/platform/observability/metrics"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

func TestPrometheusMetricsNormalizesUnmatchedPaths(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.Use(PrometheusMetrics())

	metrics.HTTPRequestsTotal.WithLabelValues(http.MethodGet, "unmatched", "404")
	before := httpRequestCounterValue(t, http.MethodGet, "unmatched", "404")

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/random/10001", nil)
	engine.ServeHTTP(response, request)

	if got := httpRequestCounterValue(t, http.MethodGet, "unmatched", "404"); got != before+1 {
		t.Fatalf("unmatched request count = %v, want %v", got, before+1)
	}
}

func TestPrometheusMetricsSkipsManagementEndpoints(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, path := range []string{"/metrics", "/healthz", "/readyz"} {
		t.Run(path, func(t *testing.T) {
			engine := gin.New()
			engine.Use(PrometheusMetrics())
			engine.GET(path, func(c *gin.Context) { c.Status(http.StatusOK) })

			metrics.HTTPRequestsTotal.WithLabelValues(http.MethodGet, path, "200")
			before := httpRequestCounterValue(t, http.MethodGet, path, "200")
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodGet, path, nil)
			engine.ServeHTTP(response, request)

			if got := httpRequestCounterValue(t, http.MethodGet, path, "200"); got != before {
				t.Fatalf("management request count = %v, want %v", got, before)
			}
		})
	}
}

func httpRequestCounterValue(t *testing.T, method, path, code string) float64 {
	t.Helper()
	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() != "courseforge_http_requests_total" {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := make(map[string]string, len(metric.GetLabel()))
			for _, label := range metric.GetLabel() {
				labels[label.GetName()] = label.GetValue()
			}
			if labels["method"] == method && labels["path"] == path && labels["code"] == code {
				return metric.GetCounter().GetValue()
			}
		}
	}
	t.Fatalf("HTTP request metric not found for method=%s path=%s code=%s", method, path, code)
	return 0
}
