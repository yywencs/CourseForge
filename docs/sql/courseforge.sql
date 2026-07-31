-- CourseForge core schema
-- Target: MySQL 8.0+
--
-- Design notes:
-- 1. Redis is responsible for the hot-path reservation. MySQL is the durable
--    source of record and repeats the quota/capacity checks during persistence.
-- 2. Business IDs and auto-increment primary keys coexist. Business IDs are
--    used in APIs, Redis and messages; numeric primary keys keep indexes small.
-- 3. Physical foreign keys are intentionally omitted so student-owned tables
--    can be sharded later. All logical foreign keys have supporting indexes.
-- 4. This script creates a new database and must not be used as a migration.

SET NAMES utf8mb4;
SET time_zone = '+08:00';

CREATE DATABASE IF NOT EXISTS `courseforge`
  DEFAULT CHARACTER SET utf8mb4
  DEFAULT COLLATE utf8mb4_0900_ai_ci;

USE `courseforge`;

-- --------------------------------------------------------------------------
-- Student identity
-- --------------------------------------------------------------------------

CREATE TABLE `student` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '内部学生ID',
  `student_no` varchar(32) DEFAULT NULL COMMENT '登录学号；未开通登录时为NULL',
  `student_name` varchar(64) NOT NULL DEFAULT '' COMMENT '学生姓名',
  `password_hash` varchar(255) DEFAULT NULL COMMENT 'bcrypt密码哈希',
  `major_id` bigint unsigned NOT NULL COMMENT '专业标识',
  `grade_year` smallint unsigned NOT NULL COMMENT '入学年份',
  `state` varchar(16) NOT NULL DEFAULT 'active'
    COMMENT 'active/suspended/graduated/withdrawn',
  `create_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `update_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_student_no` (`student_no`),
  KEY `idx_student_major_grade_state` (`major_id`, `grade_year`, `state`),
  CONSTRAINT `chk_student_state`
    CHECK (`state` IN ('active', 'suspended', 'graduated', 'withdrawn'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='学生';

-- --------------------------------------------------------------------------
-- Academic catalog
-- --------------------------------------------------------------------------

CREATE TABLE `course` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '课程ID',
  `course_code` varchar(32) NOT NULL COMMENT '课程编码',
  `course_name` varchar(128) NOT NULL COMMENT '课程名称',
  `credits` decimal(5,1) unsigned NOT NULL COMMENT '课程学分',
  `create_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `update_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_course_code` (`course_code`),
  CONSTRAINT `chk_course_credits`
    CHECK (`credits` > 0)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='课程目录';

CREATE TABLE `course_prerequisite` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `course_id` bigint unsigned NOT NULL COMMENT '目标课程ID',
  `prerequisite_course_id` bigint unsigned NOT NULL COMMENT '先修课程ID',
  `minimum_score` decimal(5,2) unsigned DEFAULT NULL COMMENT '最低成绩，NULL表示通过即可',
  `create_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `update_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_course_prerequisite`
    (`course_id`, `prerequisite_course_id`),
  KEY `idx_prerequisite_course` (`prerequisite_course_id`),
  CONSTRAINT `chk_prerequisite_not_self`
    CHECK (`course_id` <> `prerequisite_course_id`),
  CONSTRAINT `chk_prerequisite_score`
    CHECK (`minimum_score` IS NULL OR `minimum_score` <= 100)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='课程先修关系';

CREATE TABLE `student_course_history` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `student_id` bigint unsigned NOT NULL COMMENT '学生ID',
  `course_id` bigint unsigned NOT NULL COMMENT '课程ID',
  `term_id` bigint unsigned NOT NULL COMMENT '修读学期ID',
  `score` decimal(5,2) unsigned DEFAULT NULL COMMENT '百分制成绩',
  `result` varchar(16) NOT NULL COMMENT 'passed/failed/exempted',
  `completed_at` datetime(3) DEFAULT NULL COMMENT '完成时间',
  `create_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `update_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_student_course_history`
    (`student_id`, `course_id`, `term_id`),
  KEY `idx_history_student_result` (`student_id`, `result`, `course_id`),
  KEY `idx_history_course_term` (`course_id`, `term_id`),
  CONSTRAINT `chk_history_score`
    CHECK (`score` IS NULL OR `score` <= 100),
  CONSTRAINT `chk_history_result`
    CHECK (`result` IN ('passed', 'failed', 'exempted'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='学生课程历史，用于先修课校验';

-- --------------------------------------------------------------------------
-- Teaching classes and eligibility scopes
-- --------------------------------------------------------------------------

CREATE TABLE `teaching_class` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '教学班ID',
  `class_code` varchar(32) NOT NULL COMMENT '教学班编码',
  `term_id` bigint unsigned NOT NULL COMMENT '学期ID',
  `course_id` bigint unsigned NOT NULL COMMENT '课程ID',
  `capacity` int unsigned NOT NULL COMMENT '教学班容量',
  `selected_count` int unsigned NOT NULL DEFAULT 0 COMMENT 'MySQL已确认选课人数',
  `minimum_grade_year` smallint unsigned DEFAULT NULL COMMENT '最小允许入学年份',
  `maximum_grade_year` smallint unsigned DEFAULT NULL COMMENT '最大允许入学年份',
  `state` varchar(16) NOT NULL DEFAULT 'planned'
    COMMENT 'planned/open/closed/cancelled',
  `create_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `update_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_term_class_code` (`term_id`, `class_code`),
  KEY `idx_class_term_state` (`term_id`, `state`),
  KEY `idx_class_term_course_state` (`term_id`, `course_id`, `state`),
  CONSTRAINT `chk_class_capacity`
    CHECK (`capacity` > 0 AND `selected_count` <= `capacity`),
  CONSTRAINT `chk_class_grade_range`
    CHECK (
      `minimum_grade_year` IS NULL
      OR `maximum_grade_year` IS NULL
      OR `minimum_grade_year` <= `maximum_grade_year`
    ),
  CONSTRAINT `chk_class_state`
    CHECK (`state` IN ('planned', 'open', 'closed', 'cancelled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='教学班及MySQL容量镜像';

CREATE TABLE `teaching_class_schedule` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `teaching_class_id` bigint unsigned NOT NULL COMMENT '教学班ID',
  `day_of_week` tinyint unsigned NOT NULL COMMENT '星期，1-7',
  `start_week` tinyint unsigned NOT NULL COMMENT '起始教学周',
  `end_week` tinyint unsigned NOT NULL COMMENT '结束教学周',
  `start_section` tinyint unsigned NOT NULL COMMENT '起始节次',
  `end_section` tinyint unsigned NOT NULL COMMENT '结束节次',
  `create_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `update_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_class_schedule`
    (`teaching_class_id`, `day_of_week`, `start_week`, `end_week`,
     `start_section`, `end_section`),
  KEY `idx_schedule_class_day`
    (`teaching_class_id`, `day_of_week`, `start_section`, `end_section`),
  CONSTRAINT `chk_schedule_day`
    CHECK (`day_of_week` BETWEEN 1 AND 7),
  CONSTRAINT `chk_schedule_week`
    CHECK (`start_week` >= 1 AND `end_week` >= `start_week`),
  CONSTRAINT `chk_schedule_section`
    CHECK (`start_section` >= 1 AND `end_section` >= `start_section`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='教学班上课时间，用于时间冲突校验';

CREATE TABLE `teaching_class_major_scope` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `teaching_class_id` bigint unsigned NOT NULL COMMENT '教学班ID',
  `major_id` bigint unsigned NOT NULL COMMENT '专业ID',
  `scope_type` varchar(16) NOT NULL DEFAULT 'allow' COMMENT 'allow/deny',
  `create_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_class_major_scope`
    (`teaching_class_id`, `major_id`),
  KEY `idx_major_scope` (`major_id`, `scope_type`, `teaching_class_id`),
  CONSTRAINT `chk_major_scope_type`
    CHECK (`scope_type` IN ('allow', 'deny'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='教学班专业范围；无记录表示不限制专业';

-- --------------------------------------------------------------------------
-- Selection rounds and student quota
-- --------------------------------------------------------------------------

CREATE TABLE `selection_round` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '选课轮次ID',
  `term_id` bigint unsigned NOT NULL COMMENT '学期ID',
  `round_code` varchar(32) NOT NULL COMMENT '轮次编码',
  `round_name` varchar(64) NOT NULL COMMENT '轮次名称',
  `start_time` datetime(3) NOT NULL COMMENT '开放时间',
  `end_time` datetime(3) NOT NULL COMMENT '结束时间',
  `state` varchar(16) NOT NULL DEFAULT 'planned'
    COMMENT 'planned/open/closed',
  `create_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `update_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_term_round_code` (`term_id`, `round_code`),
  KEY `idx_round_state_time` (`state`, `start_time`, `end_time`),
  CONSTRAINT `chk_round_time`
    CHECK (`end_time` > `start_time`),
  CONSTRAINT `chk_round_state`
    CHECK (`state` IN ('planned', 'open', 'closed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='选课轮次';

CREATE TABLE `selection_round_class` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `round_id` bigint unsigned NOT NULL COMMENT '选课轮次ID',
  `teaching_class_id` bigint unsigned NOT NULL COMMENT '教学班ID',
  `state` varchar(16) NOT NULL DEFAULT 'open' COMMENT 'open/closed',
  `create_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `update_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_round_class` (`round_id`, `teaching_class_id`),
  KEY `idx_round_class_state` (`round_id`, `state`, `teaching_class_id`),
  KEY `idx_class_round` (`teaching_class_id`, `round_id`),
  CONSTRAINT `chk_round_class_state`
    CHECK (`state` IN ('open', 'closed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='轮次开放的教学班';

CREATE TABLE `student_selection_quota` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `round_id` bigint unsigned NOT NULL COMMENT '选课轮次ID',
  `term_id` bigint unsigned NOT NULL COMMENT '冗余学期ID，方便查询和分片',
  `student_id` bigint unsigned NOT NULL COMMENT '学生ID',
  `credit_limit` decimal(5,1) unsigned NOT NULL COMMENT '学分上限',
  `selected_credits` decimal(5,1) unsigned NOT NULL DEFAULT 0
    COMMENT 'MySQL已确认占用学分',
  `course_limit` smallint unsigned NOT NULL COMMENT '课程门数上限',
  `selected_course_count` smallint unsigned NOT NULL DEFAULT 0
    COMMENT 'MySQL已确认课程门数',
  `create_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `update_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_round_student` (`round_id`, `student_id`),
  KEY `idx_quota_student_term` (`student_id`, `term_id`),
  CONSTRAINT `chk_quota_credits`
    CHECK (
      `credit_limit` > 0
      AND `selected_credits` >= 0
      AND `selected_credits` <= `credit_limit`
    ),
  CONSTRAINT `chk_quota_courses`
    CHECK (
      `course_limit` > 0
      AND `selected_course_count` <= `course_limit`
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='学生选课额度；Redis额度的MySQL持久化镜像';

-- --------------------------------------------------------------------------
-- Selection application, durable enrollment and event records
-- --------------------------------------------------------------------------

CREATE TABLE `selection_application` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '内部主键',
  `application_id` varchar(32) NOT NULL COMMENT '选课申请单业务ID',
  `request_id` varchar(64) NOT NULL COMMENT '客户端幂等请求ID',
  `round_id` bigint unsigned NOT NULL COMMENT '选课轮次ID',
  `term_id` bigint unsigned NOT NULL COMMENT '学期ID',
  `student_id` bigint unsigned NOT NULL COMMENT '学生ID',
  `course_id` bigint unsigned NOT NULL COMMENT '课程ID快照',
  `teaching_class_id` bigint unsigned NOT NULL COMMENT '教学班ID',
  `credits` decimal(5,1) unsigned NOT NULL COMMENT '申请时课程学分快照',
  `state` varchar(16) NOT NULL DEFAULT 'created'
    COMMENT 'created/reserved/processing/selected/rejected/cancelled',
  `failure_code` varchar(32) NOT NULL DEFAULT '' COMMENT '失败原因码',
  `failure_message` varchar(256) NOT NULL DEFAULT '' COMMENT '失败原因说明',
  `applied_at` datetime(3) NOT NULL COMMENT '申请时间',
  `completed_at` datetime(3) DEFAULT NULL COMMENT '业务完成时间',
  `source` varchar(16) NOT NULL DEFAULT 'web' COMMENT 'web/mobile/admin/system',
  `create_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `update_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_application_id` (`application_id`),
  UNIQUE KEY `uq_round_student_request`
    (`round_id`, `student_id`, `request_id`),
  KEY `idx_application_student_term_state`
    (`student_id`, `term_id`, `state`, `applied_at`),
  KEY `idx_application_class_state`
    (`teaching_class_id`, `state`, `applied_at`),
  KEY `idx_application_update_state` (`state`, `update_time`, `id`),
  CONSTRAINT `chk_application_credits`
    CHECK (`credits` > 0),
  CONSTRAINT `chk_application_state`
    CHECK (
      `state` IN (
        'created', 'reserved', 'processing',
        'selected', 'rejected', 'cancelled'
      )
    ),
  CONSTRAINT `chk_application_source`
    CHECK (`source` IN ('web', 'mobile', 'admin', 'system'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='选课申请单；相同request_id重复消费时幂等';

CREATE TABLE `selection_waitlist` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '候补顺序号',
  `waitlist_id` varchar(32) NOT NULL COMMENT '候补申请业务ID',
  `request_id` varchar(64) NOT NULL COMMENT '客户端幂等请求ID',
  `round_id` bigint unsigned NOT NULL COMMENT '选课轮次ID',
  `term_id` bigint unsigned NOT NULL COMMENT '学期ID',
  `student_id` bigint unsigned NOT NULL COMMENT '学生ID',
  `course_id` bigint unsigned NOT NULL COMMENT '课程ID快照',
  `teaching_class_id` bigint unsigned NOT NULL COMMENT '教学班ID',
  `credits` decimal(5,1) unsigned NOT NULL COMMENT '课程学分快照',
  `state` varchar(16) NOT NULL DEFAULT 'waiting'
    COMMENT 'waiting/promoting/promoted/cancelled',
  `failure_code` varchar(32) NOT NULL DEFAULT '',
  `failure_message` varchar(256) NOT NULL DEFAULT '',
  `active_key` varchar(96) DEFAULT NULL
    COMMENT '有效候补唯一键；完成或取消后置NULL，允许重新候补',
  `joined_at` datetime(3) NOT NULL,
  `promoted_at` datetime(3) DEFAULT NULL,
  `cancelled_at` datetime(3) DEFAULT NULL,
  `create_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `update_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_waitlist_id` (`waitlist_id`),
  UNIQUE KEY `uq_waitlist_request` (`round_id`, `student_id`, `request_id`),
  UNIQUE KEY `uq_waitlist_active` (`active_key`),
  KEY `idx_waitlist_promote`
    (`state`, `teaching_class_id`, `id`),
  KEY `idx_waitlist_student_term`
    (`student_id`, `term_id`, `state`, `id`),
  CONSTRAINT `chk_waitlist_credits` CHECK (`credits` > 0),
  CONSTRAINT `chk_waitlist_state`
    CHECK (`state` IN ('waiting', 'promoting', 'promoted', 'cancelled'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='教学班候补队列；自增主键同时作为全局稳定排队顺序';

CREATE TABLE `student_course_enrollment` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT COMMENT '内部主键',
  `enrollment_id` varchar(32) NOT NULL COMMENT '正式选课记录业务ID',
  `application_id` varchar(32) NOT NULL COMMENT '来源申请单ID',
  `term_id` bigint unsigned NOT NULL COMMENT '学期ID',
  `student_id` bigint unsigned NOT NULL COMMENT '学生ID',
  `course_id` bigint unsigned NOT NULL COMMENT '课程ID',
  `teaching_class_id` bigint unsigned NOT NULL COMMENT '教学班ID',
  `credits` decimal(5,1) unsigned NOT NULL COMMENT '选课时学分快照',
  `state` varchar(16) NOT NULL DEFAULT 'enrolled'
    COMMENT 'enrolled/dropped/completed',
  `active_key` varchar(96) DEFAULT NULL
    COMMENT '有效修读唯一键；退课后置NULL，允许重新选课',
  `enrolled_at` datetime(3) NOT NULL COMMENT '选课成功时间',
  `dropped_at` datetime(3) DEFAULT NULL COMMENT '退课时间',
  `create_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `update_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_enrollment_id` (`enrollment_id`),
  UNIQUE KEY `uq_enrollment_application` (`application_id`),
  UNIQUE KEY `uq_enrollment_active` (`active_key`),
  KEY `idx_enrollment_term_student_course` (`term_id`, `student_id`, `course_id`),
  KEY `idx_enrollment_student_term_state`
    (`student_id`, `term_id`, `state`, `teaching_class_id`),
  KEY `idx_enrollment_class_state`
    (`teaching_class_id`, `state`, `student_id`),
  CONSTRAINT `chk_enrollment_credits`
    CHECK (`credits` > 0),
  CONSTRAINT `chk_enrollment_state`
    CHECK (`state` IN ('enrolled', 'dropped', 'completed')),
  CONSTRAINT `chk_enrollment_drop_time`
    CHECK (
      (`state` = 'dropped' AND `dropped_at` IS NOT NULL)
      OR (`state` <> 'dropped')
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='学生正式选课记录；同一学期同一课程仅允许一条有效记录';

CREATE TABLE `selection_event` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `event_id` varchar(64) NOT NULL COMMENT '事件/消息幂等ID',
  `application_id` varchar(32) NOT NULL COMMENT '选课申请单ID',
  `student_id` bigint unsigned NOT NULL COMMENT '学生ID',
  `event_type` varchar(32) NOT NULL
    COMMENT 'reserved/selected/rejected/cancelled/dropped',
  `event_payload` json NOT NULL COMMENT '标准事件载荷快照',
  `occurred_at` datetime(3) NOT NULL COMMENT '业务发生时间',
  `create_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_selection_event_id` (`event_id`),
  KEY `idx_event_application_time` (`application_id`, `occurred_at`),
  KEY `idx_event_student_time` (`student_id`, `occurred_at`),
  CONSTRAINT `chk_selection_event_type`
    CHECK (
      `event_type` IN (
        'reserved', 'selected', 'rejected', 'cancelled', 'dropped'
      )
    )
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='选课事件审计，同时作为RabbitMQ消费幂等凭据';

CREATE TABLE `enrollment_projection_repair` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `repair_id` varchar(64) NOT NULL COMMENT '修复任务业务ID',
  `enrollment_id` varchar(32) NOT NULL COMMENT '退课记录ID',
  `operation` varchar(32) NOT NULL COMMENT 'release_dropped',
  `state` varchar(16) NOT NULL DEFAULT 'pending' COMMENT 'pending/completed',
  `retry_count` int unsigned NOT NULL DEFAULT 0,
  `next_retry_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `last_error` varchar(512) NOT NULL DEFAULT '',
  `completed_at` datetime(3) DEFAULT NULL,
  `create_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `update_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_projection_repair_id` (`repair_id`),
  UNIQUE KEY `uq_projection_repair_operation` (`enrollment_id`, `operation`),
  KEY `idx_projection_repair_dispatch` (`state`, `next_retry_at`, `id`),
  CONSTRAINT `chk_projection_repair_operation`
    CHECK (`operation` IN ('release_dropped')),
  CONSTRAINT `chk_projection_repair_state`
    CHECK (`state` IN ('pending', 'completed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='MySQL到Redis选课投影的可靠修复任务';

CREATE TABLE `outbox_event` (
  `id` bigint unsigned NOT NULL AUTO_INCREMENT,
  `event_id` varchar(64) NOT NULL COMMENT '事件ID',
  `aggregate_type` varchar(32) NOT NULL COMMENT '聚合类型',
  `aggregate_id` varchar(64) NOT NULL COMMENT '聚合ID',
  `topic` varchar(64) NOT NULL COMMENT 'RabbitMQ交换机/业务主题',
  `event_type` varchar(32) NOT NULL COMMENT '事件类型',
  `payload` json NOT NULL COMMENT '消息载荷',
  `state` varchar(16) NOT NULL DEFAULT 'pending'
    COMMENT 'pending/publishing/published/failed',
  `retry_count` int unsigned NOT NULL DEFAULT 0 COMMENT '发布重试次数',
  `next_retry_at` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `published_at` datetime(3) DEFAULT NULL,
  `last_error` varchar(512) NOT NULL DEFAULT '',
  `create_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `update_time` datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
    ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq_outbox_event_id` (`event_id`),
  KEY `idx_outbox_dispatch`
    (`state`, `next_retry_at`, `id`),
  KEY `idx_outbox_aggregate`
    (`aggregate_type`, `aggregate_id`, `id`),
  CONSTRAINT `chk_outbox_state`
    CHECK (`state` IN ('pending', 'publishing', 'published', 'failed'))
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci
  COMMENT='事务Outbox，用于选课落库后的后续事件';
