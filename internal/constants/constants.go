package constants

// 时间格式常量
const (
	// TimeFormatMonth 月份格式 (YYYY-MM)
	TimeFormatMonth = "2006-01"
)

// Redis Key 前缀常量
const (
	// RedisKeyBalance 余额缓存 key 前缀
	RedisKeyBalance = "balance:"
	// RedisKeyQuota 配额缓存 key 前缀
	RedisKeyQuota = "quota:"
	// RedisKeyDeductLock 扣费锁 key 前缀
	RedisKeyDeductLock = "deduct:lock:"
	// RedisKeyRechargeOrder 充值订单 key 前缀
	RedisKeyRechargeOrder = "recharge:order:"
)

// 计费类型常量
const (
	// BillingTypeFree 免费额度扣费
	BillingTypeFree = "free"
	// BillingTypeBalance 余额扣费
	BillingTypeBalance = "balance"
)

// 计费类型消息常量
const (
	// BillingMessageFree 使用免费额度
	BillingMessageFree = "free"
	// BillingMessageBalance 使用余额
	BillingMessageBalance = "balance"
	// BillingMessageInsufficientBalance 余额不足
	BillingMessageInsufficientBalance = "insufficient balance"
)

// 订单状态常量
const (
	// OrderStatusPending 待处理
	OrderStatusPending = "pending"
	// OrderStatusSuccess 成功
	OrderStatusSuccess = "success"
	// OrderStatusFailed 失败
	OrderStatusFailed = "failed"
)

// 支付状态常量（用于支付回调）
const (
	// PaymentStatusSuccess 支付成功
	PaymentStatusSuccess = "SUCCESS"
)

// 支付方式常量
const (
	// PaymentMethodAlipay 支付宝
	PaymentMethodAlipay = "alipay"
	// PaymentMethodWechat 微信支付
	PaymentMethodWechat = "wechatpay"
)

// 配额检查结果常量
const (
	// QuotaCheckResultAllowed 允许
	QuotaCheckResultAllowed = "allowed"
	// QuotaCheckResultDenied 拒绝
	QuotaCheckResultDenied = "denied"
	// QuotaCheckResultError 错误
	QuotaCheckResultError = "error"
)

// 扣费类型常量（用于指标）
const (
	// DeductTypeMixed 混合扣费
	DeductTypeMixed = "mixed"
)

// 统计周期常量
const (
	// StatsPeriodToday 今日
	StatsPeriodToday = "today"
	// StatsPeriodMonth 本月
	StatsPeriodMonth = "month"
)

// 订单ID前缀常量
const (
	// OrderIDPrefixRecharge 充值订单ID前缀
	OrderIDPrefixRecharge = "recharge_"
)

// 支付来源常量（用于 payment-service）
const (
	// PaymentSourceBilling 充值来源
	PaymentSourceBilling = "billing"
	// PaymentSourceSubscription 订阅来源
	PaymentSourceSubscription = "subscription"
)
