package model

import (
	"time"
)

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

