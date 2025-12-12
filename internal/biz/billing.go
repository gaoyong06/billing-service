package biz

import (
	"context"
	"time"

	"billing-service/internal/constants"
	billingErrors "billing-service/internal/errors"
	"billing-service/internal/metrics"

	pkgErrors "github.com/gaoyong06/go-pkg/errors"
	"github.com/go-kratos/kratos/v2/log"
)

// BillingRepo 统一数据层接口（用于跨领域事务）
// 包含所有领域的方法，主要用于 DeductQuota 等跨领域事务操作
type BillingRepo interface {
	// 余额相关
	GetUserBalance(ctx context.Context, userID string) (*UserBalance, error)
	Recharge(ctx context.Context, userID string, amount float64) error

	// 配额相关
	GetFreeQuota(ctx context.Context, userID, serviceName, month string) (*FreeQuota, error)
	CreateFreeQuota(ctx context.Context, quota *FreeQuota) error
	UpdateFreeQuota(ctx context.Context, quota *FreeQuota) error

	// 记录相关
	CreateBillingRecord(ctx context.Context, record *BillingRecord) error
	ListBillingRecords(ctx context.Context, userID string, page, pageSize int) ([]*BillingRecord, int64, error)

	// 事务操作
	DeductQuota(ctx context.Context, userID, serviceName string, count int, cost float64, month string) (string, error)
	BatchDeductQuota(ctx context.Context, events []*DeductEvent) error

	// 订单相关（幂等性保证）
	CreateRechargeOrder(ctx context.Context, orderID, userID string, amount float64) error
	GetRechargeOrderByID(ctx context.Context, orderID string) (*RechargeOrder, error)
	GetRechargeOrderByPaymentID(ctx context.Context, paymentID string) (*RechargeOrder, error)
	UpdateRechargeOrderStatus(ctx context.Context, orderID, paymentID, status string) error
	RechargeWithIdempotency(ctx context.Context, orderID, paymentID string, amount float64) error

	// 重置相关
	GetAllUserIDs(ctx context.Context) ([]string, error)

	// 统计相关
	GetStatsToday(ctx context.Context, userID, serviceName string) (*Stats, error)
	GetStatsMonth(ctx context.Context, userID, serviceName string) (*Stats, error)
	GetStatsSummary(ctx context.Context, userID string) (*StatsSummary, error)
}

// BillingUseCase 计费业务逻辑（组合 UseCase）
// 负责协调各个领域 UseCase，处理跨领域的业务逻辑
type BillingUseCase struct {
	userBalanceUseCase   *UserBalanceUseCase
	freeQuotaUseCase     *FreeQuotaUseCase
	billingRecordUseCase *BillingRecordUseCase
	rechargeOrderUseCase *RechargeOrderUseCase
	statsUseCase         *StatsUseCase

	repo    BillingRepo // 用于跨领域事务
	conf    *BillingConfig
	log     *log.Helper
	metrics *metrics.BillingMetrics
}

// NewBillingUseCase 创建计费 UseCase
func NewBillingUseCase(
	userBalanceUseCase *UserBalanceUseCase,
	freeQuotaUseCase *FreeQuotaUseCase,
	billingRecordUseCase *BillingRecordUseCase,
	rechargeOrderUseCase *RechargeOrderUseCase,
	statsUseCase *StatsUseCase,
	repo BillingRepo,
	conf *BillingConfig,
	logger log.Logger,
) *BillingUseCase {
	return &BillingUseCase{
		userBalanceUseCase:   userBalanceUseCase,
		freeQuotaUseCase:     freeQuotaUseCase,
		billingRecordUseCase: billingRecordUseCase,
		rechargeOrderUseCase: rechargeOrderUseCase,
		statsUseCase:         statsUseCase,
		repo:                 repo,
		conf:                 conf,
		log:                  log.NewHelper(logger),
		metrics:              metrics.GetMetrics(),
	}
}

// getOrCreateQuota 获取或创建配额记录（如果不存在则创建）
// 用于确保用户在当前月份有配额记录
func (uc *BillingUseCase) getOrCreateQuota(ctx context.Context, userID, serviceName, month string) (*FreeQuota, error) {
	// 先尝试获取配额记录
	quota, err := uc.freeQuotaUseCase.GetQuota(ctx, userID, serviceName, month)
	if err != nil {
		return nil, err
	}

	// 如果记录存在，直接返回
	if quota != nil {
		return quota, nil
	}

	// 记录不存在，检查配置中是否有该服务
	totalQuota, ok := uc.conf.FreeQuotas[serviceName]
	if !ok {
		// 配置中没有该服务，返回 nil（不创建记录）
		return nil, nil
	}

	// 创建并保存配额记录
	quota = &FreeQuota{
		UID:         userID,
		ServiceName: serviceName,
		TotalQuota:  int(totalQuota),
		UsedQuota:   0,
		ResetMonth:  month,
	}
	if err := uc.freeQuotaUseCase.CreateQuota(ctx, quota); err != nil {
		// 创建失败可能是并发导致的重复创建，尝试重新获取
		quota, err = uc.freeQuotaUseCase.GetQuota(ctx, userID, serviceName, month)
		if err != nil {
			return nil, err
		}
		if quota == nil {
			// 重新获取后仍然为 nil，说明创建失败且无法获取
			uc.log.Warnf("Failed to create/get quota for user=%s, service=%s, month=%s", userID, serviceName, month)
			return nil, nil
		}
	}

	return quota, nil
}

// GetAccount 获取账户信息（组合多个领域）
func (uc *BillingUseCase) GetAccount(ctx context.Context, userID string) (*UserBalance, []*FreeQuota, error) {
	balance, err := uc.userBalanceUseCase.GetBalance(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if balance == nil {
		balance = &UserBalance{UID: userID, Balance: 0}
	}

	month := time.Now().Format(constants.TimeFormatMonth)
	var quotas []*FreeQuota
	for service := range uc.conf.FreeQuotas {
		q, err := uc.getOrCreateQuota(ctx, userID, service, month)
		if err != nil {
			uc.log.Warnf("Failed to get or create quota for user=%s, service=%s, month=%s: %v", userID, service, month, err)
			continue // 忽略错误，继续处理其他服务
		}
		if q == nil {
			// 配置中没有该服务或创建失败，跳过
			continue
		}
		quotas = append(quotas, q)
	}

	return balance, quotas, nil
}

// CheckQuota 检查配额（跨领域逻辑）
func (uc *BillingUseCase) CheckQuota(ctx context.Context, userID, serviceName string, count int) (bool, string, error) {
	startTime := time.Now()
	defer func() {
		// 记录配额检查耗时
		if uc.metrics != nil {
			uc.metrics.QuotaCheckDuration.WithLabelValues(serviceName).Observe(time.Since(startTime).Seconds())
		}
	}()

	month := time.Now().Format(constants.TimeFormatMonth)

	// 1. 检查免费额度（如果不存在则自动创建）
	quota, err := uc.getOrCreateQuota(ctx, userID, serviceName, month)
	if err != nil {
		if uc.metrics != nil {
			uc.metrics.QuotaCheckTotal.WithLabelValues(serviceName, constants.QuotaCheckResultError).Inc()
		}
		return false, "", err
	}

	// 如果配额记录不存在且无法创建，说明配置中没有该服务
	if quota == nil {
		return false, "", pkgErrors.NewBizErrorWithLang(ctx, billingErrors.ErrCodeUnknownService)
	}

	// 检查免费额度是否充足
	if quota.TotalQuota-quota.UsedQuota >= count {
		// 记录配额检查成功（使用免费额度）
		if uc.metrics != nil {
			uc.metrics.QuotaCheckTotal.WithLabelValues(serviceName, constants.QuotaCheckResultAllowed).Inc()
			// 检查配额是否即将用尽（剩余 < 阈值）
			remainingPercent := float64(quota.TotalQuota-quota.UsedQuota) / float64(quota.TotalQuota) * 100
			if remainingPercent < uc.conf.QuotaLowPercentThreshold {
				uc.metrics.QuotaLowAlert.WithLabelValues(serviceName).Set(1)
			} else {
				uc.metrics.QuotaLowAlert.WithLabelValues(serviceName).Set(0)
			}
		}
		return true, constants.BillingMessageFree, nil
	}

	// 2. 检查余额
	balance, err := uc.userBalanceUseCase.GetBalance(ctx, userID)
	if err != nil {
		return false, "", err
	}

	// 如果余额记录不存在，自动创建（初始余额为 0）
	if balance == nil {
		balance = &UserBalance{
			UID:     userID,
			Balance: 0,
		}
		// 注意：这里不创建记录，只是用于检查
		// 实际创建会在 DeductQuota 或 Recharge 时进行
	}

	price, ok := uc.conf.Prices[serviceName]
	if !ok {
		return false, "", pkgErrors.NewBizErrorWithLang(ctx, billingErrors.ErrCodeUnknownService)
	}

	cost := price * float64(count)
	if balance.Balance >= cost {
		// 记录配额检查成功（使用余额）
		if uc.metrics != nil {
			uc.metrics.QuotaCheckTotal.WithLabelValues(serviceName, constants.QuotaCheckResultAllowed).Inc()
			// 检查余额是否不足（余额 < 阈值）
			if balance.Balance < uc.conf.BalanceLowThreshold {
				uc.metrics.BalanceLowAlert.Set(1)
			} else {
				uc.metrics.BalanceLowAlert.Set(0)
			}
		}
		return true, constants.BillingMessageBalance, nil
	}

	// 记录配额检查失败（余额不足）
	if uc.metrics != nil {
		uc.metrics.QuotaCheckTotal.WithLabelValues(serviceName, constants.QuotaCheckResultDenied).Inc()
		uc.metrics.BalanceLowAlert.Set(1) // 余额不足告警
	}
	return false, constants.BillingMessageInsufficientBalance, nil
}

// DeductQuota 扣减配额（跨领域事务）
func (uc *BillingUseCase) DeductQuota(ctx context.Context, userID, serviceName string, count int) (string, error) {
	startTime := time.Now()
	price := uc.conf.Prices[serviceName]
	cost := price * float64(count)
	month := time.Now().Format(constants.TimeFormatMonth)

	recordID, err := uc.repo.DeductQuota(ctx, userID, serviceName, count, cost, month)

	// 记录扣费指标
	if uc.metrics != nil {
		duration := time.Since(startTime).Seconds()
		uc.metrics.DeductQuotaDuration.WithLabelValues(serviceName).Observe(duration)

		if err == nil {
			// 根据扣费类型记录（这里简化处理，实际应该从 repo 返回扣费类型）
			// 由于 DeductQuota 返回的是 recordID，我们需要推断扣费类型
			// 为了简化，这里先记录为 "mixed"，实际应该从业务逻辑中获取
			uc.metrics.DeductQuotaTotal.WithLabelValues(serviceName, constants.DeductTypeMixed).Inc()
			uc.metrics.DeductQuotaAmount.WithLabelValues(serviceName, constants.BillingTypeBalance).Add(cost)
		}
	}

	return recordID, err
}

// ListRecords 获取消费记录
func (uc *BillingUseCase) ListRecords(ctx context.Context, userID string, page, pageSize int) ([]*BillingRecord, int64, error) {
	return uc.billingRecordUseCase.ListRecords(ctx, userID, page, pageSize)
}

// Recharge 充值
func (uc *BillingUseCase) Recharge(ctx context.Context, userID string, amount float64, method int32, currency, returnURL, notifyURL string) (string, string, error) {
	return uc.rechargeOrderUseCase.CreateRecharge(ctx, userID, amount, method, currency, returnURL, notifyURL)
}

// RechargeCallback 充值回调
func (uc *BillingUseCase) RechargeCallback(ctx context.Context, orderID string, amount float64) error {
	return uc.rechargeOrderUseCase.RechargeCallback(ctx, orderID, amount)
}

// ResetFreeQuotas 重置所有用户的免费额度（每月1日执行）
// 为所有用户创建下个月的免费额度记录
func (uc *BillingUseCase) ResetFreeQuotas(ctx context.Context) (int, []string, error) {
	// 获取下个月
	nextMonth := time.Now().AddDate(0, 1, 0).Format(constants.TimeFormatMonth)

	// 获取所有用户ID
	userIDs, err := uc.statsUseCase.GetAllUserIDs(ctx)
	if err != nil {
		return 0, nil, pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodeGetAllUserIDsFailed)
	}

	if len(userIDs) == 0 {
		uc.log.Info("No users found, skip reset")
		return 0, []string{}, nil
	}

	successCount := 0
	successUserIDs := []string{}

	// 为每个用户创建下个月的免费额度
	for _, userID := range userIDs {
		for serviceName, totalQuota := range uc.conf.FreeQuotas {
			// 检查是否已存在下个月的记录
			existing, err := uc.freeQuotaUseCase.GetQuota(ctx, userID, serviceName, nextMonth)
			if err != nil {
				uc.log.Warnf("GetFreeQuota failed for user=%s, service=%s, month=%s: %v",
					userID, serviceName, nextMonth, err)
				continue
			}

			// 如果已存在，跳过
			if existing != nil {
				continue
			}

			// 创建新的免费额度记录
			quota := &FreeQuota{
				UID:         userID,
				ServiceName: serviceName,
				TotalQuota:  int(totalQuota),
				UsedQuota:   0,
				ResetMonth:  nextMonth,
			}

			if err := uc.freeQuotaUseCase.CreateQuota(ctx, quota); err != nil {
				uc.log.Warnf("CreateFreeQuota failed for user=%s, service=%s, month=%s: %v",
					userID, serviceName, nextMonth, err)
				continue
			}

			successCount++
			if !contains(successUserIDs, userID) {
				successUserIDs = append(successUserIDs, userID)
			}
		}
	}

	uc.log.Infof("Reset free quotas completed: nextMonth=%s, totalUsers=%d, successUsers=%d",
		nextMonth, len(userIDs), len(successUserIDs))

	return successCount, successUserIDs, nil
}

// contains 检查字符串切片是否包含指定字符串
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetStatsToday 获取今日调用统计
func (uc *BillingUseCase) GetStatsToday(ctx context.Context, userID, serviceName string) (*Stats, error) {
	return uc.statsUseCase.GetStatsToday(ctx, userID, serviceName)
}

// GetStatsMonth 获取本月调用统计
func (uc *BillingUseCase) GetStatsMonth(ctx context.Context, userID, serviceName string) (*Stats, error) {
	return uc.statsUseCase.GetStatsMonth(ctx, userID, serviceName)
}

// GetStatsSummary 获取汇总统计（所有服务）
func (uc *BillingUseCase) GetStatsSummary(ctx context.Context, userID string) (*StatsSummary, error) {
	return uc.statsUseCase.GetStatsSummary(ctx, userID)
}
