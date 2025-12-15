-- Billing Service 测试数据脚本
-- 为用户添加余额和免费配额，确保接口可以正常调用

USE `billing_service`;

-- 获取当前月份（格式：YYYY-MM）
SET @current_month = DATE_FORMAT(NOW(), '%Y-%m');

-- 为用户 00000000-0000-4000-8000-000000000071（对应 user_internal_id=113）添加余额
-- 注意：113 的十六进制是 71，所以 user_id 是 00000000-0000-4000-8000-000000000071
-- 插入或更新用户余额（如果已存在则更新，不存在则插入）
INSERT INTO `user_balance` (
    `user_balance_id`,
    `user_id`,
    `balance`,
    `created_at`,
    `updated_at`
) VALUES (
    UUID(),
    '00000000-0000-4000-8000-000000000071',
    1000.00,  -- 充值 1000 元余额
    NOW(),
    NOW()
)
ON DUPLICATE KEY UPDATE
    `balance` = 1000.00,
    `updated_at` = NOW();

-- 为用户添加 passport 服务的免费配额（10000 次/月）
INSERT INTO `free_quota` (
    `free_quota_id`,
    `user_id`,
    `service_name`,
    `total_quota`,
    `used_quota`,
    `reset_month`,
    `created_at`,
    `updated_at`
) VALUES (
    UUID(),
    '00000000-0000-4000-8000-000000000071',
    'passport',
    10000,  -- 总额度：10000 次/月（与配置文件一致）
    0,      -- 已用额度：0
    @current_month,
    NOW(),
    NOW()
)
ON DUPLICATE KEY UPDATE
    `total_quota` = 10000,
    `used_quota` = 0,
    `updated_at` = NOW();

-- 为前 10 个测试用户批量添加余额和免费配额（方便测试）
-- 用户 ID 格式：00000000-0000-4000-8000-000000000001 到 00000000-0000-4000-8000-000000000010
INSERT INTO `user_balance` (
    `user_balance_id`,
    `user_id`,
    `balance`,
    `created_at`,
    `updated_at`
)
SELECT 
    UUID(),
    CONCAT('00000000-0000-4000-8000-', LPAD(num, 12, '0')) AS user_id,
    1000.00 AS balance,
    NOW() AS created_at,
    NOW() AS updated_at
FROM (
    SELECT 1 AS num UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 UNION SELECT 5
    UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9 UNION SELECT 10
) AS numbers
ON DUPLICATE KEY UPDATE
    `balance` = 1000.00,
    `updated_at` = NOW();

-- 为前 10 个测试用户批量添加 passport 服务的免费配额
INSERT INTO `free_quota` (
    `free_quota_id`,
    `user_id`,
    `service_name`,
    `total_quota`,
    `used_quota`,
    `reset_month`,
    `created_at`,
    `updated_at`
)
SELECT 
    UUID(),
    CONCAT('00000000-0000-4000-8000-', LPAD(num, 12, '0')) AS user_id,
    'passport' AS service_name,
    10000 AS total_quota,
    0 AS used_quota,
    @current_month AS reset_month,
    NOW() AS created_at,
    NOW() AS updated_at
FROM (
    SELECT 1 AS num UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 UNION SELECT 5
    UNION SELECT 6 UNION SELECT 7 UNION SELECT 8 UNION SELECT 9 UNION SELECT 10
) AS numbers
ON DUPLICATE KEY UPDATE
    `total_quota` = 10000,
    `used_quota` = 0,
    `updated_at` = NOW();

-- 查询验证：查看用户余额
SELECT 
    `user_id`,
    `balance`,
    `created_at`,
    `updated_at`
FROM `user_balance`
WHERE `user_id` IN (
    '00000000-0000-4000-8000-000000000071',
    '00000000-0000-4000-8000-000000000001',
    '00000000-0000-4000-8000-000000000002'
)
ORDER BY `user_id`;

-- 查询验证：查看用户免费配额
SELECT 
    `user_id`,
    `service_name`,
    `total_quota`,
    `used_quota`,
    `reset_month`,
    `created_at`,
    `updated_at`
FROM `free_quota`
WHERE `user_id` IN (
    '00000000-0000-4000-8000-000000000071',
    '00000000-0000-4000-8000-000000000001',
    '00000000-0000-4000-8000-000000000002'
)
ORDER BY `user_id`, `service_name`;
