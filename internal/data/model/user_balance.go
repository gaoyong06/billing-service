package model

import (
	"time"
)

// UserBalance 账户余额表
type UserBalance struct {
	UserBalanceID string    `gorm:"primaryKey;type:varchar(36)"`
	UserID        string    `gorm:"uniqueIndex;type:varchar(36);not null"`
	Balance       float64   `gorm:"type:decimal(10,2);default:0.00"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (UserBalance) TableName() string {
	return "user_balance"
}

