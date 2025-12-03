package model

import (
	"time"
)

// 充值订单状态常量
const (
	RechargeStatusPending = "pending" // 待支付
	RechargeStatusSuccess = "success" // 支付成功
	RechargeStatusFailed  = "failed"  // 支付失败
)

// RechargeOrder 充值订单表（用于幂等性保证）
type RechargeOrder struct {
	OrderID        string    `gorm:"primaryKey;type:varchar(64)"`
	UserID         string    `gorm:"type:varchar(36);not null;index"`
	Amount         float64   `gorm:"type:decimal(10,2);not null"`
	PaymentOrderID string    `gorm:"type:varchar(64);uniqueIndex"` // payment-service 的订单ID
	Status         string    `gorm:"type:enum('pending','success','failed');not null;default:'pending'"` // pending:待支付, success:支付成功, failed:支付失败
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (RechargeOrder) TableName() string {
	return "recharge_order"
}



