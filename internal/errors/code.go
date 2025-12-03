package errors

import (
	pkgErrors "github.com/gaoyong06/go-pkg/errors"
	i18nPkg "github.com/gaoyong06/go-pkg/middleware/i18n"
)

func init() {
	// 初始化全局错误管理器（使用项目特定的配置）
	pkgErrors.InitGlobalErrorManager("i18n", i18nPkg.Language)
}

// Billing Service 错误码定义
// 错误码格式：SSMMEE (6位数字)
//   SS: 服务标识，Billing 固定为 19
//   MM: 模块标识，按业务划分
//   EE: 模块内错误序号
//
// 模块划分：
//   00: 通用模块（复用 go-pkg 通用错误码）
//   01: 余额模块
//   02: 配额模块
//   03: 充值模块
//   04: 扣费模块
//   05: 订单模块
//   06-99: 预留扩展

// 余额模块错误码 (190100-190199)
const (
	// ErrCodeBalanceNotFound 余额记录不存在
	ErrCodeBalanceNotFound = 190101
	// ErrCodeInsufficientBalance 余额不足
	ErrCodeInsufficientBalance = 190102
	// ErrCodeBalanceUpdateFailed 余额更新失败
	ErrCodeBalanceUpdateFailed = 190103
)

// 配额模块错误码 (190200-190299)
const (
	// ErrCodeQuotaNotFound 配额记录不存在
	ErrCodeQuotaNotFound = 190201
	// ErrCodeInsufficientQuota 配额不足
	ErrCodeInsufficientQuota = 190202
	// ErrCodeQuotaCreateFailed 配额创建失败
	ErrCodeQuotaCreateFailed = 190203
	// ErrCodeQuotaUpdateFailed 配额更新失败
	ErrCodeQuotaUpdateFailed = 190204
	// ErrCodeUnknownService 未知的服务名称
	ErrCodeUnknownService = 190205
)

// 充值模块错误码 (190300-190299)
const (
	// ErrCodeRechargeOrderNotFound 充值订单不存在
	ErrCodeRechargeOrderNotFound = 190301
	// ErrCodeRechargeOrderCreateFailed 充值订单创建失败
	ErrCodeRechargeOrderCreateFailed = 190302
	// ErrCodeRechargeOrderUpdateFailed 充值订单更新失败
	ErrCodeRechargeOrderUpdateFailed = 190303
	// ErrCodeRechargeFailed 充值失败
	ErrCodeRechargeFailed = 190304
	// ErrCodeRechargeOrderAlreadyExists 充值订单已存在
	ErrCodeRechargeOrderAlreadyExists = 190305
)

// 扣费模块错误码 (190400-190499)
const (
	// ErrCodeDeductQuotaFailed 扣费失败
	ErrCodeDeductQuotaFailed = 190401
	// ErrCodeDeductLockFailed 获取扣费锁失败
	ErrCodeDeductLockFailed = 190402
)

// 订单模块错误码 (190500-190599)
const (
	// ErrCodePaymentServiceUnavailable 支付服务不可用
	ErrCodePaymentServiceUnavailable = 190501
	// ErrCodePaymentCreateFailed 创建支付订单失败
	ErrCodePaymentCreateFailed = 190502
	// ErrCodeCurrencyRequired 币种必填
	ErrCodeCurrencyRequired = 190503
)

// 统计模块错误码 (190600-190699)
const (
	// ErrCodeGetAllUserIDsFailed 获取所有用户ID失败
	ErrCodeGetAllUserIDsFailed = 190601
	// ErrCodeGetStatsFailed 获取统计失败
	ErrCodeGetStatsFailed = 190602
)

// 通用数据访问错误码 (190700-190799)
const (
	// ErrCodeRechargeOrderGetFailed 获取充值订单失败
	ErrCodeRechargeOrderGetFailed = 190701
	// ErrCodeUserBalanceCreateFailed 创建用户余额失败
	ErrCodeUserBalanceCreateFailed = 190704
	// ErrCodeUserBalanceGetFailed 获取用户余额失败
	ErrCodeUserBalanceGetFailed = 190705
	// ErrCodeUserBalanceUpdateFailed 更新用户余额失败
	ErrCodeUserBalanceUpdateFailed = 190706
	// ErrCodePaymentServiceConfigNil 支付服务配置为空
	ErrCodePaymentServiceConfigNil = 190707
	// ErrCodePaymentServiceDialFailed 连接支付服务失败
	ErrCodePaymentServiceDialFailed = 190708
	// ErrCodeInvalidUserID 无效的用户ID
	ErrCodeInvalidUserID = 190709
)

