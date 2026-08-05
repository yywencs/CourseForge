package metrics

import "github.com/prometheus/client_golang/prometheus"

const metricNamespace = "courseforge"

var (
	HTTPRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: "http",
			Name:      "requests_total",
			Help:      "Total number of HTTP requests.",
		},
		[]string{"method", "path", "code"},
	)

	HTTPRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Subsystem: "http",
			Name:      "request_duration_seconds",
			Help:      "HTTP request latency.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)

	SelectionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: "selection",
			Name:      "requests_total",
			Help:      "Total number of course selection decisions.",
		},
		[]string{"result"},
	)

	SelectionDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Subsystem: "selection",
			Name:      "duration_seconds",
			Help:      "Course selection decision latency.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"result"},
	)

	SelectionPersistenceTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: "selection",
			Name:      "persistence_total",
			Help:      "Total number of selection result persistence attempts.",
		},
		[]string{"result"},
	)

	SelectionPersistenceDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Subsystem: "selection",
			Name:      "persistence_duration_seconds",
			Help:      "Selection result MySQL persistence latency.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"result"},
	)

	EnrollmentProjectionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: "enrollment",
			Name:      "projection_total",
			Help:      "Total enrollment Redis projection updates.",
		},
		[]string{"operation", "result"},
	)

	WaitlistPromotionTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: "waitlist",
			Name:      "promotion_total",
			Help:      "Total number of waitlist promotion results.",
		},
		[]string{"result"},
	)

	ProjectionRepairPending = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: "enrollment",
			Name:      "projection_repair_pending",
			Help:      "Current pending MySQL to Redis projection repairs.",
		},
	)

	RabbitMQPublishTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: "rabbitmq",
			Name:      "publish_total",
			Help:      "Total RabbitMQ publish results.",
		},
		[]string{"topic", "result"},
	)

	RabbitMQConsumeTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: "rabbitmq",
			Name:      "consume_total",
			Help:      "Total RabbitMQ consumer outcomes.",
		},
		[]string{"topic", "result"},
	)

	WebSocketActiveConnections = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: "websocket",
			Name:      "active_connections",
			Help:      "Current number of active WebSocket connections.",
		},
	)

	WebSocketConnectionsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: "websocket",
			Name:      "connections_total",
			Help:      "Total WebSocket connection lifecycle events.",
		},
		[]string{"event"},
	)

	WebSocketBroadcastEventsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: "websocket",
			Name:      "broadcast_events_total",
			Help:      "Total WebSocket broadcast events submitted to the local hub.",
		},
		[]string{"result"},
	)

	WebSocketDeliveriesTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: "websocket",
			Name:      "deliveries_total",
			Help:      "Total WebSocket payload delivery attempts to client queues.",
		},
		[]string{"result"},
	)

	DanmakuSubscriberReconnectTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: "danmaku",
			Name:      "subscriber_reconnect_total",
			Help:      "Total Redis danmaku subscriber reconnect outcomes.",
		},
		[]string{"result"},
	)

	OutboxDispatchTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: "outbox",
			Name:      "dispatch_total",
			Help:      "Total number of generic outbox dispatch attempts.",
		},
		[]string{"topic", "result"},
	)

	OutboxDispatchDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Subsystem: "outbox",
			Name:      "dispatch_duration_seconds",
			Help:      "Generic outbox dispatch latency.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"topic", "result"},
	)

	RedisOpsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: "redis",
			Name:      "ops_total",
			Help:      "Total Redis operations.",
		},
		[]string{"cmd", "result"},
	)

	RedisOpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Subsystem: "redis",
			Name:      "op_duration_seconds",
			Help:      "Redis operation latency.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"cmd"},
	)

	AsynqTaskTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: metricNamespace,
			Subsystem: "asynq",
			Name:      "task_total",
			Help:      "Total Asynq task execution results.",
		},
		[]string{"task_type", "result"},
	)

	AsynqTaskDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: metricNamespace,
			Subsystem: "asynq",
			Name:      "task_duration_seconds",
			Help:      "Asynq task execution latency.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"task_type"},
	)

	AsynqQueueSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: "asynq",
			Name:      "queue_size",
			Help:      "Current total Asynq queue size.",
		},
		[]string{"queue"},
	)

	AsynqRetrySize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: "asynq",
			Name:      "retry_size",
			Help:      "Current Asynq retry queue size.",
		},
		[]string{"queue"},
	)

	AsynqScheduledSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: "asynq",
			Name:      "scheduled_size",
			Help:      "Current Asynq scheduled queue size.",
		},
		[]string{"queue"},
	)

	MySQLOpenConnections = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: "mysql",
			Name:      "open_connections",
			Help:      "Current number of open MySQL connections.",
		},
		[]string{"db_name", "role"},
	)

	MySQLInUse = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: "mysql",
			Name:      "in_use",
			Help:      "Current number of in-use MySQL connections.",
		},
		[]string{"db_name", "role"},
	)

	MySQLIdle = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: "mysql",
			Name:      "idle",
			Help:      "Current number of idle MySQL connections.",
		},
		[]string{"db_name", "role"},
	)

	MySQLWaitCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: "mysql",
			Name:      "wait_count",
			Help:      "Current sampled MySQL wait count.",
		},
		[]string{"db_name", "role"},
	)

	MySQLWaitDurationSeconds = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: "mysql",
			Name:      "wait_duration_seconds",
			Help:      "Current sampled MySQL wait duration in seconds.",
		},
		[]string{"db_name", "role"},
	)

	MySQLMaxIdleClosedTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: "mysql",
			Name:      "max_idle_closed_total",
			Help:      "Current sampled MySQL max idle closed total.",
		},
		[]string{"db_name", "role"},
	)

	MySQLMaxLifetimeClosedTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: metricNamespace,
			Subsystem: "mysql",
			Name:      "max_lifetime_closed_total",
			Help:      "Current sampled MySQL max lifetime closed total.",
		},
		[]string{"db_name", "role"},
	)
)

func init() {
	prometheus.MustRegister(
		HTTPRequestsTotal,
		HTTPRequestDuration,
		SelectionTotal,
		SelectionDuration,
		SelectionPersistenceTotal,
		SelectionPersistenceDuration,
		EnrollmentProjectionTotal,
		WaitlistPromotionTotal,
		ProjectionRepairPending,
		RabbitMQPublishTotal,
		RabbitMQConsumeTotal,
		WebSocketActiveConnections,
		WebSocketConnectionsTotal,
		WebSocketBroadcastEventsTotal,
		WebSocketDeliveriesTotal,
		DanmakuSubscriberReconnectTotal,
		OutboxDispatchTotal,
		OutboxDispatchDuration,
		RedisOpsTotal,
		RedisOpDuration,
		AsynqTaskTotal,
		AsynqTaskDuration,
		AsynqQueueSize,
		AsynqRetrySize,
		AsynqScheduledSize,
		MySQLOpenConnections,
		MySQLInUse,
		MySQLIdle,
		MySQLWaitCount,
		MySQLWaitDurationSeconds,
		MySQLMaxIdleClosedTotal,
		MySQLMaxLifetimeClosedTotal,
	)
}
