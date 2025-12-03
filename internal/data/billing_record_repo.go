package data

import (
	"context"

	"billing-service/internal/biz"
	"billing-service/internal/data/model"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
)

// billingRecordRepo 消费记录相关数据访问
type billingRecordRepo struct {
	data *Data
	log  *log.Helper
}

// NewBillingRecordRepo 创建消费记录 repo（返回 biz.BillingRecordRepo 接口）
func NewBillingRecordRepo(data *Data, logger log.Logger) biz.BillingRecordRepo {
	return &billingRecordRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// CreateBillingRecord 创建消费记录
func (r *billingRecordRepo) CreateBillingRecord(ctx context.Context, record *biz.BillingRecord) error {
	m := model.BillingRecord{
		BillingRecordID: uuid.New().String(),
		UserID:          record.UserID,
		ServiceName:     record.ServiceName,
		Type:            record.Type,
		Amount:          record.Amount,
		Count:           record.Count,
	}
	return r.data.db.WithContext(ctx).Create(&m).Error
}

// ListBillingRecords 获取消费流水列表
func (r *billingRecordRepo) ListBillingRecords(ctx context.Context, userID string, page, pageSize int) ([]*biz.BillingRecord, int64, error) {
	var models []model.BillingRecord
	var total int64

	offset := (page - 1) * pageSize
	db := r.data.db.WithContext(ctx).Model(&model.BillingRecord{}).Where("user_id = ?", userID)

	if err := db.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	if err := db.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&models).Error; err != nil {
		return nil, 0, err
	}

	var records []*biz.BillingRecord
	for _, m := range models {
		records = append(records, &biz.BillingRecord{
			ID:          m.BillingRecordID,
			UserID:      m.UserID,
			ServiceName: m.ServiceName,
			Type:        m.Type,
			Amount:      m.Amount,
			Count:       m.Count,
			CreatedAt:   m.CreatedAt,
		})
	}
	return records, total, nil
}

