/*
 Navicat Premium Dump SQL

 Source Server         : localhost
 Source Server Type    : MySQL
 Source Server Version : 90500 (9.5.0)
 Source Host           : localhost:3306
 Source Schema         : billing_service

 Target Server Type    : MySQL
 Target Server Version : 90500 (9.5.0)
 File Encoding         : 65001

 Date: 22/01/2026 11:30:30
*/

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ----------------------------
-- Table structure for billing_record
-- ----------------------------
DROP TABLE IF EXISTS `billing_record`;
CREATE TABLE `billing_record` (
  `billing_record_id` varchar(36) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '主键ID',
  `user_id` varchar(36) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '用户ID',
  `app_id` varchar(36) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '应用ID（用于按应用统计成本）',
  `service_name` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '服务名',
  `type` enum('free','balance') COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'free:免费额度, balance:余额扣费',
  `amount` decimal(10,4) DEFAULT '0.0000' COMMENT '扣费金额',
  `count` int DEFAULT '1' COMMENT '调用次数',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  PRIMARY KEY (`billing_record_id`),
  KEY `idx_user_id_date` (`user_id`,`created_at`) COMMENT '用户消费记录索引',
  KEY `idx_app_id` (`app_id`) COMMENT '应用ID索引（用于按应用统计）',
  KEY `idx_user_app` (`user_id`,`app_id`) COMMENT '用户和应用ID复合索引（用于查询某个应用的消费记录）'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='消费流水表';

-- ----------------------------
-- Table structure for free_quota
-- ----------------------------
DROP TABLE IF EXISTS `free_quota`;
CREATE TABLE `free_quota` (
  `free_quota_id` varchar(36) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '主键ID',
  `user_id` varchar(36) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '用户ID',
  `app_id` varchar(36) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '' COMMENT '应用ID（用于按应用管理免费额度）',
  `service_name` varchar(32) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '服务名: passport/payment/asset',
  `total_quota` int DEFAULT '0' COMMENT '总额度',
  `used_quota` int DEFAULT '0' COMMENT '已用额度',
  `reset_month` varchar(7) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '重置月份: 2024-11',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`free_quota_id`),
  UNIQUE KEY `uk_user_app_service_month` (`user_id`,`app_id`,`service_name`,`reset_month`) COMMENT '用户、应用、服务、月度配额唯一索引',
  KEY `idx_app_id` (`app_id`) COMMENT '应用ID索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='免费额度表';

-- ----------------------------
-- Table structure for recharge_order
-- ----------------------------
DROP TABLE IF EXISTS `recharge_order`;
CREATE TABLE `recharge_order` (
  `order_id` varchar(64) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '订单号（billing-service生成，格式：recharge_{user_id}_{timestamp}，作为主键，传给payment-service作为业务订单号order_id）',
  `user_id` varchar(36) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '用户ID',
  `amount` decimal(10,2) NOT NULL COMMENT '充值金额',
  `payment_id` varchar(64) COLLATE utf8mb4_unicode_ci DEFAULT NULL COMMENT '支付流水号（payment-service返回的payment_id，用于关联payment-service的支付订单，有唯一索引保证幂等性）',
  `status` enum('pending','success','failed') COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'pending' COMMENT '订单状态: pending-待支付, success-支付成功, failed-支付失败',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`order_id`),
  UNIQUE KEY `uk_payment_id` (`payment_id`) COMMENT 'payment_id唯一索引（幂等性保证）',
  KEY `idx_user_id` (`user_id`) COMMENT '用户ID索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='充值订单表（幂等性保证）';

-- ----------------------------
-- Table structure for user_balance
-- ----------------------------
DROP TABLE IF EXISTS `user_balance`;
CREATE TABLE `user_balance` (
  `user_balance_id` varchar(36) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '主键ID',
  `user_id` varchar(36) COLLATE utf8mb4_unicode_ci NOT NULL COMMENT '用户ID',
  `balance` decimal(10,2) DEFAULT '0.00' COMMENT '余额',
  `created_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` timestamp NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  PRIMARY KEY (`user_balance_id`),
  UNIQUE KEY `uk_user_id` (`user_id`) COMMENT '用户ID唯一索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='账户余额表';

SET FOREIGN_KEY_CHECKS = 1;
