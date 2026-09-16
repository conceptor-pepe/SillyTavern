-- 002_message_source.sql 为候选选择补齐来源及幂等约束。
-- 仅用于已执行 001_init.sql 的数据库，每个数据库执行一次。
-- 已通过 AutoMigrate 创建此字段及索引的数据库不得重复执行。
ALTER TABLE messages
    ADD COLUMN source_variant_id BIGINT UNSIGNED NULL AFTER parent_id,
    ADD UNIQUE KEY idx_messages_source_variant_id (source_variant_id);
