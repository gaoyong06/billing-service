package model

import (
	"billing-service/internal/constants"
	"time"
)

// 计费类型常量（引用 constants 包中的常量，保持一致性）
const (
	BillingTypeFree    = constants.BillingTypeFree    // 免费额度
	BillingTypeBalance = constants.BillingTypeBalance // 余额扣费
)

// BillingRecord 消费流水表
type BillingRecord struct {
	BillingRecordID string    `gorm:"primaryKey;type:varchar(36)"`
	UserID          string    `gorm:"column:user_id;type:varchar(36);not null;index:idx_user_id_date,priority:1;index:idx_user_app,priority:1"`
	AppID           string    `gorm:"column:app_id;type:varchar(36);not null;default:'';index:idx_app_id;index:idx_user_app,priority:2"`
	ServiceName     string    `gorm:"type:varchar(32);not null"`
	Type            string    `gorm:"type:enum('free','balance');not null"` // free:免费额度, balance:余额扣费
	Amount          float64   `gorm:"type:decimal(10,4);default:0.0000"`
	Count           int       `gorm:"default:1"`
	CreatedAt       time.Time `gorm:"autoCreateTime;index:idx_user_id_date,priority:2"`
}

// TableName 指定表名
func (BillingRecord) TableName() string {
	return "billing_record"
}
