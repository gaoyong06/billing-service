package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

// FreeQuota 免费额度领域对象
type FreeQuota struct {
	UID         string
	ServiceName string
	TotalQuota  int
	UsedQuota   int
	ResetMonth  string
}

// FreeQuotaRepo 免费额度数据层接口（定义在 biz 层）
type FreeQuotaRepo interface {
	GetFreeQuota(ctx context.Context, userID, serviceName, month string) (*FreeQuota, error)
	CreateFreeQuota(ctx context.Context, quota *FreeQuota) error
	UpdateFreeQuota(ctx context.Context, quota *FreeQuota) error
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
func (uc *FreeQuotaUseCase) GetQuota(ctx context.Context, userID, serviceName, month string) (*FreeQuota, error) {
	return uc.repo.GetFreeQuota(ctx, userID, serviceName, month)
}

// CreateQuota 创建免费额度
func (uc *FreeQuotaUseCase) CreateQuota(ctx context.Context, quota *FreeQuota) error {
	return uc.repo.CreateFreeQuota(ctx, quota)
}

// UpdateQuota 更新免费额度
func (uc *FreeQuotaUseCase) UpdateQuota(ctx context.Context, quota *FreeQuota) error {
	return uc.repo.UpdateFreeQuota(ctx, quota)
}
