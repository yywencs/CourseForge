package middleware

import (
	"github.com/yywencs/courseforge/internal/platform/observability/metrics"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// PrometheusMetrics 返回一个用于记录 HTTP 请求指标的 Gin 中间件。
func PrometheusMetrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isManagementEndpoint(c.Request.URL.Path) {
			c.Next()
			return
		}

		start := time.Now()

		// 处理请求
		c.Next()

		// 在响应完成后记录指标
		duration := time.Since(start)
		method := c.Request.Method
		path := c.FullPath()
		if path == "" {
			path = "unmatched"
		}
		code := strconv.Itoa(c.Writer.Status())

		metrics.IncHTTPRequest(method, path, code)
		metrics.ObserveHTTPDuration(method, path, duration)
	}
}

func isManagementEndpoint(path string) bool {
	switch path {
	case "/metrics", "/healthz", "/readyz":
		return true
	default:
		return false
	}
}
