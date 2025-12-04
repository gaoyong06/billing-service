package biz

import (
	"billing-service/internal/conf"
)

// BillingConfig 计费配置
type BillingConfig struct {
	Prices                   map[string]float64
	FreeQuotas               map[string]int32
	BalanceLowThreshold      float64 // 余额低阈值（单位：元）
	QuotaLowPercentThreshold float64 // 配额低阈值（百分比）
}

// NewBillingConfig 从配置创建 BillingConfig
func NewBillingConfig(c *conf.Bootstrap) *BillingConfig {
	config := &BillingConfig{
		Prices:                   make(map[string]float64),
		FreeQuotas:               make(map[string]int32),
		BalanceLowThreshold:      10.0,  // 默认值
		QuotaLowPercentThreshold: 20.0,  // 默认值
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
	}
	return config
}

