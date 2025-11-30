-- Billing Service Database Schema
-- Version: 1.0
-- Created: 2024-12-01

CREATE DATABASE IF NOT EXISTS `billing_service` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

USE `billing_service`;

-- Table: user_balance
CREATE TABLE IF NOT EXISTS `user_balance` (
    `user_balance_id` VARCHAR(36) NOT NULL COMMENT '主键ID',
    `user_id` VARCHAR(36) NOT NULL COMMENT '用户ID',
    `balance` DECIMAL(10, 2) DEFAULT 0.00 COMMENT '余额',
    `version` INT DEFAULT 0 COMMENT '乐观锁版本号',
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`user_balance_id`),
    UNIQUE KEY `uk_user_id` (`user_id`) COMMENT '用户ID唯一索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='账户余额表';

-- Table: free_quota
CREATE TABLE IF NOT EXISTS `free_quota` (
    `free_quota_id` VARCHAR(36) NOT NULL COMMENT '主键ID',
    `user_id` VARCHAR(36) NOT NULL COMMENT '用户ID',
    `service_name` VARCHAR(32) NOT NULL COMMENT '服务名: passport/payment/asset',
    `total_quota` INT DEFAULT 0 COMMENT '总额度',
    `used_quota` INT DEFAULT 0 COMMENT '已用额度',
    `reset_month` VARCHAR(7) NOT NULL COMMENT '重置月份: 2024-11',
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    `updated_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
    PRIMARY KEY (`free_quota_id`),
    UNIQUE KEY `uk_user_service_month` (`user_id`, `service_name`, `reset_month`) COMMENT '用户服务月度配额唯一索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='免费额度表';

-- Table: billing_record
CREATE TABLE IF NOT EXISTS `billing_record` (
    `billing_record_id` VARCHAR(36) NOT NULL COMMENT '主键ID',
    `user_id` VARCHAR(36) NOT NULL COMMENT '用户ID',
    `service_name` VARCHAR(32) NOT NULL COMMENT '服务名',
    `type` TINYINT NOT NULL COMMENT '1:免费额度, 2:余额扣费',
    `amount` DECIMAL(10, 4) DEFAULT 0.0000 COMMENT '扣费金额',
    `count` INT DEFAULT 1 COMMENT '调用次数',
    `created_at` TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    PRIMARY KEY (`billing_record_id`),
    INDEX `idx_user_date` (`user_id`, `created_at`) COMMENT '用户消费记录索引'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='消费流水表';
