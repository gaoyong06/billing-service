package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"billing-service/internal/biz"
	"billing-service/internal/data/model"
	billingErrors "billing-service/internal/errors"

	pkgErrors "github.com/gaoyong06/go-pkg/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// rechargeOrderRepo 充值订单相关数据访问
type rechargeOrderRepo struct {
	data *Data
	log  *log.Helper
}

// NewRechargeOrderRepo 创建充值订单 repo（返回 biz.RechargeOrderRepo 接口）
func NewRechargeOrderRepo(data *Data, logger log.Logger) biz.RechargeOrderRepo {
	return &rechargeOrderRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// CreateRechargeOrder 创建充值订单记录
func (r *rechargeOrderRepo) CreateRechargeOrder(ctx context.Context, orderID, userID string, amount float64) error {
	order := model.RechargeOrder{
		OrderID: orderID,
		UserID:  userID,
		Amount:  amount,
		Status:  model.RechargeStatusPending,
	}
	return r.data.db.WithContext(ctx).Create(&order).Error
}

// GetRechargeOrderByID 通过订单ID查询充值订单
func (r *rechargeOrderRepo) GetRechargeOrderByID(ctx context.Context, orderID string) (*biz.RechargeOrder, error) {
	var m model.RechargeOrder
	if err := r.data.db.WithContext(ctx).Where("order_id = ?", orderID).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &biz.RechargeOrder{
		OrderID:        m.OrderID,
		UserID:         m.UserID,
		Amount:         m.Amount,
		PaymentOrderID: m.PaymentOrderID,
		Status:         m.Status,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}, nil
}

// GetRechargeOrderByPaymentID 通过支付订单ID查询充值订单
func (r *rechargeOrderRepo) GetRechargeOrderByPaymentID(ctx context.Context, paymentOrderID string) (*biz.RechargeOrder, error) {
	var m model.RechargeOrder
	if err := r.data.db.WithContext(ctx).Where("payment_order_id = ?", paymentOrderID).First(&m).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}

	return &biz.RechargeOrder{
		OrderID:        m.OrderID,
		UserID:         m.UserID,
		Amount:         m.Amount,
		PaymentOrderID: m.PaymentOrderID,
		Status:         m.Status,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}, nil
}

// UpdateRechargeOrderStatus 更新充值订单状态
func (r *rechargeOrderRepo) UpdateRechargeOrderStatus(ctx context.Context, orderID, paymentOrderID, status string) error {
	updates := map[string]interface{}{
		"status": status,
	}
	if paymentOrderID != "" {
		updates["payment_order_id"] = paymentOrderID
	}

	return r.data.db.WithContext(ctx).Model(&model.RechargeOrder{}).
		Where("order_id = ?", orderID).
		Updates(updates).Error
}

// RechargeWithIdempotency 带幂等性保证的充值
func (r *rechargeOrderRepo) RechargeWithIdempotency(ctx context.Context, orderID, paymentOrderID string, amount float64) error {
	return r.data.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 锁定订单记录
		var order model.RechargeOrder
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("order_id = ?", orderID).
			First(&order).Error; err != nil {
			return pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodeRechargeOrderGetFailed)
		}

		// 2. 检查订单状态（幂等性）
		if order.Status == model.RechargeStatusSuccess {
			r.log.Infof("Recharge already processed: order_id=%s", orderID)
			return nil // 已经处理过，直接返回成功
		}

		// 3. 更新订单状态和 payment_order_id
		if err := tx.Model(&order).Updates(map[string]interface{}{
			"payment_order_id": paymentOrderID,
			"status":           model.RechargeStatusSuccess,
		}).Error; err != nil {
			return pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodeRechargeOrderUpdateFailed)
		}

		// 4. 执行充值
		var balance model.UserBalance
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("user_id = ?", order.UserID).
			First(&balance).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				// 用户余额不存在，创建新记录
				balance = model.UserBalance{
					UserBalanceID: uuid.New().String(),
					UserID:        order.UserID,
					Balance:       amount,
				}
				if err := tx.Create(&balance).Error; err != nil {
					return pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodeUserBalanceCreateFailed)
				}
			} else {
				return pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodeUserBalanceGetFailed)
			}
		} else {
			// 余额存在，增加余额
			if err := tx.Model(&balance).Update("balance", gorm.Expr("balance + ?", amount)).Error; err != nil {
				return pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodeUserBalanceUpdateFailed)
			}
		}

		// 5. 更新 Redis 缓存（设置超时避免阻塞）
		balanceKey := fmt.Sprintf("balance:%s", order.UserID)
		newBalance := balance.Balance + amount
		cacheCtx, cacheCancel := context.WithTimeout(context.Background(), 1*time.Second)
		defer cacheCancel()
		if err := r.data.rdb.Set(cacheCtx, balanceKey, fmt.Sprintf("%.2f", newBalance), 5*time.Minute).Err(); err != nil {
			// 缓存更新失败不影响主流程，只记录日志
			r.log.Warnf("failed to update balance cache in RechargeWithIdempotency: %v", err)
		}

		return nil
	})
}

