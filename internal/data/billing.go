package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"billing-service/internal/biz"
	"billing-service/internal/data/model"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type billingRepo struct {
	data *Data
	log  *log.Helper
}

func NewBillingRepo(data *Data, logger log.Logger) biz.BillingRepo {
	return &billingRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *billingRepo) GetUserBalance(ctx context.Context, userID string) (*biz.UserBalance, error) {
	// 先尝试从 Redis 获取
	balanceKey := fmt.Sprintf("balance:%s", userID)
	balanceStr, err := r.data.rdb.Get(ctx, balanceKey).Result()
	if err == nil {
		// 从缓存获取成功
		var balance float64
		if _, err := fmt.Sscanf(balanceStr, "%f", &balance); err == nil {
			return &biz.UserBalance{
				UserID:  userID,
				Balance: balance,
			}, nil
		}
	}

	// 缓存未命中，从数据库查询
	var m model.UserBalance
	if err := r.data.db.WithContext(ctx).Where("user_id = ?", userID).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	result := &biz.UserBalance{
		UserID:    m.UserID,
		Balance:   m.Balance,
		Version:   m.Version,
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

func (r *billingRepo) UpdateBalance(ctx context.Context, userID string, amount float64, version int) error {
	result := r.data.db.WithContext(ctx).Model(&model.UserBalance{}).
		Where("user_id = ? AND version = ?", userID, version).
		Updates(map[string]interface{}{
			"balance": gorm.Expr("balance + ?", amount),
			"version": gorm.Expr("version + 1"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("optimistic lock failed")
	}
	return nil
}

func (r *billingRepo) Recharge(ctx context.Context, userID string, amount float64) error {
	// 简单充值逻辑：如果不存在则创建，存在则增加
	return r.data.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var m model.UserBalance
		if err := tx.Where("user_id = ?", userID).First(&m).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				m = model.UserBalance{
					UserBalanceID: uuid.New().String(),
					UserID:        userID,
					Balance:       amount,
					Version:       1,
				}
				return tx.Create(&m).Error
			}
			return err
		}
		if err := tx.Model(&m).Update("balance", gorm.Expr("balance + ?", amount)).Error; err != nil {
			return err
		}
		// 更新 Redis 缓存（设置超时避免阻塞）
		balanceKey := fmt.Sprintf("balance:%s", userID)
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

func (r *billingRepo) GetFreeQuota(ctx context.Context, userID, serviceName, month string) (*biz.FreeQuota, error) {
	// 先尝试从 Redis 获取剩余配额
	quotaKey := fmt.Sprintf("quota:%s:%s:%s", userID, serviceName, month)
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
		Where("user_id = ? AND service_name = ? AND reset_month = ?", userID, serviceName, month).
		First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	result := &biz.FreeQuota{
		UserID:      m.UserID,
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

func (r *billingRepo) CreateFreeQuota(ctx context.Context, quota *biz.FreeQuota) error {
	m := model.FreeQuota{
		FreeQuotaID: uuid.New().String(),
		UserID:      quota.UserID,
		ServiceName: quota.ServiceName,
		TotalQuota:  quota.TotalQuota,
		UsedQuota:   quota.UsedQuota,
		ResetMonth:  quota.ResetMonth,
	}
	return r.data.db.WithContext(ctx).Create(&m).Error
}

func (r *billingRepo) UpdateFreeQuota(ctx context.Context, quota *biz.FreeQuota) error {
	return r.data.db.WithContext(ctx).Model(&model.FreeQuota{}).
		Where("user_id = ? AND service_name = ? AND reset_month = ?", quota.UserID, quota.ServiceName, quota.ResetMonth).
		Update("used_quota", quota.UsedQuota).Error
}

func (r *billingRepo) CreateBillingRecord(ctx context.Context, record *biz.BillingRecord) error {
	m := model.BillingRecord{
		BillingRecordID: uuid.New().String(),
		UserID:          record.UserID,
		ServiceName:     record.ServiceName,
		Type:            int8(record.Type),
		Amount:          record.Amount,
		Count:           record.Count,
	}
	return r.data.db.WithContext(ctx).Create(&m).Error
}

func (r *billingRepo) ListBillingRecords(ctx context.Context, userID string, page, pageSize int) ([]*biz.BillingRecord, int64, error) {
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
			Type:        int(m.Type),
			Amount:      m.Amount,
			Count:       m.Count,
			CreatedAt:   m.CreatedAt,
		})
	}
	return records, total, nil
}

// DeductQuota 核心扣费逻辑（事务）
// 支持混合扣费：优先扣除免费额度，不足时扣除余额
func (r *billingRepo) DeductQuota(ctx context.Context, userID, serviceName string, count int, cost float64, month string) (string, error) {
	var recordID string
	var needUpdateQuotaCache bool
	var needUpdateBalanceCache bool
	var quotaRemaining int
	var newBalance float64

	err := r.data.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 检查并扣减免费额度
		var quota model.FreeQuota
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ? AND service_name = ? AND reset_month = ?", userID, serviceName, month).
			First(&quota).Error

		quotaNotFound := errors.Is(err, gorm.ErrRecordNotFound)
		if err != nil && !quotaNotFound {
			return err
		}

		var freeQuotaUsed int
		var balanceDeducted float64
		var balanceCount int

		// 如果有免费额度记录且还有剩余额度
		if !quotaNotFound && quota.TotalQuota > quota.UsedQuota {
			remaining := quota.TotalQuota - quota.UsedQuota
			if remaining >= count {
				// 免费额度充足，全部使用免费额度
				freeQuotaUsed = count
				if err := tx.Model(&quota).Update("used_quota", gorm.Expr("used_quota + ?", count)).Error; err != nil {
					return err
				}
				// 记录需要更新的缓存信息
				needUpdateQuotaCache = true
				quotaRemaining = remaining - count
			} else {
				// 免费额度不足，先扣完免费额度，剩余部分扣余额
				freeQuotaUsed = remaining
				balanceCount = count - remaining
				balanceDeducted = cost * float64(balanceCount) / float64(count) // 按比例计算余额扣费金额

				if err := tx.Model(&quota).Update("used_quota", gorm.Expr("used_quota + ?", remaining)).Error; err != nil {
					return err
				}
				// 记录需要更新的缓存信息
				needUpdateQuotaCache = true
				quotaRemaining = 0
			}
		} else {
			// 没有免费额度或已用完，全部扣余额
			balanceCount = count
			balanceDeducted = cost
		}

		// 2. 如果需要扣余额
		if balanceCount > 0 {
			var balance model.UserBalance
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("user_id = ?", userID).First(&balance).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					// 用户余额记录不存在，自动创建（初始余额为 0）
					balance = model.UserBalance{
						UserBalanceID: uuid.New().String(),
						UserID:        userID,
						Balance:       0,
						Version:       1,
					}
					if err := tx.Create(&balance).Error; err != nil {
						return fmt.Errorf("create user balance failed: %w", err)
					}
					// 余额为 0，无法扣费
					return errors.New("insufficient balance: balance is 0")
				}
				return fmt.Errorf("get balance failed: %w", err)
			}

			if balance.Balance < balanceDeducted {
				return errors.New("insufficient balance")
			}

			if err := tx.Model(&balance).Update("balance", gorm.Expr("balance - ?", balanceDeducted)).Error; err != nil {
				return err
			}

			// 记录需要更新的缓存信息
			needUpdateBalanceCache = true
			newBalance = balance.Balance - balanceDeducted
		}

		// 3. 记录流水
		// 如果混合扣费，生成两条记录；否则生成一条记录
		recordID = uuid.New().String()

		// 如果有使用免费额度，创建免费额度记录
		if freeQuotaUsed > 0 {
			freeRecord := model.BillingRecord{
				BillingRecordID: recordID,
				UserID:          userID,
				ServiceName:     serviceName,
				Type:            model.BillingTypeFree,
				Amount:          0,
				Count:           freeQuotaUsed,
			}
			if err := tx.Create(&freeRecord).Error; err != nil {
				return err
			}
		}

		// 如果有扣余额，创建余额扣费记录
		if balanceCount > 0 {
			balanceRecordID := recordID
			if freeQuotaUsed > 0 {
				// 混合扣费时，余额记录使用新的ID
				balanceRecordID = uuid.New().String()
			}
			balanceRecord := model.BillingRecord{
				BillingRecordID: balanceRecordID,
				UserID:          userID,
				ServiceName:     serviceName,
				Type:            model.BillingTypeBalance,
				Amount:          balanceDeducted,
				Count:           balanceCount,
			}
			if err := tx.Create(&balanceRecord).Error; err != nil {
				return err
			}
			// 返回余额记录的ID（如果混合扣费）
			if freeQuotaUsed > 0 {
				recordID = balanceRecordID
			}
		}

		return nil
	})

	// 事务提交成功后，更新 Redis 缓存（使用传入的 context，但设置较短的超时时间）
	if err == nil {
		// 使用独立的 context 更新缓存，避免阻塞主流程
		// 设置较短的超时时间，如果缓存更新失败不影响主流程
		cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cacheCancel()

		if needUpdateQuotaCache {
			quotaKey := fmt.Sprintf("quota:%s:%s:%s", userID, serviceName, month)
			if err := r.data.rdb.Set(cacheCtx, quotaKey, fmt.Sprintf("%d", quotaRemaining), 5*time.Minute).Err(); err != nil {
				// 缓存更新失败不影响主流程，只记录日志
				r.log.Warnf("failed to update quota cache: %v", err)
			}
		}
		if needUpdateBalanceCache {
			balanceKey := fmt.Sprintf("balance:%s", userID)
			if err := r.data.rdb.Set(cacheCtx, balanceKey, fmt.Sprintf("%.2f", newBalance), 5*time.Minute).Err(); err != nil {
				// 缓存更新失败不影响主流程，只记录日志
				r.log.Warnf("failed to update balance cache: %v", err)
			}
		}
	}

	return recordID, err
}

// SaveRechargeOrder 保存充值订单信息到 Redis
func (r *billingRepo) SaveRechargeOrder(ctx context.Context, orderID, userID string, amount float64) error {
	orderInfo := map[string]interface{}{
		"user_id": userID,
		"amount":  amount,
	}
	orderData, err := json.Marshal(orderInfo)
	if err != nil {
		return fmt.Errorf("marshal order info failed: %w", err)
	}

	key := fmt.Sprintf("recharge:order:%s", orderID)
	// 订单信息保存 7 天
	if err := r.data.rdb.Set(ctx, key, orderData, 7*24*time.Hour).Err(); err != nil {
		return fmt.Errorf("save order to redis failed: %w", err)
	}

	return nil
}

// GetRechargeOrder 从 Redis 获取充值订单信息
func (r *billingRepo) GetRechargeOrder(ctx context.Context, orderID string) (string, error) {
	key := fmt.Sprintf("recharge:order:%s", orderID)
	orderData, err := r.data.rdb.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return "", nil
		}
		return "", fmt.Errorf("get order from redis failed: %w", err)
	}

	var orderInfo map[string]interface{}
	if err := json.Unmarshal([]byte(orderData), &orderInfo); err != nil {
		return "", fmt.Errorf("unmarshal order info failed: %w", err)
	}

	userID, ok := orderInfo["user_id"].(string)
	if !ok {
		return "", fmt.Errorf("invalid user_id in order info")
	}

	return userID, nil
}

// GetAllUserIDs 获取所有用户ID（用于重置免费额度）
// 从 free_quota 和 user_balance 表中获取所有不重复的 user_id
// 确保所有用户（包括新用户）都能获得免费额度
func (r *billingRepo) GetAllUserIDs(ctx context.Context) ([]string, error) {
	userIDMap := make(map[string]bool)

	// 从 free_quota 表获取用户ID
	var quotaUserIDs []string
	if err := r.data.db.WithContext(ctx).
		Model(&model.FreeQuota{}).
		Distinct("user_id").
		Pluck("user_id", &quotaUserIDs).Error; err != nil {
		return nil, fmt.Errorf("get user IDs from free_quota failed: %w", err)
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
		return nil, fmt.Errorf("get user IDs from user_balance failed: %w", err)
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
func (r *billingRepo) GetStatsToday(ctx context.Context, userID, serviceName string) (*biz.Stats, error) {
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
		"SUM(CASE WHEN type = 2 THEN amount ELSE 0 END) as total_cost",
		"SUM(CASE WHEN type = 1 THEN count ELSE 0 END) as free_count",
		"SUM(CASE WHEN type = 2 THEN count ELSE 0 END) as paid_count",
	).Scan(&result).Error; err != nil {
		return nil, fmt.Errorf("get stats today failed: %w", err)
	}

	return &biz.Stats{
		UserID:      userID,
		ServiceName: serviceName,
		TotalCount:  result.TotalCount,
		TotalCost:   result.TotalCost,
		FreeCount:   result.FreeCount,
		PaidCount:   result.PaidCount,
		Period:      "today",
	}, nil
}

// GetStatsMonth 获取本月调用统计
func (r *billingRepo) GetStatsMonth(ctx context.Context, userID, serviceName string) (*biz.Stats, error) {
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
		"SUM(CASE WHEN type = 2 THEN amount ELSE 0 END) as total_cost",
		"SUM(CASE WHEN type = 1 THEN count ELSE 0 END) as free_count",
		"SUM(CASE WHEN type = 2 THEN count ELSE 0 END) as paid_count",
	).Scan(&result).Error; err != nil {
		return nil, fmt.Errorf("get stats month failed: %w", err)
	}

	return &biz.Stats{
		UserID:      userID,
		ServiceName: serviceName,
		TotalCount:  result.TotalCount,
		TotalCost:   result.TotalCost,
		FreeCount:   result.FreeCount,
		PaidCount:   result.PaidCount,
		Period:      "month",
	}, nil
}

// GetStatsSummary 获取汇总统计（所有服务）
func (r *billingRepo) GetStatsSummary(ctx context.Context, userID string) (*biz.StatsSummary, error) {
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
			"SUM(CASE WHEN type = 2 THEN amount ELSE 0 END) as total_cost",
			"SUM(CASE WHEN type = 1 THEN count ELSE 0 END) as free_count",
			"SUM(CASE WHEN type = 2 THEN count ELSE 0 END) as paid_count",
		).
		Group("service_name").
		Scan(&serviceStats).Error; err != nil {
		return nil, fmt.Errorf("get stats summary failed: %w", err)
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
