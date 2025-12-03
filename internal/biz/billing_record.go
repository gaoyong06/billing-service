package biz

import (
	"context"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// BillingRecord 消费记录领域对象
type BillingRecord struct {
	ID          string
	UserID      string
	ServiceName string
	Type        string // "free": 免费额度, "balance": 余额扣费
	Amount      float64
	Count       int
	CreatedAt   time.Time
}

// BillingRecordRepo 消费记录数据层接口（定义在 biz 层）
type BillingRecordRepo interface {
	CreateBillingRecord(ctx context.Context, record *BillingRecord) error
	ListBillingRecords(ctx context.Context, userID string, page, pageSize int) ([]*BillingRecord, int64, error)
}

// BillingRecordUseCase 消费记录业务逻辑
type BillingRecordUseCase struct {
	repo BillingRecordRepo
	log  *log.Helper
}

// NewBillingRecordUseCase 创建消费记录 UseCase
func NewBillingRecordUseCase(repo BillingRecordRepo, logger log.Logger) *BillingRecordUseCase {
	return &BillingRecordUseCase{
		repo: repo,
		log:  log.NewHelper(logger),
	}
}

// CreateRecord 创建消费记录
func (uc *BillingRecordUseCase) CreateRecord(ctx context.Context, record *BillingRecord) error {
	return uc.repo.CreateBillingRecord(ctx, record)
}

// ListRecords 获取消费记录列表
func (uc *BillingRecordUseCase) ListRecords(ctx context.Context, userID string, page, pageSize int) ([]*BillingRecord, int64, error) {
	return uc.repo.ListBillingRecords(ctx, userID, page, pageSize)
}

