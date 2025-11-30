package model

import (
	"time"
)

// UserBalance 账户余额表
type UserBalance struct {
	UserBalanceID string    `gorm:"primaryKey;type:varchar(36)"`
	UserID        string    `gorm:"uniqueIndex;type:varchar(36);not null"`
	Balance       float64   `gorm:"type:decimal(10,2);default:0.00"`
	Version       int       `gorm:"default:0"` // 乐观锁版本号
	CreatedAt     time.Time `gorm:"autoCreateTime"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (UserBalance) TableName() string {
	return "user_balance"
}

// FreeQuota 免费额度表
type FreeQuota struct {
	FreeQuotaID string    `gorm:"primaryKey;type:varchar(36)"`
	UserID      string    `gorm:"type:varchar(36);not null;uniqueIndex:uk_user_service_month,priority:1"`
	ServiceName string    `gorm:"type:varchar(32);not null;uniqueIndex:uk_user_service_month,priority:2"`
	TotalQuota  int       `gorm:"default:0"`
	UsedQuota   int       `gorm:"default:0"`
	ResetMonth  string    `gorm:"type:varchar(7);not null;uniqueIndex:uk_user_service_month,priority:3"` // 2024-11
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// TableName 指定表名
func (FreeQuota) TableName() string {
	return "free_quota"
}

// BillingRecord 消费流水表
type BillingRecord struct {
	BillingRecordID string    `gorm:"primaryKey;type:varchar(36)"`
	UserID          string    `gorm:"type:varchar(36);not null;index:idx_user_date,priority:1"`
	ServiceName     string    `gorm:"type:varchar(32);not null"`
	Type            int8      `gorm:"type:tinyint;not null"` // 1:免费额度, 2:余额扣费
	Amount          float64   `gorm:"type:decimal(10,4);default:0.0000"`
	Count           int       `gorm:"default:1"`
	CreatedAt       time.Time `gorm:"autoCreateTime;index:idx_user_date,priority:2"`
}

// TableName 指定表名
func (BillingRecord) TableName() string {
	return "billing_record"
}

const (
	BillingTypeFree    = 1
	BillingTypeBalance = 2
)
