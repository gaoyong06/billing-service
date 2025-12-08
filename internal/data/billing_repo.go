package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"billing-service/internal/biz"
	"billing-service/internal/constants"
	"billing-service/internal/data/model"
	billingErrors "billing-service/internal/errors"
	"billing-service/internal/metrics"

	pkgErrors "github.com/gaoyong06/go-pkg/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-redsync/redsync/v4"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// billingRepo 组合 repo，实现 biz.BillingRepo 接口
type billingRepo struct {
	data              *Data
	log               *log.Helper
	sync              *redsync.Redsync
	metrics           *metrics.BillingMetrics
	userBalanceRepo   biz.UserBalanceRepo
	freeQuotaRepo     biz.FreeQuotaRepo
	billingRecordRepo biz.BillingRecordRepo
	rechargeOrderRepo biz.RechargeOrderRepo
	statsRepo         biz.StatsRepo
}

// NewBillingRepo 创建组合 repo
func NewBillingRepo(
	data *Data,
	sync *redsync.Redsync,
	logger log.Logger,
	userBalanceRepo biz.UserBalanceRepo,
	freeQuotaRepo biz.FreeQuotaRepo,
	billingRecordRepo biz.BillingRecordRepo,
	rechargeOrderRepo biz.RechargeOrderRepo,
	statsRepo biz.StatsRepo,
) biz.BillingRepo {
	return &billingRepo{
		data:              data,
		log:               log.NewHelper(logger),
		sync:              sync,
		metrics:           metrics.GetMetrics(),
		userBalanceRepo:   userBalanceRepo,
		freeQuotaRepo:     freeQuotaRepo,
		billingRecordRepo: billingRecordRepo,
		rechargeOrderRepo: rechargeOrderRepo,
		statsRepo:         statsRepo,
	}
}

// ========== 余额相关 ==========

// GetUserBalance 获取用户余额
func (r *billingRepo) GetUserBalance(ctx context.Context, userID string) (*biz.UserBalance, error) {
	return r.userBalanceRepo.GetUserBalance(ctx, userID)
}

// Recharge 充值
func (r *billingRepo) Recharge(ctx context.Context, userID string, amount float64) error {
	return r.userBalanceRepo.Recharge(ctx, userID, amount)
}

// ========== 免费额度相关 ==========

// GetFreeQuota 获取免费额度
func (r *billingRepo) GetFreeQuota(ctx context.Context, userID, serviceName, month string) (*biz.FreeQuota, error) {
	return r.freeQuotaRepo.GetFreeQuota(ctx, userID, serviceName, month)
}

// CreateFreeQuota 创建免费额度
func (r *billingRepo) CreateFreeQuota(ctx context.Context, quota *biz.FreeQuota) error {
	return r.freeQuotaRepo.CreateFreeQuota(ctx, quota)
}

// UpdateFreeQuota 更新免费额度
func (r *billingRepo) UpdateFreeQuota(ctx context.Context, quota *biz.FreeQuota) error {
	return r.freeQuotaRepo.UpdateFreeQuota(ctx, quota)
}

// ========== 消费记录相关 ==========

// CreateBillingRecord 创建消费记录
func (r *billingRepo) CreateBillingRecord(ctx context.Context, record *biz.BillingRecord) error {
	return r.billingRecordRepo.CreateBillingRecord(ctx, record)
}

// ListBillingRecords 获取消费流水列表
func (r *billingRepo) ListBillingRecords(ctx context.Context, userID string, page, pageSize int) ([]*biz.BillingRecord, int64, error) {
	return r.billingRecordRepo.ListBillingRecords(ctx, userID, page, pageSize)
}

// ========== 事务操作 ==========

// DeductQuota 核心扣费逻辑（事务）
// 支持混合扣费：优先扣除免费额度，不足时扣除余额
// 使用分布式锁防止高并发超扣
func (r *billingRepo) DeductQuota(ctx context.Context, userID, serviceName string, count int, cost float64, month string) (string, error) {
	// 获取分布式锁（按用户+服务+月份）
	lockKey := fmt.Sprintf("%s%s:%s:%s", constants.RedisKeyDeductLock, userID, serviceName, month)
	if r.sync != nil {
		lockStartTime := time.Now()
		mutex := r.sync.NewMutex(lockKey, redsync.WithExpiry(5*time.Second))
		if err := mutex.Lock(); err != nil {
			r.log.Errorf("Failed to acquire lock for deduct quota: user_id=%s, service=%s, error=%v", userID, serviceName, err)
			if r.metrics != nil {
				r.metrics.LockAcquireTotal.WithLabelValues(constants.OrderStatusFailed).Inc()
				r.metrics.LockAcquireDuration.Observe(time.Since(lockStartTime).Seconds())
			}
			return "", pkgErrors.NewBizErrorWithLang(context.Background(), billingErrors.ErrCodeDeductLockFailed)
		}
		if r.metrics != nil {
			r.metrics.LockAcquireTotal.WithLabelValues(constants.OrderStatusSuccess).Inc()
			r.metrics.LockAcquireDuration.Observe(time.Since(lockStartTime).Seconds())
		}
		defer func() {
			if ok, err := mutex.Unlock(); !ok || err != nil {
				r.log.Warnf("Failed to unlock for deduct quota: user_id=%s, service=%s, error=%v", userID, serviceName, err)
			}
		}()
	}

	var recordID string
	var needUpdateQuotaCache bool
	var needUpdateBalanceCache bool
	var quotaRemaining int
	var newBalance float64

	err := r.data.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 检查并扣减免费额度
		var quota model.FreeQuota
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND service_name = ? AND reset_month = ?", userID, serviceName, month).
			First(&quota).Error

		quotaNotFound := errors.Is(err, gorm.ErrRecordNotFound)
		if err != nil && !quotaNotFound {
			return err
		}

		var freeQuotaUsed int
		var balanceDeducted float64
		var balanceCount int

		// 如果有免费额度记录且还有剩余额度
		if !quotaNotFound && quota.TotalQuota > quota.UsedQuota {
			remaining := quota.TotalQuota - quota.UsedQuota
			if remaining >= count {
				// 免费额度充足，全部使用免费额度
				freeQuotaUsed = count
				if err := tx.Model(&quota).Update("used_quota", gorm.Expr("used_quota + ?", count)).Error; err != nil {
					return err
				}
				// 记录需要更新的缓存信息
				needUpdateQuotaCache = true
				quotaRemaining = remaining - count
			} else {
				// 免费额度不足，先扣完免费额度，剩余部分扣余额
				freeQuotaUsed = remaining
				balanceCount = count - remaining
				balanceDeducted = cost * float64(balanceCount) / float64(count) // 按比例计算余额扣费金额

				if err := tx.Model(&quota).Update("used_quota", gorm.Expr("used_quota + ?", remaining)).Error; err != nil {
					return err
				}
				// 记录需要更新的缓存信息
				needUpdateQuotaCache = true
				quotaRemaining = 0
			}
		} else {
			// 没有免费额度或已用完，全部扣余额
			balanceCount = count
			balanceDeducted = cost
		}

		// 2. 如果需要扣余额
		if balanceCount > 0 {
			var balance model.UserBalance
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("user_id = ?", userID).First(&balance).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					// 用户余额记录不存在，自动创建（初始余额为 0）
					balance = model.UserBalance{
						UserBalanceID: uuid.New().String(),
						UID:           userID,
						Balance:       0,
					}
					if err := tx.Create(&balance).Error; err != nil {
						return pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodeBalanceUpdateFailed)
					}
					// 余额为 0，无法扣费
					return pkgErrors.NewBizErrorWithLang(ctx, billingErrors.ErrCodeInsufficientBalance)
				}
				return pkgErrors.WrapErrorWithLang(ctx, err, pkgErrors.ErrCodeDatabaseError)
			}

			if balance.Balance < balanceDeducted {
				return pkgErrors.NewBizErrorWithLang(ctx, billingErrors.ErrCodeInsufficientBalance)
			}

			if err := tx.Model(&balance).Update("balance", gorm.Expr("balance - ?", balanceDeducted)).Error; err != nil {
				return err
			}

			// 记录需要更新的缓存信息
			needUpdateBalanceCache = true
			newBalance = balance.Balance - balanceDeducted
		}

		// 3. 记录流水
		// 如果混合扣费，生成两条记录；否则生成一条记录
		recordID = uuid.New().String()

		// 如果有使用免费额度，创建免费额度记录
		if freeQuotaUsed > 0 {
			freeRecord := model.BillingRecord{
				BillingRecordID: recordID,
				UID:             userID,
				ServiceName:     serviceName,
				Type:            model.BillingTypeFree,
				Amount:          0,
				Count:           freeQuotaUsed,
			}
			if err := tx.Create(&freeRecord).Error; err != nil {
				return err
			}
		}

		// 如果有扣余额，创建余额扣费记录
		if balanceCount > 0 {
			balanceRecordID := recordID
			if freeQuotaUsed > 0 {
				// 混合扣费时，余额记录使用新的ID
				balanceRecordID = uuid.New().String()
			}
			balanceRecord := model.BillingRecord{
				BillingRecordID: balanceRecordID,
				UID:             userID,
				ServiceName:     serviceName,
				Type:            model.BillingTypeBalance,
				Amount:          balanceDeducted,
				Count:           balanceCount,
			}
			if err := tx.Create(&balanceRecord).Error; err != nil {
				return err
			}
			// 返回余额记录的ID（如果混合扣费）
			if freeQuotaUsed > 0 {
				recordID = balanceRecordID
			}
		}

		return nil
	})

	// 事务提交成功后，更新 Redis 缓存（使用传入的 context，但设置较短的超时时间）
	if err == nil {
		// 使用独立的 context 更新缓存，避免阻塞主流程
		// 设置较短的超时时间，如果缓存更新失败不影响主流程
		cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cacheCancel()

		if needUpdateQuotaCache {
			quotaKey := fmt.Sprintf("%s%s:%s:%s", constants.RedisKeyQuota, userID, serviceName, month)
			if err := r.data.rdb.Set(cacheCtx, quotaKey, fmt.Sprintf("%d", quotaRemaining), 5*time.Minute).Err(); err != nil {
				// 缓存更新失败不影响主流程，只记录日志
				r.log.Warnf("failed to update quota cache: %v", err)
			}
		}
		if needUpdateBalanceCache {
			balanceKey := fmt.Sprintf("%s%s", constants.RedisKeyBalance, userID)
			if err := r.data.rdb.Set(cacheCtx, balanceKey, fmt.Sprintf("%.2f", newBalance), 5*time.Minute).Err(); err != nil {
				// 缓存更新失败不影响主流程，只记录日志
				r.log.Warnf("failed to update balance cache: %v", err)
			}
		}
	}

	return recordID, err
}

// ========== 充值订单相关 ==========

// CreateRechargeOrder 创建充值订单记录
func (r *billingRepo) CreateRechargeOrder(ctx context.Context, orderID, userID string, amount float64) error {
	return r.rechargeOrderRepo.CreateRechargeOrder(ctx, orderID, userID, amount)
}

// GetRechargeOrderByID 通过订单ID查询充值订单
func (r *billingRepo) GetRechargeOrderByID(ctx context.Context, orderID string) (*biz.RechargeOrder, error) {
	return r.rechargeOrderRepo.GetRechargeOrderByID(ctx, orderID)
}

// GetRechargeOrderByPaymentID 通过支付流水号查询充值订单
func (r *billingRepo) GetRechargeOrderByPaymentID(ctx context.Context, paymentID string) (*biz.RechargeOrder, error) {
	return r.rechargeOrderRepo.GetRechargeOrderByPaymentID(ctx, paymentID)
}

// UpdateRechargeOrderStatus 更新充值订单状态
func (r *billingRepo) UpdateRechargeOrderStatus(ctx context.Context, orderID, paymentID, status string) error {
	return r.rechargeOrderRepo.UpdateRechargeOrderStatus(ctx, orderID, paymentID, status)
}

// RechargeWithIdempotency 带幂等性保证的充值
func (r *billingRepo) RechargeWithIdempotency(ctx context.Context, orderID, paymentID string, amount float64) error {
	return r.rechargeOrderRepo.RechargeWithIdempotency(ctx, orderID, paymentID, amount)
}

// ========== 统计相关 ==========

// GetAllUserIDs 获取所有用户ID
func (r *billingRepo) GetAllUserIDs(ctx context.Context) ([]string, error) {
	return r.statsRepo.GetAllUserIDs(ctx)
}

// GetStatsToday 获取今日调用统计
func (r *billingRepo) GetStatsToday(ctx context.Context, userID, serviceName string) (*biz.Stats, error) {
	return r.statsRepo.GetStatsToday(ctx, userID, serviceName)
}

// GetStatsMonth 获取本月调用统计
func (r *billingRepo) GetStatsMonth(ctx context.Context, userID, serviceName string) (*biz.Stats, error) {
	return r.statsRepo.GetStatsMonth(ctx, userID, serviceName)
}

// GetStatsSummary 获取汇总统计（所有服务）
func (r *billingRepo) GetStatsSummary(ctx context.Context, userID string) (*biz.StatsSummary, error) {
	return r.statsRepo.GetStatsSummary(ctx, userID)
}
