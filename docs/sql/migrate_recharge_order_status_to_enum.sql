-- 迁移脚本：将 recharge_order.status 从 VARCHAR 改为 ENUM
-- 执行时间：需要根据实际情况调整
-- 注意：此迁移需要先将现有数据转换

USE `billing_service`;

-- 步骤1: 添加临时列
ALTER TABLE `recharge_order` 
ADD COLUMN `status_new` ENUM('pending', 'success', 'failed') NULL COMMENT 'pending:待支付, success:支付成功, failed:支付失败' AFTER `status`;

-- 步骤2: 迁移现有数据（保持原有值，但需要验证是否符合枚举值）
UPDATE `recharge_order` 
SET `status_new` = CASE 
    WHEN `status` IN ('pending', 'success', 'failed') THEN `status`
    ELSE 'pending'  -- 默认值，处理异常数据
END
WHERE `status_new` IS NULL;

-- 步骤3: 验证数据迁移（可选，检查是否有异常数据）
-- SELECT `status`, `status_new`, COUNT(*) 
-- FROM `recharge_order` 
-- GROUP BY `status`, `status_new`;

-- 步骤4: 删除旧列
ALTER TABLE `recharge_order` DROP COLUMN `status`;

-- 步骤5: 重命名新列
ALTER TABLE `recharge_order` 
CHANGE COLUMN `status_new` `status` ENUM('pending', 'success', 'failed') NOT NULL DEFAULT 'pending' COMMENT 'pending:待支付, success:支付成功, failed:支付失败';

