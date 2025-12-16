-- 迁移脚本：在 billing_record 和 free_quota 表中增加 app_id 字段
-- 原因：支持按应用维度统计成本，开发者可以知道每个 app 的支出

USE billing_service;

-- 1. 在 billing_record 表中增加 app_id 字段
ALTER TABLE `billing_record` 
ADD COLUMN `app_id` VARCHAR(36) NOT NULL DEFAULT '' COMMENT '应用ID（用于按应用统计成本）' AFTER `user_id`;

-- 2. 添加索引
ALTER TABLE `billing_record` 
ADD INDEX `idx_app_id` (`app_id`) COMMENT '应用ID索引（用于按应用统计）',
ADD INDEX `idx_user_app` (`user_id`, `app_id`) COMMENT '用户和应用ID复合索引（用于查询某个应用的消费记录）';

-- 3. 在 free_quota 表中增加 app_id 字段
-- 注意：免费额度通常是开发者级别的，但为了支持按应用分配和管理，增加此字段
ALTER TABLE `free_quota` 
ADD COLUMN `app_id` VARCHAR(36) NOT NULL DEFAULT '' COMMENT '应用ID（用于按应用管理免费额度）' AFTER `user_id`;

-- 4. 修改 free_quota 的唯一索引，包含 app_id
-- 先删除旧的唯一索引
ALTER TABLE `free_quota` 
DROP INDEX `uk_user_service_month`;

-- 添加新的唯一索引（包含 app_id）
ALTER TABLE `free_quota` 
ADD UNIQUE KEY `uk_user_app_service_month` (`user_id`, `app_id`, `service_name`, `reset_month`) COMMENT '用户、应用、服务、月度配额唯一索引';

-- 5. 添加 app_id 索引
ALTER TABLE `free_quota` 
ADD INDEX `idx_app_id` (`app_id`) COMMENT '应用ID索引';

-- 验证：检查表结构
DESCRIBE billing_record;
DESCRIBE free_quota;
