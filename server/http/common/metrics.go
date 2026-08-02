package common

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// RegisterMetricsRoute 注册供 Prometheus 通过内部网络抓取的指标端点。
func RegisterMetricsRoute(engine *gin.Engine) {
	engine.GET("/metrics", gin.WrapH(promhttp.Handler()))
}
