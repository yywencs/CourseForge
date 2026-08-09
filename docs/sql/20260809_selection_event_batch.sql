ALTER TABLE `selection_event`
  ADD COLUMN `consume_batch_id` varchar(32) DEFAULT NULL
    COMMENT '首次取得该事件的Redis Stream消费批次；用于批量幂等占位'
    AFTER `occurred_at`,
  ADD KEY `idx_selection_event_consume_batch` (`consume_batch_id`, `id`);
