package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"billing-service/internal/biz"
	"billing-service/internal/constants"
	"billing-service/internal/data/model"
	"billing-service/internal/metrics"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// freeQuotaRepo 免费额度相关数据访问
type freeQuotaRepo struct {
	data    *Data
	log     *log.Helper
	metrics *metrics.BillingMetrics
}

// NewFreeQuotaRepo 创建免费额度 repo（返回 biz.FreeQuotaRepo 接口）
func NewFreeQuotaRepo(data *Data, logger log.Logger) biz.FreeQuotaRepo {
	return &freeQuotaRepo{
		data:    data,
		log:     log.NewHelper(logger),
		metrics: metrics.GetMetrics(),
	}
}

// GetFreeQuota 获取免费额度
func (r *freeQuotaRepo) GetFreeQuota(ctx context.Context, userID, serviceName, month string) (*biz.FreeQuota, error) {
	// 记录配额查询指标
	if r.metrics != nil {
		r.metrics.QuotaQueryTotal.Inc()
	}

	// 先尝试从 Redis 获取剩余配额
	quotaKey := fmt.Sprintf("%s%s:%s:%s", constants.RedisKeyQuota, userID, serviceName, month)
	remainingStr, err := r.data.rdb.Get(ctx, quotaKey).Result()
	if err == nil {
		// 从缓存获取成功
		var remaining int
		if _, err := fmt.Sscanf(remainingStr, "%d", &remaining); err == nil {
			// 需要从配置获取总额度，这里简化处理，返回缓存值
			// 实际应该从数据库获取完整信息或从配置获取总额度
			// 为了简化，这里仍然查询数据库获取完整信息，但可以优化
		}
	}

	// 从数据库查询完整信息
	var m model.FreeQuota
	if err := r.data.db.WithContext(ctx).
		Where("uid = ? AND service_name = ? AND reset_month = ?", userID, serviceName, month).
		First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	result := &biz.FreeQuota{
		UID:         m.UID,
		ServiceName: m.ServiceName,
		TotalQuota:  m.TotalQuota,
		UsedQuota:   m.UsedQuota,
		ResetMonth:  m.ResetMonth,
	}

	// 更新缓存（异步，不阻塞，设置超时避免长时间等待）
	go func() {
		cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cacheCancel()
		remaining := m.TotalQuota - m.UsedQuota
		if err := r.data.rdb.Set(cacheCtx, quotaKey, fmt.Sprintf("%d", remaining), 5*time.Minute).Err(); err != nil {
			// 缓存更新失败不影响主流程，只记录日志（异步操作，使用默认 logger）
			// 注意：这里不能使用 r.log，因为是在 goroutine 中
		}
	}()

	return result, nil
}

// CreateFreeQuota 创建免费额度
func (r *freeQuotaRepo) CreateFreeQuota(ctx context.Context, quota *biz.FreeQuota) error {
	m := model.FreeQuota{
		FreeQuotaID: uuid.New().String(),
		UID:         quota.UID,
		ServiceName: quota.ServiceName,
		TotalQuota:  quota.TotalQuota,
		UsedQuota:   quota.UsedQuota,
		ResetMonth:  quota.ResetMonth,
	}
	return r.data.db.WithContext(ctx).Create(&m).Error
}

// UpdateFreeQuota 更新免费额度
func (r *freeQuotaRepo) UpdateFreeQuota(ctx context.Context, quota *biz.FreeQuota) error {
	return r.data.db.WithContext(ctx).Model(&model.FreeQuota{}).
		Where("uid = ? AND service_name = ? AND reset_month = ?", quota.UID, quota.ServiceName, quota.ResetMonth).
		Update("used_quota", quota.UsedQuota).Error
}
