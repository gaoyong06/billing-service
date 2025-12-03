package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

// Stats 统计对象
type Stats struct {
	UserID      string
	ServiceName string
	TotalCount  int     // 总调用次数
	TotalCost   float64 // 总费用（仅余额扣费部分）
	FreeCount   int     // 免费额度使用次数
	PaidCount   int     // 余额扣费次数
	Period      string  // 统计周期：today 或 month
}

// ServiceStats 服务统计对象
type ServiceStats struct {
	ServiceName string
	TotalCount  int
	TotalCost   float64
	FreeCount   int
	PaidCount   int
}

// StatsSummary 汇总统计对象
type StatsSummary struct {
	UserID     string
	TotalCount int
	TotalCost  float64
	Services   []*ServiceStats
}

// StatsRepo 统计数据层接口（定义在 biz 层）
type StatsRepo interface {
	GetAllUserIDs(ctx context.Context) ([]string, error)
	GetStatsToday(ctx context.Context, userID, serviceName string) (*Stats, error)
	GetStatsMonth(ctx context.Context, userID, serviceName string) (*Stats, error)
	GetStatsSummary(ctx context.Context, userID string) (*StatsSummary, error)
}

// StatsUseCase 统计业务逻辑
type StatsUseCase struct {
	repo StatsRepo
	log  *log.Helper
}

// NewStatsUseCase 创建统计 UseCase
func NewStatsUseCase(repo StatsRepo, logger log.Logger) *StatsUseCase {
	return &StatsUseCase{
		repo: repo,
		log:  log.NewHelper(logger),
	}
}

// GetAllUserIDs 获取所有用户ID
func (uc *StatsUseCase) GetAllUserIDs(ctx context.Context) ([]string, error) {
	return uc.repo.GetAllUserIDs(ctx)
}

// GetStatsToday 获取今日统计
func (uc *StatsUseCase) GetStatsToday(ctx context.Context, userID, serviceName string) (*Stats, error) {
	return uc.repo.GetStatsToday(ctx, userID, serviceName)
}

// GetStatsMonth 获取本月统计
func (uc *StatsUseCase) GetStatsMonth(ctx context.Context, userID, serviceName string) (*Stats, error) {
	return uc.repo.GetStatsMonth(ctx, userID, serviceName)
}

// GetStatsSummary 获取汇总统计
func (uc *StatsUseCase) GetStatsSummary(ctx context.Context, userID string) (*StatsSummary, error) {
	return uc.repo.GetStatsSummary(ctx, userID)
}

