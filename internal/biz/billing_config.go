package biz

import (
	"billing-service/internal/conf"
)

// BillingConfig 计费配置
type BillingConfig struct {
	Prices                   map[string]float64
	FreeQuotas               map[string]int32
	BalanceLowThreshold      float64  // 余额低阈值（单位：元）
	QuotaLowPercentThreshold float64  // 配额低阈值（百分比）
	DevMode                  bool     // 开发模式开关，开启后跳过余额检查
	FreeAppIDs               []string // 免费应用白名单（app_id 列表），这些应用的所有调用都不扣费
}

// NewBillingConfig 从配置创建 BillingConfig
func NewBillingConfig(c *conf.Bootstrap) *BillingConfig {
	config := &BillingConfig{
		Prices:                   make(map[string]float64),
		FreeQuotas:               make(map[string]int32),
		BalanceLowThreshold:      10.0, // 默认值
		QuotaLowPercentThreshold: 20.0, // 默认值
		DevMode:                  true, // 开发模式默认开启
	}
	if c.Billing != nil {
		for k, v := range c.Billing.Prices {
			config.Prices[k] = v
		}
		for k, v := range c.Billing.FreeQuotas {
			config.FreeQuotas[k] = v
		}
		// 从配置读取阈值，如果未配置则使用默认值
		if c.Billing.BalanceLowThreshold > 0 {
			config.BalanceLowThreshold = c.Billing.BalanceLowThreshold
		}
		if c.Billing.QuotaLowPercentThreshold > 0 {
			config.QuotaLowPercentThreshold = c.Billing.QuotaLowPercentThreshold
		}
		// 从配置读取开发模式开关，如果未配置则使用默认值 true
		config.DevMode = c.Billing.DevMode
		// 从配置读取免费应用白名单
		if c.Billing.GetFreeAppIds() != nil {
			config.FreeAppIDs = c.Billing.GetFreeAppIds()
		}
	}
	return config
}

// IsFreeApp 检查 app_id 是否在免费应用白名单中
func (c *BillingConfig) IsFreeApp(appID string) bool {
	if c.FreeAppIDs == nil || len(c.FreeAppIDs) == 0 {
		return false
	}
	for _, freeAppID := range c.FreeAppIDs {
		if freeAppID == appID {
			return true
		}
	}
	return false
}
