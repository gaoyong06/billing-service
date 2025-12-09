-- recharge_order_id 重命名为 order_id 迁移脚本
-- 将 recharge_order 表的主键字段从 recharge_order_id 重命名为 order_id
-- 与 payment-service 的 order_id 保持一致
-- 执行时间：建议在低峰期执行

-- 1. 重命名字段
ALTER TABLE `recharge_order` 
CHANGE COLUMN `recharge_order_id` `order_id` VARCHAR(64) NOT NULL COMMENT '订单号（billing-service生成，格式：recharge_{uid}_{timestamp}，作为主键，传给payment-service作为业务订单号order_id）';

-- 2. 验证迁移结果
-- 检查表结构
DESCRIBE `recharge_order`;

-- 3. 检查数据完整性
SELECT COUNT(*) as total_orders 
FROM `recharge_order`;

-- 4. 验证主键约束
SELECT COUNT(*) as duplicate_count
FROM (
    SELECT order_id, COUNT(*) as cnt
    FROM recharge_order
    GROUP BY order_id
    HAVING cnt > 1
) as duplicates;
