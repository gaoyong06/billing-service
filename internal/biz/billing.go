package biz

import (
	"context"
	"fmt"
	"time"

	"billing-service/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// ProviderSet is biz providers.
var ProviderSet = wire.NewSet(
	NewBillingConfig,
	NewBillingUseCase,
)

// UserBalance 业务对象
type UserBalance struct {
	UserID    string
	Balance   float64
	Version   int
	UpdatedAt time.Time
}

// FreeQuota 业务对象
type FreeQuota struct {
	UserID      string
	ServiceName string
	TotalQuota  int
	UsedQuota   int
	ResetMonth  string
}

// BillingRecord 业务对象
type BillingRecord struct {
	ID          string
	UserID      string
	ServiceName string
	Type        int
	Amount      float64
	Count       int
	CreatedAt   time.Time
}

// BillingRepo 定义数据层接口
type BillingRepo interface {
	// 余额相关
	GetUserBalance(ctx context.Context, userID string) (*UserBalance, error)
	UpdateBalance(ctx context.Context, userID string, amount float64, version int) error // 乐观锁更新
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

	// 订单相关
	SaveRechargeOrder(ctx context.Context, orderID, userID string, amount float64) error
	GetRechargeOrder(ctx context.Context, orderID string) (string, error) // 返回 userID

	// 重置相关
	GetAllUserIDs(ctx context.Context) ([]string, error) // 获取所有用户ID
}

// BillingUseCase 业务逻辑
type BillingUseCase struct {
	repo BillingRepo
	log  *log.Helper
	conf *BillingConfig
}

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

func NewBillingUseCase(repo BillingRepo, logger log.Logger, conf *BillingConfig) *BillingUseCase {
	return &BillingUseCase{
		repo: repo,
		log:  log.NewHelper(logger),
		conf: conf,
	}
}

// GetAccount 获取账户信息
func (uc *BillingUseCase) GetAccount(ctx context.Context, userID string) (*UserBalance, []*FreeQuota, error) {
	balance, err := uc.repo.GetUserBalance(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if balance == nil {
		balance = &UserBalance{UserID: userID, Balance: 0}
	}

	month := time.Now().Format("2006-01")
	var quotas []*FreeQuota
	for service := range uc.conf.FreeQuotas {
		q, err := uc.repo.GetFreeQuota(ctx, userID, service, month)
		if err != nil {
			continue // 忽略错误或记录日志
		}
		if q == nil {
			q = &FreeQuota{
				UserID:      userID,
				ServiceName: service,
				TotalQuota:  int(uc.conf.FreeQuotas[service]),
				UsedQuota:   0,
				ResetMonth:  month,
			}
		}
		quotas = append(quotas, q)
	}

	return balance, quotas, nil
}

// CheckQuota 检查配额
func (uc *BillingUseCase) CheckQuota(ctx context.Context, userID, serviceName string, count int) (bool, string, error) {
	month := time.Now().Format("2006-01")

	// 1. 检查免费额度
	quota, err := uc.repo.GetFreeQuota(ctx, userID, serviceName, month)
	if err != nil {
		return false, "", err
	}

	// 如果没有记录，视为有额度（将在扣减时创建）
	if quota == nil || quota.TotalQuota-quota.UsedQuota >= count {
		return true, "free", nil
	}

	// 2. 检查余额
	balance, err := uc.repo.GetUserBalance(ctx, userID)
	if err != nil {
		return false, "", err
	}
	if balance == nil {
		return false, "insufficient balance", nil
	}

	price, ok := uc.conf.Prices[serviceName]
	if !ok {
		return false, "unknown service", nil
	}

	cost := price * float64(count)
	if balance.Balance >= cost {
		return true, "balance", nil
	}

	return false, "insufficient balance", nil
}

// DeductQuota 扣减配额
func (uc *BillingUseCase) DeductQuota(ctx context.Context, userID, serviceName string, count int) (string, error) {
	price := uc.conf.Prices[serviceName]
	cost := price * float64(count)
	month := time.Now().Format("2006-01")

	return uc.repo.DeductQuota(ctx, userID, serviceName, count, cost, month)
}

// ListRecords 获取消费记录
func (uc *BillingUseCase) ListRecords(ctx context.Context, userID string, page, pageSize int) ([]*BillingRecord, int64, error) {
	return uc.repo.ListBillingRecords(ctx, userID, page, pageSize)
}

// Recharge 充值
func (uc *BillingUseCase) Recharge(ctx context.Context, userID string, amount float64) (string, string, error) {
	// TODO: 调用 Payment Service 创建订单
	// 这里仅模拟，实际应该调用 Payment Service 的 gRPC 接口
	orderID := "order_" + userID + "_" + time.Now().Format("20060102150405")
	payURL := "https://mock.payment.url?order_id=" + orderID

	// 保存订单信息到 Redis（用于回调时查询 userID）
	if err := uc.repo.SaveRechargeOrder(ctx, orderID, userID, amount); err != nil {
		uc.log.Errorf("SaveRechargeOrder failed: %v", err)
		// 不返回错误，因为订单可能已经创建成功
	}

	return orderID, payURL, nil
}

// RechargeCallback 充值回调
func (uc *BillingUseCase) RechargeCallback(ctx context.Context, orderID string, amount float64) error {
	// 从 Redis 获取订单对应的 userID
	userID, err := uc.repo.GetRechargeOrder(ctx, orderID)
	if err != nil {
		uc.log.Errorf("GetRechargeOrder failed: %v", err)
		return err
	}
	if userID == "" {
		return fmt.Errorf("order not found: %s", orderID)
	}

	return uc.repo.Recharge(ctx, userID, amount)
}

// ResetFreeQuotas 重置所有用户的免费额度（每月1日执行）
// 为所有用户创建下个月的免费额度记录
func (uc *BillingUseCase) ResetFreeQuotas(ctx context.Context) (int, []string, error) {
	// 获取下个月
	nextMonth := time.Now().AddDate(0, 1, 0).Format("2006-01")
	
	// 获取所有用户ID
	userIDs, err := uc.repo.GetAllUserIDs(ctx)
	if err != nil {
		return 0, nil, fmt.Errorf("get all user IDs failed: %w", err)
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
			existing, err := uc.repo.GetFreeQuota(ctx, userID, serviceName, nextMonth)
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
				UserID:      userID,
				ServiceName: serviceName,
				TotalQuota:  int(totalQuota),
				UsedQuota:   0,
				ResetMonth:  nextMonth,
			}

			if err := uc.repo.CreateFreeQuota(ctx, quota); err != nil {
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
