-- 迁移脚本：删除 user_balance.version 字段
-- 执行时间：需要根据实际情况调整
-- 注意：此字段未被使用，可以安全删除

USE `billing_service`;

-- 删除 version 字段
ALTER TABLE `user_balance` DROP COLUMN `version`;

