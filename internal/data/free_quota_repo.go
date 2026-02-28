package data

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
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

// parseQuotaCache 解析缓存值 "totalQuota,usedQuota"，解析成功返回 (total, used, true)；否则返回 (0, 0, false)
func parseQuotaCache(s string) (total, used int, ok bool) {
	parts := strings.SplitN(strings.TrimSpace(s), ",", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	t, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	u, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	if t < 0 || u < 0 || u > t {
		return 0, 0, false
	}
	return t, u, true
}

// GetFreeQuota 获取免费额度
func (r *freeQuotaRepo) GetFreeQuota(ctx context.Context, userID, appID, serviceName, month string) (*biz.FreeQuota, error) {
	// 记录配额查询指标
	if r.metrics != nil {
		r.metrics.QuotaQueryTotal.Inc()
	}

	// 先尝试从 Redis 获取配额（格式 "totalQuota,usedQuota"）
	quotaKey := fmt.Sprintf("%s%s:%s:%s:%s", constants.RedisKeyQuota, userID, appID, serviceName, month)
	cacheVal, err := r.data.rdb.Get(ctx, quotaKey).Result()
	if err == nil {
		total, used, ok := parseQuotaCache(cacheVal)
		if ok {
			return &biz.FreeQuota{
				UserID:      userID,
				AppID:       appID,
				ServiceName: serviceName,
				TotalQuota:  total,
				UsedQuota:   used,
				ResetMonth:  month,
			}, nil
		}
		// 格式不符或解析失败，回源 DB
	}

	// 从数据库查询完整信息
	var m model.FreeQuota
	if err := r.data.db.WithContext(ctx).
		Where("user_id = ? AND app_id = ? AND service_name = ? AND reset_month = ?", userID, appID, serviceName, month).
		First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	result := &biz.FreeQuota{
		UserID:      m.UserID,
		AppID:       m.AppID,
		ServiceName: m.ServiceName,
		TotalQuota:  m.TotalQuota,
		UsedQuota:   m.UsedQuota,
		ResetMonth:  m.ResetMonth,
	}

	// 更新缓存：存 "totalQuota,usedQuota" 便于命中时直接返回完整配额（异步，不阻塞）
	go func() {
		cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cacheCancel()
		cacheVal := fmt.Sprintf("%d,%d", m.TotalQuota, m.UsedQuota)
		if err := r.data.rdb.Set(cacheCtx, quotaKey, cacheVal, 5*time.Minute).Err(); err != nil {
			// 缓存更新失败不影响主流程（异步操作，不使用 r.log）
		}
	}()

	return result, nil
}

// CreateFreeQuota 创建免费额度
func (r *freeQuotaRepo) CreateFreeQuota(ctx context.Context, quota *biz.FreeQuota) error {
	m := model.FreeQuota{
		FreeQuotaID: uuid.New().String(),
		UserID:      quota.UserID,
		AppID:       quota.AppID,
		ServiceName: quota.ServiceName,
		TotalQuota:  quota.TotalQuota,
		UsedQuota:   quota.UsedQuota,
		ResetMonth:  quota.ResetMonth,
	}
	return r.data.db.WithContext(ctx).Create(&m).Error
}

// IncrementUsedQuota 原子增加已使用配额（用于免费应用记录用量，避免读-改-写竞态）
func (r *freeQuotaRepo) IncrementUsedQuota(ctx context.Context, userID, appID, serviceName, month string, count int) error {
	return r.data.db.WithContext(ctx).Model(&model.FreeQuota{}).
		Where("user_id = ? AND app_id = ? AND service_name = ? AND reset_month = ?", userID, appID, serviceName, month).
		Update("used_quota", gorm.Expr("used_quota + ?", count)).Error
}
