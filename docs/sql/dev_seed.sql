-- CourseForge local development seed
-- Target: MySQL 8.0+
--
-- 仅用于本地开发和界面联调，请勿导入生产环境。
-- 本脚本可重复执行，不会清空已有的申请单、选课记录或异步事件。
--
-- 演示账号：
--   学号：2026001001 - 2026001005
--   密码：CourseForge@123
--
-- 开发管理员：
--   用户名：admin
--   密码：CourseForgeAdmin@123

SET NAMES utf8mb4;
SET time_zone = '+08:00';

USE `courseforge`;

START TRANSACTION;

-- --------------------------------------------------------------------------
-- Account and student profile
-- --------------------------------------------------------------------------

-- 五个学生账号共用开发密码 CourseForge@123；仅用于演示环境。
INSERT INTO `user_account` (
  `id`,
  `password_hash`,
  `state`,
  `password_changed_at`,
  `token_version`
) VALUES
  (30001, '$2y$10$PawyuzQfYq8026AiZvyHyOyBnEC0gqwEG44sSEZ5uYwQpUF6xJWvG', 'enabled', NOW(3), 1),
  (30003, '$2y$10$PawyuzQfYq8026AiZvyHyOyBnEC0gqwEG44sSEZ5uYwQpUF6xJWvG', 'enabled', NOW(3), 1),
  (30004, '$2y$10$PawyuzQfYq8026AiZvyHyOyBnEC0gqwEG44sSEZ5uYwQpUF6xJWvG', 'enabled', NOW(3), 1),
  (30005, '$2y$10$PawyuzQfYq8026AiZvyHyOyBnEC0gqwEG44sSEZ5uYwQpUF6xJWvG', 'enabled', NOW(3), 1),
  (30006, '$2y$10$PawyuzQfYq8026AiZvyHyOyBnEC0gqwEG44sSEZ5uYwQpUF6xJWvG', 'enabled', NOW(3), 1)
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
) VALUES
  (30001, 30001, '2026001001', '林知夏', 1001, 2026, 'active'),
  (30003, 30003, '2026001002', '周明远', 1001, 2025, 'active'),
  (30004, 30004, '2026001003', '许安然', 1002, 2026, 'active'),
  (30005, 30005, '2026001004', '陈星野', 1003, 2024, 'active'),
  (30006, 30006, '2026001005', '沈嘉言', 1002, 2025, 'active')
ON DUPLICATE KEY UPDATE
  `account_id` = VALUES(`account_id`),
  `student_no` = VALUES(`student_no`),
  `student_name` = VALUES(`student_name`),
  `major_id` = VALUES(`major_id`),
  `grade_year` = VALUES(`grade_year`),
  `state` = VALUES(`state`);

-- password_hash 为 CourseForgeAdmin@123 的 bcrypt 哈希，cost=10。
INSERT INTO `user_account` (
  `id`,
  `password_hash`,
  `state`,
  `password_changed_at`,
  `token_version`
) VALUES (
  30002,
  '$2a$10$f..MW0bDyzpdXe9RbYYI8ON/B9oFkQWLiAfp/RSw4QX98AltD.5ry',
  'enabled',
  NOW(3),
  1
)
ON DUPLICATE KEY UPDATE
  `password_hash` = VALUES(`password_hash`),
  `state` = VALUES(`state`),
  `password_changed_at` = VALUES(`password_changed_at`),
  `token_version` = VALUES(`token_version`);

INSERT INTO `administrator` (
  `id`,
  `account_id`,
  `username`
) VALUES (
  30001,
  30002,
  'admin'
)
ON DUPLICATE KEY UPDATE
  `account_id` = VALUES(`account_id`),
  `username` = VALUES(`username`);

-- --------------------------------------------------------------------------
-- Course catalog
-- 使用稳定 ID，便于接口、选课记录和界面联调。
-- --------------------------------------------------------------------------

INSERT INTO `course` (
  `id`, `course_code`, `course_name`, `credits`, `introduction`, `tags`
) VALUES
  (10001, 'CS-304', '分布式系统设计', 3.5,
   '从一致性协议到消息可靠性，用一次完整工程实践理解分布式系统。',
   JSON_ARRAY('专业核心', '项目制')),
  (10002, 'AI-217', '智能交互产品实践', 2.0,
   '围绕真实校园场景，完成从用户研究、原型到可用产品的完整过程。',
   JSON_ARRAY('跨专业', '工作坊')),
  (10003, 'HUM-109', '影像叙事与当代文化', 2.0,
   '以经典影像片段为入口，讨论媒介如何改变我们理解世界的方式。',
   JSON_ARRAY('通识选修', '研讨')),
  (10004, 'CS-101', '数据结构与算法', 4.0,
   '使用工程问题串联线性结构、树、图以及常用算法设计方法。',
   JSON_ARRAY('专业基础', '算法')),
  (10005, 'CS-205', '数据库系统原理', 3.5,
   '从关系模型、索引与事务出发，理解数据库系统的设计与实现。',
   JSON_ARRAY('专业核心', '数据库')),
  (10006, 'CS-206', '计算机网络', 3.5,
   '围绕互联网分层体系，学习协议设计、可靠传输与网络编程。',
   JSON_ARRAY('专业核心', '网络')),
  (10007, 'CS-318', 'Go 高并发服务实践', 3.0,
   '使用 Go 完成一个具备限流、异步处理和可观测性的高并发服务。',
   JSON_ARRAY('项目制', '后端开发', 'Go')),
  (10008, 'AI-201', '机器学习导论', 3.0,
   '通过分类、回归与聚类任务掌握机器学习的基本思想和实践流程。',
   JSON_ARRAY('专业基础', '人工智能'))
ON DUPLICATE KEY UPDATE
  `course_code` = VALUES(`course_code`),
  `course_name` = VALUES(`course_name`),
  `credits` = VALUES(`credits`),
  `introduction` = VALUES(`introduction`),
  `tags` = VALUES(`tags`);

-- 先修关系用于演示通过、分数不足和缺少先修课程三种校验结果。
INSERT INTO `course_prerequisite` (
  `course_id`, `prerequisite_course_id`, `minimum_score`
) VALUES
  (10001, 10004, 75.00),
  (10001, 10006, 70.00),
  (10002, 10008, 70.00),
  (10007, 10004, 70.00),
  (10007, 10005, 70.00)
ON DUPLICATE KEY UPDATE
  `minimum_score` = VALUES(`minimum_score`);

-- 开发学期使用稳定 ID 202601；当前开放轮次使用稳定 ID 30001。
INSERT INTO `selection_round` (
  `id`,
  `term_id`,
  `round_code`,
  `round_name`,
  `start_time`,
  `end_time`,
  `state`
) VALUES
  (30001, 202601, 'DEV-ROUND-1', '2026 春季第一轮选课',
   DATE_SUB(NOW(3), INTERVAL 1 DAY), DATE_ADD(NOW(3), INTERVAL 30 DAY), 'open'),
  (30002, 202601, 'DEV-ROUND-2', '2026 春季补退选',
   DATE_ADD(NOW(3), INTERVAL 31 DAY), DATE_ADD(NOW(3), INTERVAL 45 DAY), 'planned'),
  (30003, 202501, 'DEV-HISTORY-ROUND', '2025 春季选课（已结束）',
   DATE_SUB(NOW(3), INTERVAL 400 DAY), DATE_SUB(NOW(3), INTERVAL 370 DAY), 'closed')
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
  `teacher_name`,
  `location`,
  `capacity`,
  `selected_count`,
  `minimum_grade_year`,
  `maximum_grade_year`,
  `state`
) VALUES
  (20001, 'CS-304-01', 202601, 10001, '周屿教授', '格物楼 A308', 60, 0, NULL, NULL, 'open'),
  (20002, 'AI-217-01', 202601, 10002, '许南乔副教授', '创新中心 C201', 40, 0, NULL, NULL, 'open'),
  (20003, 'HUM-109-01', 202601, 10003, '陈见微讲师', '人文馆 109', 80, 0, NULL, NULL, 'open'),
  (20004, 'CS-101-01', 202601, 10004, '顾清川教授', '格物楼 B101', 90, 0, NULL, NULL, 'open'),
  (20005, 'CS-205-01', 202601, 10005, '唐映雪副教授', '格物楼 B204', 70, 0, 2024, 2026, 'open'),
  (20006, 'CS-206-01', 202601, 10006, '陆行舟副教授', '格物楼 A205', 65, 0, NULL, NULL, 'open'),
  (20007, 'CS-318-01', 202601, 10007, '江临讲师', '工程训练中心 302', 35, 0, 2024, 2026, 'open'),
  (20008, 'CS-304-02', 202601, 10001, '周屿教授', '格物楼 A310', 45, 0, NULL, NULL, 'open'),
  (20009, 'AI-201-01', 202601, 10008, '宋青禾教授', '创新中心 C105', 55, 0, NULL, NULL, 'open')
ON DUPLICATE KEY UPDATE
  `class_code` = VALUES(`class_code`),
  `term_id` = VALUES(`term_id`),
  `course_id` = VALUES(`course_id`),
  `teacher_name` = VALUES(`teacher_name`),
  `location` = VALUES(`location`),
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
  (20001, 4, 1, 16, 3, 4),
  (20002, 4, 1, 16, 7, 8),
  (20003, 5, 1, 16, 5, 6),
  (20004, 1, 1, 16, 1, 2),
  (20004, 3, 1, 16, 1, 2),
  (20005, 2, 1, 16, 5, 6),
  (20005, 5, 1, 16, 3, 4),
  (20006, 3, 1, 16, 3, 4),
  (20006, 5, 1, 16, 1, 2),
  (20007, 4, 1, 16, 7, 8),
  (20007, 6, 1, 16, 1, 4),
  (20008, 1, 1, 16, 7, 8),
  (20008, 3, 1, 16, 7, 8),
  (20009, 2, 1, 16, 7, 8),
  (20009, 4, 1, 16, 5, 6)
ON DUPLICATE KEY UPDATE
  `teaching_class_id` = VALUES(`teaching_class_id`);

-- 专业范围：Go 实践课仅开放给 1001/1002，机器学习课程拒绝专业 1003。
INSERT INTO `teaching_class_major_scope` (
  `teaching_class_id`, `major_id`, `scope_type`
) VALUES
  (20007, 1001, 'allow'),
  (20007, 1002, 'allow'),
  (20009, 1003, 'deny')
ON DUPLICATE KEY UPDATE
  `scope_type` = VALUES(`scope_type`);

INSERT INTO `selection_round_class` (
  `round_id`,
  `teaching_class_id`,
  `state`
) VALUES
  (30001, 20001, 'open'),
  (30001, 20002, 'open'),
  (30001, 20003, 'open'),
  (30001, 20004, 'open'),
  (30001, 20005, 'open'),
  (30001, 20006, 'open'),
  (30001, 20007, 'open'),
  (30001, 20008, 'open'),
  (30001, 20009, 'open')
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
) VALUES
  (30001, 202601, 30001, 20.0, 0, 8, 0),
  (30001, 202601, 30003, 20.0, 0, 8, 0),
  (30001, 202601, 30004, 18.0, 0, 7, 0),
  (30001, 202601, 30005, 16.0, 0, 6, 0),
  (30001, 202601, 30006, 20.0, 0, 8, 0)
ON DUPLICATE KEY UPDATE
  `term_id` = VALUES(`term_id`),
  `credit_limit` = GREATEST(`selected_credits`, VALUES(`credit_limit`)),
  `course_limit` = GREATEST(`selected_course_count`, VALUES(`course_limit`));

-- 历史成绩让不同账号具备不同的先修课条件。
INSERT INTO `student_course_history` (
  `student_id`, `course_id`, `term_id`, `score`, `result`, `completed_at`
) VALUES
  (30001, 10004, 202501, 88.00, 'passed', DATE_SUB(NOW(3), INTERVAL 180 DAY)),
  (30001, 10005, 202501, 91.00, 'passed', DATE_SUB(NOW(3), INTERVAL 175 DAY)),
  (30001, 10006, 202501, 82.00, 'passed', DATE_SUB(NOW(3), INTERVAL 170 DAY)),
  (30001, 10008, 202501, 76.00, 'passed', DATE_SUB(NOW(3), INTERVAL 165 DAY)),
  (30003, 10004, 202501, 68.00, 'passed', DATE_SUB(NOW(3), INTERVAL 180 DAY)),
  (30003, 10005, 202501, 86.00, 'passed', DATE_SUB(NOW(3), INTERVAL 175 DAY)),
  (30003, 10006, 202501, 73.00, 'passed', DATE_SUB(NOW(3), INTERVAL 170 DAY)),
  (30004, 10004, 202501, 92.00, 'passed', DATE_SUB(NOW(3), INTERVAL 180 DAY)),
  (30004, 10008, 202501, 65.00, 'failed', DATE_SUB(NOW(3), INTERVAL 165 DAY)),
  (30005, 10003, 202401, NULL, 'exempted', DATE_SUB(NOW(3), INTERVAL 500 DAY)),
  (30006, 10004, 202501, 79.00, 'passed', DATE_SUB(NOW(3), INTERVAL 180 DAY)),
  (30006, 10005, 202501, 74.00, 'passed', DATE_SUB(NOW(3), INTERVAL 175 DAY)),
  (30006, 10008, 202501, 89.00, 'passed', DATE_SUB(NOW(3), INTERVAL 165 DAY))
ON DUPLICATE KEY UPDATE
  `score` = VALUES(`score`),
  `result` = VALUES(`result`),
  `completed_at` = VALUES(`completed_at`);

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
WHERE s.`id` IN (30001, 30003, 30004, 30005, 30006)
  AND sr.`id` = 30001;

SELECT
  a.`id` AS `administrator_id`,
  a.`username`,
  ua.`state` AS `account_state`,
  ua.`token_version`
FROM `administrator` AS a
JOIN `user_account` AS ua ON ua.`id` = a.`account_id`
WHERE a.`id` = 30001;
