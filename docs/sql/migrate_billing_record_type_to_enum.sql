-- 迁移脚本：将 billing_record.type 从 TINYINT 改为 ENUM
-- 执行时间：需要根据实际情况调整
-- 注意：此迁移需要先将现有数据转换

USE `billing_service`;

-- 步骤1: 添加临时列
ALTER TABLE `billing_record` 
ADD COLUMN `type_new` ENUM('free', 'balance') NULL COMMENT 'free:免费额度, balance:余额扣费' AFTER `type`;

-- 步骤2: 迁移现有数据（1 -> 'free', 2 -> 'balance'）
UPDATE `billing_record` 
SET `type_new` = CASE 
    WHEN `type` = 1 THEN 'free'
    WHEN `type` = 2 THEN 'balance'
    ELSE 'free'  -- 默认值，处理异常数据
END
WHERE `type_new` IS NULL;

-- 步骤3: 验证数据迁移（可选，检查是否有异常数据）
-- SELECT `type`, `type_new`, COUNT(*) 
-- FROM `billing_record` 
-- GROUP BY `type`, `type_new`;

-- 步骤4: 删除旧列
ALTER TABLE `billing_record` DROP COLUMN `type`;

-- 步骤5: 重命名新列
ALTER TABLE `billing_record` 
CHANGE COLUMN `type_new` `type` ENUM('free', 'balance') NOT NULL COMMENT 'free:免费额度, balance:余额扣费';

