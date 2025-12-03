package biz

import (
	"billing-service/internal/conf"
)

// BillingConfig 计费配置
type BillingConfig struct {
	Prices     map[string]float64
	FreeQuotas map[string]int32
}

// NewBillingConfig 从配置创建 BillingConfig
func NewBillingConfig(c *conf.Bootstrap) *BillingConfig {
	config := &BillingConfig{
		Prices:     make(map[string]float64),
		FreeQuotas: make(map[string]int32),
	}
	if c.Billing != nil {
		for k, v := range c.Billing.Prices {
			config.Prices[k] = v
		}
		for k, v := range c.Billing.FreeQuotas {
			config.FreeQuotas[k] = v
		}
	}
	return config
}

