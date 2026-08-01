-- CourseForge catalog CRUD incremental migration
-- Target: MySQL 8.0+
-- 用于已经执行过旧版 courseforge.sql 的数据库；全新建库无需再执行本文件。

SET NAMES utf8mb4;
USE `courseforge`;

ALTER TABLE `course`
  ADD COLUMN `introduction` varchar(1000) NOT NULL DEFAULT '' COMMENT '课程简介' AFTER `credits`,
  ADD COLUMN `tags` json DEFAULT NULL COMMENT '课程标签JSON数组' AFTER `introduction`,
  ADD COLUMN `video_url` varchar(512) NOT NULL DEFAULT '' COMMENT '课程介绍视频地址' AFTER `tags`;

ALTER TABLE `teaching_class`
  ADD COLUMN `teacher_name` varchar(64) NOT NULL DEFAULT '' COMMENT '任课教师展示名称' AFTER `course_id`,
  ADD COLUMN `location` varchar(128) NOT NULL DEFAULT '' COMMENT '上课地点' AFTER `teacher_name`;
