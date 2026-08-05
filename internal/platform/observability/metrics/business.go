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

func IncRabbitMQConsume(topic, result string) {
	RabbitMQConsumeTotal.WithLabelValues(normalizeLabel(topic), normalizeLabel(result)).Inc()
}

// WebSocketConnectionOpened 记录一条连接成功注册到本机 Hub。
func WebSocketConnectionOpened() {
	WebSocketActiveConnections.Inc()
	WebSocketConnectionsTotal.WithLabelValues("connected").Inc()
}

// WebSocketConnectionClosed 记录一条已注册连接从本机 Hub 移除。
func WebSocketConnectionClosed() {
	WebSocketActiveConnections.Dec()
	WebSocketConnectionsTotal.WithLabelValues("closed").Inc()
}

// IncWebSocketBroadcastEvent 记录广播事件进入本机 Hub 的结果。
func IncWebSocketBroadcastEvent(result string) {
	WebSocketBroadcastEventsTotal.WithLabelValues(normalizeLabel(result)).Inc()
}

// IncWebSocketDelivery 记录 Hub 向单个客户端有界队列投递消息的结果。
func IncWebSocketDelivery(result string) {
	WebSocketDeliveriesTotal.WithLabelValues(normalizeLabel(result)).Inc()
}

// IncDanmakuSubscriberReconnect 记录 Redis 实时弹幕订阅的重连结果。
func IncDanmakuSubscriberReconnect(result string) {
	DanmakuSubscriberReconnectTotal.WithLabelValues(normalizeLabel(result)).Inc()
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
