ALTER TABLE `selection_event`
  MODIFY COLUMN `consume_batch_id` varchar(32) DEFAULT NULL
    COMMENT '首次取得该事件的Redis Stream消费批次；用于批量幂等占位',
  COMMENT='选课事件审计，同时作为Redis Stream消费幂等凭据';
