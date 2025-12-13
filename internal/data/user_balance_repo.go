package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"billing-service/internal/biz"
	"billing-service/internal/constants"
	"billing-service/internal/data/model"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// userBalanceRepo 余额相关数据访问
type userBalanceRepo struct {
	data *Data
	log  *log.Helper
}

// NewUserBalanceRepo 创建余额 repo（返回 biz.UserBalanceRepo 接口）
func NewUserBalanceRepo(data *Data, logger log.Logger) biz.UserBalanceRepo {
	return &userBalanceRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// GetUserBalance 获取用户余额
func (r *userBalanceRepo) GetUserBalance(ctx context.Context, userID string) (*biz.UserBalance, error) {
	if userID == "" {
		return nil, fmt.Errorf("userID is required")
	}

	// 先尝试从 Redis 获取
	balanceKey := fmt.Sprintf("%s%s", constants.RedisKeyBalance, userID)
	balanceStr, err := r.data.rdb.Get(ctx, balanceKey).Result()
	if err == nil {
		// 从缓存获取成功
		var balance float64
		if _, err := fmt.Sscanf(balanceStr, "%f", &balance); err == nil {
			return &biz.UserBalance{
				UID:     userID,
				Balance: balance,
			}, nil
		}
	}

	// 缓存未命中，从数据库查询
	var m model.UserBalance
	if err := r.data.db.WithContext(ctx).Where("uid = ?", userID).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 用户不存在，返回 nil 而不是错误（业务层会处理为余额 0）
			return nil, nil
		}
		r.log.Errorf("GetUserBalance failed: userID=%s, error=%v", userID, err)
		return nil, fmt.Errorf("failed to query user balance from database: %w", err)
	}

	result := &biz.UserBalance{
		UID:       m.UID,
		Balance:   m.Balance,
		UpdatedAt: m.UpdatedAt,
	}

	// 更新缓存（异步，不阻塞，设置超时避免长时间等待）
	go func() {
		cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cacheCancel()
		if err := r.data.rdb.Set(cacheCtx, balanceKey, fmt.Sprintf("%.2f", m.Balance), 5*time.Minute).Err(); err != nil {
			// 缓存更新失败不影响主流程，只记录日志（异步操作，使用默认 logger）
			// 注意：这里不能使用 r.log，因为是在 goroutine 中
		}
	}()

	return result, nil
}

// Recharge 充值（简单逻辑：如果不存在则创建，存在则增加）
func (r *userBalanceRepo) Recharge(ctx context.Context, userID string, amount float64) error {
	return r.data.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m model.UserBalance
		if err := tx.Where("uid = ?", userID).First(&m).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				m = model.UserBalance{
					UserBalanceID: uuid.New().String(),
					UID:           userID,
					Balance:       amount,
				}
				return tx.Create(&m).Error
			}
			return err
		}
		if err := tx.Model(&m).Update("balance", gorm.Expr("balance + ?", amount)).Error; err != nil {
			return err
		}
		// 更新 Redis 缓存（设置超时避免阻塞）
		balanceKey := fmt.Sprintf("%s%s", constants.RedisKeyBalance, userID)
		newBalance := m.Balance + amount
		cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cacheCancel()
		if err := r.data.rdb.Set(cacheCtx, balanceKey, fmt.Sprintf("%.2f", newBalance), 5*time.Minute).Err(); err != nil {
			// 缓存更新失败不影响主流程，只记录日志
			r.log.Warnf("failed to update balance cache in Recharge: %v", err)
		}
		return nil
	})
}
