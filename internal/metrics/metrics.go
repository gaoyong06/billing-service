package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// BillingMetrics 计费服务指标
type BillingMetrics struct {
	// 配额检查相关指标
	QuotaCheckTotal     *prometheus.CounterVec   // 配额检查总数（按服务、结果）
	QuotaCheckDuration  *prometheus.HistogramVec // 配额检查耗时

	// 扣费相关指标
	DeductQuotaTotal    *prometheus.CounterVec   // 扣费总数（按服务、类型）
	DeductQuotaDuration *prometheus.HistogramVec // 扣费耗时
	DeductQuotaAmount   *prometheus.CounterVec   // 扣费金额（按服务、类型）

	// 余额相关指标
	BalanceQueryTotal   prometheus.Counter   // 余额查询总数
	BalanceUpdateTotal  *prometheus.CounterVec // 余额更新总数（按操作类型）
	BalanceLowAlert     prometheus.Gauge     // 余额不足告警（余额 < 阈值）

	// 配额相关指标
	QuotaQueryTotal     prometheus.Counter   // 配额查询总数
	QuotaLowAlert       *prometheus.GaugeVec // 配额即将用尽告警（剩余配额 < 20%）

	// 充值相关指标
	RechargeTotal       *prometheus.CounterVec // 充值总数（按状态）
	RechargeAmount      *prometheus.CounterVec // 充值金额（按状态）
	RechargeDuration    *prometheus.HistogramVec // 充值耗时
	RechargeFailedTotal prometheus.Counter   // 充值失败总数

	// 订单相关指标
	RechargeOrderTotal  *prometheus.CounterVec // 充值订单总数（按状态）
	RechargeOrderCreateDuration prometheus.Histogram // 订单创建耗时

	// 分布式锁相关指标
	LockAcquireTotal    *prometheus.CounterVec // 锁获取总数（按结果）
	LockAcquireDuration prometheus.Histogram  // 锁获取耗时
}

// NewBillingMetrics 创建计费服务指标
func NewBillingMetrics() *BillingMetrics {
	return &BillingMetrics{
		// 配额检查指标
		QuotaCheckTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "billing_quota_check_total",
				Help: "Total number of quota checks",
			},
			[]string{"service", "result"}, // result: allowed/denied
		),
		QuotaCheckDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "billing_quota_check_duration_seconds",
				Help:    "Duration of quota check operations",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"service"},
		),

		// 扣费指标
		DeductQuotaTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "billing_deduct_quota_total",
				Help: "Total number of quota deductions",
			},
			[]string{"service", "type"}, // type: free/balance/mixed
		),
		DeductQuotaDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "billing_deduct_quota_duration_seconds",
				Help:    "Duration of quota deduction operations",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"service"},
		),
		DeductQuotaAmount: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "billing_deduct_quota_amount_total",
				Help: "Total amount deducted",
			},
			[]string{"service", "type"}, // type: free/balance
		),

		// 余额指标
		BalanceQueryTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "billing_balance_query_total",
				Help: "Total number of balance queries",
			},
		),
		BalanceUpdateTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "billing_balance_update_total",
				Help: "Total number of balance updates",
			},
			[]string{"operation"}, // operation: recharge/deduct
		),
		BalanceLowAlert: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "billing_balance_low_alert",
				Help: "Number of users with low balance (< threshold)",
			},
		),

		// 配额指标
		QuotaQueryTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "billing_quota_query_total",
				Help: "Total number of quota queries",
			},
		),
		QuotaLowAlert: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "billing_quota_low_alert",
				Help: "Number of users with low quota (< 20% remaining)",
			},
			[]string{"service"},
		),

		// 充值指标
		RechargeTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "billing_recharge_total",
				Help: "Total number of recharge operations",
			},
			[]string{"status"}, // status: success/failed
		),
		RechargeAmount: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "billing_recharge_amount_total",
				Help: "Total amount recharged",
			},
			[]string{"status"},
		),
		RechargeDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "billing_recharge_duration_seconds",
				Help:    "Duration of recharge operations",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"operation"}, // operation: create/callback
		),
		RechargeFailedTotal: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "billing_recharge_failed_total",
				Help: "Total number of failed recharge operations",
			},
		),

		// 订单指标
		RechargeOrderTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "billing_recharge_order_total",
				Help: "Total number of recharge orders",
			},
			[]string{"status"}, // status: pending/success/failed
		),
		RechargeOrderCreateDuration: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "billing_recharge_order_create_duration_seconds",
				Help:    "Duration of recharge order creation",
				Buckets: prometheus.DefBuckets,
			},
		),

		// 分布式锁指标
		LockAcquireTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "billing_lock_acquire_total",
				Help: "Total number of lock acquisition attempts",
			},
			[]string{"result"}, // result: success/failed
		),
		LockAcquireDuration: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "billing_lock_acquire_duration_seconds",
				Help:    "Duration of lock acquisition",
				Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0}, // 毫秒级
			},
		),
	}
}

// 全局指标实例
var defaultMetrics *BillingMetrics

// InitMetrics 初始化全局指标
func InitMetrics() {
	defaultMetrics = NewBillingMetrics()
}

// GetMetrics 获取全局指标实例
func GetMetrics() *BillingMetrics {
	if defaultMetrics == nil {
		InitMetrics()
	}
	return defaultMetrics
}

