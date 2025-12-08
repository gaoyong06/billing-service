package biz

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// UserBalance 账户余额领域对象
type UserBalance struct {
	UID       string
	Balance   float64
	UpdatedAt time.Time
}

// UserBalanceRepo 余额数据层接口（定义在 biz 层）
type UserBalanceRepo interface {
	GetUserBalance(ctx context.Context, userID string) (*UserBalance, error)
	Recharge(ctx context.Context, userID string, amount float64) error
}

// UserBalanceUseCase 余额业务逻辑
type UserBalanceUseCase struct {
	repo UserBalanceRepo
	log  *log.Helper
}

// NewUserBalanceUseCase 创建余额 UseCase
func NewUserBalanceUseCase(repo UserBalanceRepo, logger log.Logger) *UserBalanceUseCase {
	return &UserBalanceUseCase{
		repo: repo,
		log:  log.NewHelper(logger),
	}
}

// GetBalance 获取余额
func (uc *UserBalanceUseCase) GetBalance(ctx context.Context, userID string) (*UserBalance, error) {
	return uc.repo.GetUserBalance(ctx, userID)
}

// Recharge 充值
func (uc *UserBalanceUseCase) Recharge(ctx context.Context, userID string, amount float64) error {
	return uc.repo.Recharge(ctx, userID, amount)
}
