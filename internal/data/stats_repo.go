package data

import (
	"context"
	"fmt"
	"time"

	"billing-service/internal/biz"
	"billing-service/internal/constants"
	"billing-service/internal/data/model"
	billingErrors "billing-service/internal/errors"

	pkgErrors "github.com/gaoyong06/go-pkg/errors"
	"github.com/go-kratos/kratos/v2/log"
)

// statsRepo 统计相关数据访问
type statsRepo struct {
	data *Data
	log  *log.Helper
}

// NewStatsRepo 创建统计 repo（返回 biz.StatsRepo 接口）
func NewStatsRepo(data *Data, logger log.Logger) biz.StatsRepo {
	return &statsRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// GetAllUserIDs 获取所有用户ID（用于重置免费额度）
// 从 free_quota 和 user_balance 表中获取所有不重复的 user_id
// 确保所有用户（包括新用户）都能获得免费额度
func (r *statsRepo) GetAllUserIDs(ctx context.Context) ([]string, error) {
	userIDMap := make(map[string]bool)

	// 从 free_quota 表获取用户ID
	var quotaUserIDs []string
	if err := r.data.db.WithContext(ctx).
		Model(&model.FreeQuota{}).
		Distinct("user_id").
		Pluck("user_id", &quotaUserIDs).Error; err != nil {
		return nil, pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodeGetAllUserIDsFailed)
	}
	for _, userID := range quotaUserIDs {
		userIDMap[userID] = true
	}

	// 从 user_balance 表获取用户ID（可能有些用户只有余额，还没有免费额度记录）
	var balanceUserIDs []string
	if err := r.data.db.WithContext(ctx).
		Model(&model.UserBalance{}).
		Distinct("user_id").
		Pluck("user_id", &balanceUserIDs).Error; err != nil {
		return nil, pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodeGetAllUserIDsFailed)
	}
	for _, userID := range balanceUserIDs {
		userIDMap[userID] = true
	}

	// 转换为切片
	userIDs := make([]string, 0, len(userIDMap))
	for userID := range userIDMap {
		userIDs = append(userIDs, userID)
	}

	return userIDs, nil
}

// GetStatsToday 获取今日调用统计
func (r *statsRepo) GetStatsToday(ctx context.Context, userID, serviceName string) (*biz.Stats, error) {
	// 获取今日开始时间
	now := time.Now()
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayEnd := todayStart.Add(24 * time.Hour)

	// 构建查询条件
	query := r.data.db.WithContext(ctx).Model(&model.BillingRecord{}).
		Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, todayStart, todayEnd)

	// 如果指定了服务名称，添加过滤条件
	if serviceName != "" {
		query = query.Where("service_name = ?", serviceName)
	}

	// 统计总调用次数和总费用
	var result struct {
		TotalCount int
		TotalCost  float64
		FreeCount  int
		PaidCount  int
	}

	if err := query.Select(
		"SUM(count) as total_count",
		fmt.Sprintf("SUM(CASE WHEN type = '%s' THEN amount ELSE 0 END) as total_cost", constants.BillingTypeBalance),
		fmt.Sprintf("SUM(CASE WHEN type = '%s' THEN count ELSE 0 END) as free_count", constants.BillingTypeFree),
		fmt.Sprintf("SUM(CASE WHEN type = '%s' THEN count ELSE 0 END) as paid_count", constants.BillingTypeBalance),
	).Scan(&result).Error; err != nil {
		return nil, pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodeGetStatsFailed)
	}

	return &biz.Stats{
		UserID:      userID,
		ServiceName: serviceName,
		TotalCount:  result.TotalCount,
		TotalCost:   result.TotalCost,
		FreeCount:   result.FreeCount,
		PaidCount:   result.PaidCount,
		Period:      constants.StatsPeriodToday,
	}, nil
}

// GetStatsMonth 获取本月调用统计
func (r *statsRepo) GetStatsMonth(ctx context.Context, userID, serviceName string) (*biz.Stats, error) {
	// 获取本月开始时间
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	nextMonthStart := monthStart.AddDate(0, 1, 0)

	// 构建查询条件
	query := r.data.db.WithContext(ctx).Model(&model.BillingRecord{}).
		Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, monthStart, nextMonthStart)

	// 如果指定了服务名称，添加过滤条件
	if serviceName != "" {
		query = query.Where("service_name = ?", serviceName)
	}

	// 统计总调用次数和总费用
	var result struct {
		TotalCount int
		TotalCost  float64
		FreeCount  int
		PaidCount  int
	}

	if err := query.Select(
		"SUM(count) as total_count",
		fmt.Sprintf("SUM(CASE WHEN type = '%s' THEN amount ELSE 0 END) as total_cost", constants.BillingTypeBalance),
		fmt.Sprintf("SUM(CASE WHEN type = '%s' THEN count ELSE 0 END) as free_count", constants.BillingTypeFree),
		fmt.Sprintf("SUM(CASE WHEN type = '%s' THEN count ELSE 0 END) as paid_count", constants.BillingTypeBalance),
	).Scan(&result).Error; err != nil {
		return nil, pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodeGetStatsFailed)
	}

	return &biz.Stats{
		UserID:      userID,
		ServiceName: serviceName,
		TotalCount:  result.TotalCount,
		TotalCost:   result.TotalCost,
		FreeCount:   result.FreeCount,
		PaidCount:   result.PaidCount,
		Period:      constants.StatsPeriodMonth,
	}, nil
}

// GetStatsSummary 获取汇总统计（所有服务）
func (r *statsRepo) GetStatsSummary(ctx context.Context, userID string) (*biz.StatsSummary, error) {
	// 获取本月开始时间
	now := time.Now()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	nextMonthStart := monthStart.AddDate(0, 1, 0)

	// 按服务名称分组统计
	var serviceStats []struct {
		ServiceName string
		TotalCount  int
		TotalCost   float64
		FreeCount   int
		PaidCount   int
	}

	if err := r.data.db.WithContext(ctx).Model(&model.BillingRecord{}).
		Where("user_id = ? AND created_at >= ? AND created_at < ?", userID, monthStart, nextMonthStart).
		Select(
			"service_name",
			"SUM(count) as total_count",
			fmt.Sprintf("SUM(CASE WHEN type = '%s' THEN amount ELSE 0 END) as total_cost", constants.BillingTypeBalance),
			fmt.Sprintf("SUM(CASE WHEN type = '%s' THEN count ELSE 0 END) as free_count", constants.BillingTypeFree),
			fmt.Sprintf("SUM(CASE WHEN type = '%s' THEN count ELSE 0 END) as paid_count", constants.BillingTypeBalance),
		).
		Group("service_name").
		Scan(&serviceStats).Error; err != nil {
		return nil, pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodeGetStatsFailed)
	}

	// 转换为业务对象
	services := make([]*biz.ServiceStats, 0, len(serviceStats))
	totalCount := 0
	totalCost := 0.0

	for _, s := range serviceStats {
		services = append(services, &biz.ServiceStats{
			ServiceName: s.ServiceName,
			TotalCount:  s.TotalCount,
			TotalCost:   s.TotalCost,
			FreeCount:   s.FreeCount,
			PaidCount:   s.PaidCount,
		})
		totalCount += s.TotalCount
		totalCost += s.TotalCost
	}

	return &biz.StatsSummary{
		UserID:     userID,
		TotalCount: totalCount,
		TotalCost:  totalCost,
		Services:   services,
	}, nil
}

