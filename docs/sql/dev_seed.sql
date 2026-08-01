-- CourseForge local development seed
-- Target: MySQL 8.0+
--
-- 仅用于本地开发和界面联调，请勿导入生产环境。
-- 本脚本可重复执行，不会清空已有的申请单、选课记录或异步事件。
--
-- 演示账号：
--   学号：2026001001
--   密码：CourseForge@123

SET NAMES utf8mb4;
SET time_zone = '+08:00';

USE `courseforge`;

START TRANSACTION;

-- --------------------------------------------------------------------------
-- Account and student profile
-- --------------------------------------------------------------------------

-- password_hash 为 CourseForge@123 的 bcrypt 哈希，cost=10。
INSERT INTO `user_account` (
  `id`,
  `password_hash`,
  `state`,
  `password_changed_at`,
  `token_version`
) VALUES (
  30001,
  '$2y$10$PawyuzQfYq8026AiZvyHyOyBnEC0gqwEG44sSEZ5uYwQpUF6xJWvG',
  'enabled',
  NOW(3),
  1
)
ON DUPLICATE KEY UPDATE
  `password_hash` = VALUES(`password_hash`),
  `state` = VALUES(`state`),
  `password_changed_at` = VALUES(`password_changed_at`),
  `token_version` = VALUES(`token_version`);

INSERT INTO `student` (
  `id`,
  `account_id`,
  `student_no`,
  `student_name`,
  `major_id`,
  `grade_year`,
  `state`
) VALUES (
  30001,
  30001,
  '2026001001',
  '开发测试学生',
  1001,
  2026,
  'active'
)
ON DUPLICATE KEY UPDATE
  `account_id` = VALUES(`account_id`),
  `student_no` = VALUES(`student_no`),
  `student_name` = VALUES(`student_name`),
  `major_id` = VALUES(`major_id`),
  `grade_year` = VALUES(`grade_year`),
  `state` = VALUES(`state`);

-- --------------------------------------------------------------------------
-- Course catalog
-- IDs 与 web/src/data/courseCatalog.ts 保持一致。
-- --------------------------------------------------------------------------

INSERT INTO `course` (`id`, `course_code`, `course_name`, `credits`) VALUES
  (10001, 'CS-304', '分布式系统设计', 3.5),
  (10002, 'AI-217', '智能交互产品实践', 2.0),
  (10003, 'HUM-109', '影像叙事与当代文化', 2.0)
ON DUPLICATE KEY UPDATE
  `course_code` = VALUES(`course_code`),
  `course_name` = VALUES(`course_name`),
  `credits` = VALUES(`credits`);

-- 开发学期使用稳定 ID 202601；当前开放轮次使用稳定 ID 30001。
INSERT INTO `selection_round` (
  `id`,
  `term_id`,
  `round_code`,
  `round_name`,
  `start_time`,
  `end_time`,
  `state`
) VALUES (
  30001,
  202601,
  'DEV-ROUND-1',
  '开发环境第一轮选课',
  DATE_SUB(NOW(3), INTERVAL 1 DAY),
  DATE_ADD(NOW(3), INTERVAL 30 DAY),
  'open'
)
ON DUPLICATE KEY UPDATE
  `term_id` = VALUES(`term_id`),
  `round_name` = VALUES(`round_name`),
  `start_time` = VALUES(`start_time`),
  `end_time` = VALUES(`end_time`),
  `state` = VALUES(`state`);

INSERT INTO `teaching_class` (
  `id`,
  `class_code`,
  `term_id`,
  `course_id`,
  `capacity`,
  `selected_count`,
  `minimum_grade_year`,
  `maximum_grade_year`,
  `state`
) VALUES
  (20001, 'CS-304-01', 202601, 10001, 60, 0, NULL, NULL, 'open'),
  (20002, 'AI-217-01', 202601, 10002, 40, 0, NULL, NULL, 'open'),
  (20003, 'HUM-109-01', 202601, 10003, 80, 0, NULL, NULL, 'open')
ON DUPLICATE KEY UPDATE
  `class_code` = VALUES(`class_code`),
  `term_id` = VALUES(`term_id`),
  `course_id` = VALUES(`course_id`),
  `capacity` = GREATEST(`selected_count`, VALUES(`capacity`)),
  `minimum_grade_year` = VALUES(`minimum_grade_year`),
  `maximum_grade_year` = VALUES(`maximum_grade_year`),
  `state` = VALUES(`state`);

INSERT INTO `teaching_class_schedule` (
  `teaching_class_id`,
  `day_of_week`,
  `start_week`,
  `end_week`,
  `start_section`,
  `end_section`
) VALUES
  (20001, 2, 1, 16, 3, 4),
  (20002, 4, 1, 16, 7, 8),
  (20003, 5, 1, 16, 5, 6)
ON DUPLICATE KEY UPDATE
  `teaching_class_id` = VALUES(`teaching_class_id`);

INSERT INTO `selection_round_class` (
  `round_id`,
  `teaching_class_id`,
  `state`
) VALUES
  (30001, 20001, 'open'),
  (30001, 20002, 'open'),
  (30001, 20003, 'open')
ON DUPLICATE KEY UPDATE
  `state` = VALUES(`state`);

-- 重复执行时保留已经占用的额度，只更新额度上限和开发上下文。
INSERT INTO `student_selection_quota` (
  `round_id`,
  `term_id`,
  `student_id`,
  `credit_limit`,
  `selected_credits`,
  `course_limit`,
  `selected_course_count`
) VALUES (
  30001,
  202601,
  30001,
  20.0,
  0,
  8,
  0
)
ON DUPLICATE KEY UPDATE
  `term_id` = VALUES(`term_id`),
  `credit_limit` = GREATEST(`selected_credits`, VALUES(`credit_limit`)),
  `course_limit` = GREATEST(`selected_course_count`, VALUES(`course_limit`));

COMMIT;

-- 便于命令行执行后立即核对登录账号与当前选课上下文。
SELECT
  s.`id` AS `student_id`,
  s.`student_no`,
  s.`student_name`,
  sr.`term_id`,
  sr.`id` AS `round_id`,
  sr.`end_time` AS `round_end_time`
FROM `student` AS s
CROSS JOIN `selection_round` AS sr
WHERE s.`id` = 30001
  AND sr.`id` = 30001;
