package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

// FreeQuota 免费额度领域对象
type FreeQuota struct {
	UserID      string
	AppID       string // 应用ID（用于按应用管理免费额度）
	ServiceName string
	TotalQuota  int
	UsedQuota   int
	ResetMonth  string
}

// FreeQuotaRepo 免费额度数据层接口（定义在 biz 层）
type FreeQuotaRepo interface {
	GetFreeQuota(ctx context.Context, userID, appID, serviceName, month string) (*FreeQuota, error)
	CreateFreeQuota(ctx context.Context, quota *FreeQuota) error
	// IncrementUsedQuota 原子增加已使用配额（用于免费应用记录用量，避免读-改-写竞态）
	IncrementUsedQuota(ctx context.Context, userID, appID, serviceName, month string, count int) error
}

// FreeQuotaUseCase 免费额度业务逻辑
type FreeQuotaUseCase struct {
	repo FreeQuotaRepo
	conf *BillingConfig
	log  *log.Helper
}

// NewFreeQuotaUseCase 创建免费额度 UseCase
func NewFreeQuotaUseCase(repo FreeQuotaRepo, conf *BillingConfig, logger log.Logger) *FreeQuotaUseCase {
	return &FreeQuotaUseCase{
		repo: repo,
		conf: conf,
		log:  log.NewHelper(logger),
	}
}

// GetQuota 获取免费额度
func (uc *FreeQuotaUseCase) GetQuota(ctx context.Context, userID, appID, serviceName, month string) (*FreeQuota, error) {
	return uc.repo.GetFreeQuota(ctx, userID, appID, serviceName, month)
}

// CreateQuota 创建免费额度
func (uc *FreeQuotaUseCase) CreateQuota(ctx context.Context, quota *FreeQuota) error {
	return uc.repo.CreateFreeQuota(ctx, quota)
}

// IncrementUsedQuota 原子增加已使用配额
func (uc *FreeQuotaUseCase) IncrementUsedQuota(ctx context.Context, userID, appID, serviceName, month string, count int) error {
	return uc.repo.IncrementUsedQuota(ctx, userID, appID, serviceName, month, count)
}
