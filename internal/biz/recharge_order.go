package biz

import (
	"context"
	"fmt"
	"time"

	"billing-service/internal/metrics"

	pkgErrors "github.com/gaoyong06/go-pkg/errors"
	pkgUtils "github.com/gaoyong06/go-pkg/utils"
	billingErrors "billing-service/internal/errors"
	"github.com/go-kratos/kratos/v2/log"
)

// RechargeOrder 充值订单领域对象
type RechargeOrder struct {
	OrderID        string
	UserID         string
	Amount         float64
	PaymentOrderID string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// RechargeOrderRepo 充值订单数据层接口（定义在 biz 层）
type RechargeOrderRepo interface {
	CreateRechargeOrder(ctx context.Context, orderID, userID string, amount float64) error
	GetRechargeOrderByID(ctx context.Context, orderID string) (*RechargeOrder, error)
	GetRechargeOrderByPaymentID(ctx context.Context, paymentOrderID string) (*RechargeOrder, error)
	UpdateRechargeOrderStatus(ctx context.Context, orderID, paymentOrderID, status string) error
	RechargeWithIdempotency(ctx context.Context, orderID, paymentOrderID string, amount float64) error
}

// RechargeOrderUseCase 充值订单业务逻辑
type RechargeOrderUseCase struct {
	repo                RechargeOrderRepo
	paymentServiceClient PaymentServiceClient
	conf                *BillingConfig
	log                 *log.Helper
	metrics             *metrics.BillingMetrics
}

// NewRechargeOrderUseCase 创建充值订单 UseCase
func NewRechargeOrderUseCase(
	repo RechargeOrderRepo,
	paymentServiceClient PaymentServiceClient,
	conf *BillingConfig,
	logger log.Logger,
) *RechargeOrderUseCase {
	return &RechargeOrderUseCase{
		repo:                repo,
		paymentServiceClient: paymentServiceClient,
		conf:                conf,
		log:                 log.NewHelper(logger),
		metrics:             metrics.GetMetrics(),
	}
}

// CreateRecharge 创建充值订单
func (uc *RechargeOrderUseCase) CreateRecharge(ctx context.Context, userID string, amount float64, method int32, currency, returnURL, notifyURL string) (string, string, error) {
	startTime := time.Now()

	// 验证币种必填
	if currency == "" {
		return "", "", pkgErrors.NewBizErrorWithLang(ctx, billingErrors.ErrCodeCurrencyRequired)
	}

	// 生成订单ID
	orderID := fmt.Sprintf("recharge_%s_%d", userID, time.Now().Unix())

	// 创建充值订单记录（用于幂等性保证）
	orderCreateStart := time.Now()
	if err := uc.repo.CreateRechargeOrder(ctx, orderID, userID, amount); err != nil {
		uc.log.Errorf("CreateRechargeOrder failed: %v", err)
		if uc.metrics != nil {
			uc.metrics.RechargeOrderTotal.WithLabelValues("failed").Inc()
			uc.metrics.RechargeFailedTotal.Inc()
		}
		return "", "", pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodeRechargeOrderCreateFailed)
	}
	if uc.metrics != nil {
		uc.metrics.RechargeOrderCreateDuration.Observe(time.Since(orderCreateStart).Seconds())
		uc.metrics.RechargeOrderTotal.WithLabelValues("pending").Inc()
	}

	// 调用 Payment Service 创建支付订单
	if uc.paymentServiceClient == nil {
		return "", "", pkgErrors.NewBizErrorWithLang(ctx, billingErrors.ErrCodePaymentServiceUnavailable)
	}

	// 默认支付方式：支付宝
	if method == 0 {
		method = 1 // PAYMENT_METHOD_ALIPAY
	}

	// 从 context 中获取客户端 IP
	clientIP := pkgUtils.GetClientIP(ctx)

	// 调用 payment-service 创建支付订单
	paymentResp, err := uc.paymentServiceClient.CreatePayment(ctx, &CreatePaymentRequest{
		OrderID:   orderID,
		UserID:    userID,
		Amount:    amount,
		Currency:  currency,
		Method:    method,
		Subject:   fmt.Sprintf("账户充值 - %.2f元", amount),
		ReturnURL: returnURL,
		NotifyURL: notifyURL,
		ClientIP:  clientIP,
	})
	if err != nil {
		uc.log.Errorf("CreatePayment failed: order_id=%s, error=%v", orderID, err)
		if uc.metrics != nil {
			uc.metrics.RechargeTotal.WithLabelValues("failed").Inc()
			uc.metrics.RechargeFailedTotal.Inc()
			uc.metrics.RechargeDuration.WithLabelValues("create").Observe(time.Since(startTime).Seconds())
		}
		return "", "", pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodePaymentCreateFailed)
	}

	// 记录充值成功指标
	if uc.metrics != nil {
		uc.metrics.RechargeTotal.WithLabelValues("success").Inc()
		uc.metrics.RechargeAmount.WithLabelValues("success").Add(amount)
		uc.metrics.RechargeDuration.WithLabelValues("create").Observe(time.Since(startTime).Seconds())
	}

	uc.log.Infof("Recharge order created: order_id=%s, payment_id=%s, pay_url=%s", orderID, paymentResp.PaymentID, paymentResp.PayURL)
	return orderID, paymentResp.PayURL, nil
}

// RechargeCallback 充值回调（支持幂等性）
func (uc *RechargeOrderUseCase) RechargeCallback(ctx context.Context, orderID string, amount float64) error {
	// 使用 payment-service 的订单ID作为幂等性标识
	// 这里假设 orderID 就是 payment-service 的订单ID
	paymentOrderID := orderID

	// 尝试通过 payment_order_id 查询订单（幂等性检查）
	existingOrder, err := uc.repo.GetRechargeOrderByPaymentID(ctx, paymentOrderID)
	if err != nil {
		uc.log.Errorf("GetRechargeOrderByPaymentID failed: %v", err)
		return err
	}

	if existingOrder != nil {
		// 订单已存在，检查状态
		if existingOrder.Status == "success" {
			uc.log.Infof("Recharge already processed: payment_order_id=%s, status=%s", paymentOrderID, existingOrder.Status)
			return nil // 已经处理过，直接返回成功（幂等性）
		}
		// 使用已存在的订单ID
		orderID = existingOrder.OrderID
	} else {
		// 订单不存在，可能是旧数据或测试数据，尝试通过订单ID查询
		existingOrder, err = uc.repo.GetRechargeOrderByID(ctx, orderID)
		if err != nil {
			uc.log.Errorf("GetRechargeOrderByID failed: %v", err)
			return err
		}
		if existingOrder == nil {
			return pkgErrors.NewBizErrorWithLang(ctx, billingErrors.ErrCodeRechargeOrderNotFound)
		}
		if existingOrder.Status == "success" {
			uc.log.Infof("Recharge already processed: order_id=%s, status=%s", orderID, existingOrder.Status)
			return nil // 已经处理过，直接返回成功（幂等性）
		}
	}

	// 执行充值（带幂等性保证）
	return uc.repo.RechargeWithIdempotency(ctx, orderID, paymentOrderID, amount)
}

