package model

import (
	"billing-service/internal/constants"
	"time"
)

// 充值订单状态常量（引用 constants 包中的常量，保持一致性）
const (
	RechargeStatusPending = constants.OrderStatusPending // 待支付
	RechargeStatusSuccess = constants.OrderStatusSuccess // 支付成功
	RechargeStatusFailed  = constants.OrderStatusFailed  // 支付失败
)

// RechargeOrder 充值订单表（用于幂等性保证）
type RechargeOrder struct {
	RechargeOrderID string    `gorm:"primaryKey;column:recharge_order_id;type:varchar(64)"` // 充值订单ID（billing-service生成，传给payment-service作为业务订单号）
	UID             string    `gorm:"column:uid;type:varchar(36);not null;index:idx_uid"`
	Amount          float64   `gorm:"type:decimal(10,2);not null"`
	PaymentID       string    `gorm:"column:payment_id;type:varchar(64);uniqueIndex"`                     // 支付流水号（payment-service返回的payment_id）
	Status          string    `gorm:"type:enum('pending','success','failed');not null;default:'pending'"` // pending:待支付, success:支付成功, failed:支付失败
	CreatedAt       time.Time `gorm:"autoCreateTime"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (RechargeOrder) TableName() string {
	return "recharge_order"
}
