CREATE TABLE `student_notification` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `event_id` varchar(64) NOT NULL COMMENT '来源集成事件ID，同时作为消费幂等键',
  `student_id` bigint unsigned NOT NULL COMMENT '通知接收学生ID',
  `type` varchar(32) NOT NULL COMMENT '通知类型',
  `title` varchar(64) NOT NULL COMMENT '通知标题',
  `content` varchar(512) NOT NULL COMMENT '通知正文',
  `payload` json NOT NULL COMMENT '来源事件载荷快照',
  `occurred_at` datetime(3) NOT NULL COMMENT '业务事件发生时间',
  `read_at` datetime(3) DEFAULT NULL COMMENT '首次已读时间',
  `create_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_student_notification_event` (`event_id`),
  KEY `idx_student_notification_inbox` (`student_id`, `read_at`, `id`),
  CONSTRAINT `chk_student_notification_type`
    CHECK (`type` IN ('selection_result'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='学生站内通知；RabbitMQ至少一次消费并按事件ID幂等落库';
