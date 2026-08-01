package metrics

import (
	"database/sql"
	"time"
)

func ObserveSelection(result string, duration time.Duration) {
	result = normalizeLabel(result)
	SelectionTotal.WithLabelValues(result).Inc()
	SelectionDuration.WithLabelValues(result).Observe(duration.Seconds())
}

func ObserveSelectionPersistence(result string, duration time.Duration) {
	result = normalizeLabel(result)
	SelectionPersistenceTotal.WithLabelValues(result).Inc()
	SelectionPersistenceDuration.WithLabelValues(result).Observe(duration.Seconds())
}

func IncEnrollmentProjection(operation, result string) {
	EnrollmentProjectionTotal.WithLabelValues(
		normalizeLabel(operation),
		normalizeLabel(result),
	).Inc()
}

// IncWaitlistPromotion 记录候补晋级结果。
func IncWaitlistPromotion(result string) {
	WaitlistPromotionTotal.WithLabelValues(normalizeLabel(result)).Inc()
}

// SetProjectionRepairPending 设置等待执行的投影修复任务数。
func SetProjectionRepairPending(count int64) {
	ProjectionRepairPending.Set(float64(count))
}

func ObserveOutboxDispatch(topic, result string, duration time.Duration) {
	topic = normalizeLabel(topic)
	result = normalizeLabel(result)
	OutboxDispatchTotal.WithLabelValues(topic, result).Inc()
	OutboxDispatchDuration.WithLabelValues(topic, result).Observe(duration.Seconds())
}

func IncRabbitMQPublish(topic, result string) {
	RabbitMQPublishTotal.WithLabelValues(normalizeLabel(topic), normalizeLabel(result)).Inc()
}

func IncRedisOp(cmd, result string) {
	RedisOpsTotal.WithLabelValues(normalizeLabel(cmd), normalizeLabel(result)).Inc()
}

func ObserveRedisOpDuration(cmd string, duration time.Duration) {
	RedisOpDuration.WithLabelValues(normalizeLabel(cmd)).Observe(duration.Seconds())
}

func IncAsynqTask(taskType, result string) {
	AsynqTaskTotal.WithLabelValues(normalizeLabel(taskType), normalizeLabel(result)).Inc()
}

func ObserveAsynqTaskDuration(taskType string, duration time.Duration) {
	AsynqTaskDuration.WithLabelValues(normalizeLabel(taskType)).Observe(duration.Seconds())
}

func SetAsynqQueueStats(queue string, size, retry, scheduled int) {
	queue = normalizeLabel(queue)
	AsynqQueueSize.WithLabelValues(queue).Set(float64(size))
	AsynqRetrySize.WithLabelValues(queue).Set(float64(retry))
	AsynqScheduledSize.WithLabelValues(queue).Set(float64(scheduled))
}

func SetMySQLStats(dbName, role string, stats sql.DBStats) {
	dbName = normalizeLabel(dbName)
	role = normalizeLabel(role)

	MySQLOpenConnections.WithLabelValues(dbName, role).Set(float64(stats.OpenConnections))
	MySQLInUse.WithLabelValues(dbName, role).Set(float64(stats.InUse))
	MySQLIdle.WithLabelValues(dbName, role).Set(float64(stats.Idle))
	MySQLWaitCount.WithLabelValues(dbName, role).Set(float64(stats.WaitCount))
	MySQLWaitDurationSeconds.WithLabelValues(dbName, role).Set(stats.WaitDuration.Seconds())
	MySQLMaxIdleClosedTotal.WithLabelValues(dbName, role).Set(float64(stats.MaxIdleClosed))
	MySQLMaxLifetimeClosedTotal.WithLabelValues(dbName, role).Set(float64(stats.MaxLifetimeClosed))
}

func normalizeLabel(v string) string {
	if v == "" {
		return "unknown"
	}
	return v
}

// IncHTTPRequest increments the HTTP request counter.
func IncHTTPRequest(method, path, code string) {
	HTTPRequestsTotal.WithLabelValues(method, path, code).Inc()
}

// ObserveHTTPDuration records HTTP request latency.
func ObserveHTTPDuration(method, path string, duration time.Duration) {
	HTTPRequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}
