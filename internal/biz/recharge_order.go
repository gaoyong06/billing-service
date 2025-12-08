package biz

import (
	"context"
	"fmt"
	"time"

	"billing-service/internal/constants"
	"billing-service/internal/metrics"

	billingErrors "billing-service/internal/errors"

	pkgErrors "github.com/gaoyong06/go-pkg/errors"
	pkgUtils "github.com/gaoyong06/go-pkg/utils"
	"github.com/go-kratos/kratos/v2/log"
)

// RechargeOrder 充值订单领域对象
type RechargeOrder struct {
	RechargeOrderID string    // 充值订单ID（billing-service生成，传给payment-service作为业务订单号）
	UID             string    // 用户ID
	Amount          float64   // 充值金额
	PaymentID       string    // 支付流水号（payment-service返回的payment_id）
	Status          string    // 订单状态
	CreatedAt       time.Time // 创建时间
	UpdatedAt       time.Time // 更新时间
}

// RechargeOrderRepo 充值订单数据层接口（定义在 biz 层）
type RechargeOrderRepo interface {
	CreateRechargeOrder(ctx context.Context, orderID, userID string, amount float64) error
	GetRechargeOrderByID(ctx context.Context, orderID string) (*RechargeOrder, error)
	GetRechargeOrderByPaymentID(ctx context.Context, paymentID string) (*RechargeOrder, error)
	UpdateRechargeOrderStatus(ctx context.Context, orderID, paymentID, status string) error
	RechargeWithIdempotency(ctx context.Context, orderID, paymentID string, amount float64) error
}

// RechargeOrderUseCase 充值订单业务逻辑
type RechargeOrderUseCase struct {
	repo                 RechargeOrderRepo
	paymentServiceClient PaymentServiceClient
	conf                 *BillingConfig
	log                  *log.Helper
	metrics              *metrics.BillingMetrics
}

// NewRechargeOrderUseCase 创建充值订单 UseCase
func NewRechargeOrderUseCase(
	repo RechargeOrderRepo,
	paymentServiceClient PaymentServiceClient,
	conf *BillingConfig,
	logger log.Logger,
) *RechargeOrderUseCase {
	return &RechargeOrderUseCase{
		repo:                 repo,
		paymentServiceClient: paymentServiceClient,
		conf:                 conf,
		log:                  log.NewHelper(logger),
		metrics:              metrics.GetMetrics(),
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
	orderID := fmt.Sprintf("%s%s_%d", constants.OrderIDPrefixRecharge, userID, time.Now().Unix())

	// 创建充值订单记录（用于幂等性保证）
	orderCreateStart := time.Now()
	if err := uc.repo.CreateRechargeOrder(ctx, orderID, userID, amount); err != nil {
		uc.log.Errorf("CreateRechargeOrder failed: %v", err)
		if uc.metrics != nil {
			uc.metrics.RechargeOrderTotal.WithLabelValues(constants.OrderStatusFailed).Inc()
			uc.metrics.RechargeFailedTotal.Inc()
		}
		return "", "", pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodeRechargeOrderCreateFailed)
	}
	if uc.metrics != nil {
		uc.metrics.RechargeOrderCreateDuration.Observe(time.Since(orderCreateStart).Seconds())
		uc.metrics.RechargeOrderTotal.WithLabelValues(constants.OrderStatusPending).Inc()
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
	// 注意：充值场景是开发者向 DevShare 平台充值，用于支付平台提供的服务（如 passport、asset 等）
	// 这不是为开发者的某个具体应用充值，所以 app_id 应该为空
	// 如果将来需要区分平台充值和应用充值，可以考虑使用特殊的 app_id 值（如 "platform"）
	paymentResp, err := uc.paymentServiceClient.CreatePayment(ctx, &CreatePaymentRequest{
		OrderID:   orderID,
		UID:       userID,
		AppID:     "", // 充值场景：开发者向平台充值，不关联具体应用
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
			uc.metrics.RechargeTotal.WithLabelValues(constants.OrderStatusFailed).Inc()
			uc.metrics.RechargeFailedTotal.Inc()
			uc.metrics.RechargeDuration.WithLabelValues("create").Observe(time.Since(startTime).Seconds())
		}
		return "", "", pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodePaymentCreateFailed)
	}

	// 记录充值成功指标
	if uc.metrics != nil {
		uc.metrics.RechargeTotal.WithLabelValues(constants.OrderStatusSuccess).Inc()
		uc.metrics.RechargeAmount.WithLabelValues(constants.OrderStatusSuccess).Add(amount)
		uc.metrics.RechargeDuration.WithLabelValues("create").Observe(time.Since(startTime).Seconds())
	}

	uc.log.Infof("Recharge order created: order_id=%s, payment_id=%s, pay_url=%s", orderID, paymentResp.PaymentID, paymentResp.PayURL)
	return orderID, paymentResp.PayURL, nil
}

// RechargeCallback 充值回调（支持幂等性）
func (uc *RechargeOrderUseCase) RechargeCallback(ctx context.Context, orderID string, amount float64) error {
	// 使用 payment-service 的 payment_id 作为幂等性标识
	// 这里假设 orderID 就是 payment-service 的 payment_id
	paymentID := orderID

	// 尝试通过 payment_id 查询订单（幂等性检查）
	existingOrder, err := uc.repo.GetRechargeOrderByPaymentID(ctx, paymentID)
	if err != nil {
		uc.log.Errorf("GetRechargeOrderByPaymentID failed: %v", err)
		return err
	}

	if existingOrder != nil {
		// 订单已存在，检查状态
		if existingOrder.Status == constants.OrderStatusSuccess {
			uc.log.Infof("Recharge already processed: payment_id=%s, status=%s", paymentID, existingOrder.Status)
			return nil // 已经处理过，直接返回成功（幂等性）
		}
		// 使用已存在的订单ID
		orderID = existingOrder.RechargeOrderID
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
		if existingOrder.Status == constants.OrderStatusSuccess {
			uc.log.Infof("Recharge already processed: recharge_order_id=%s, status=%s", orderID, existingOrder.Status)
			return nil // 已经处理过，直接返回成功（幂等性）
		}
	}

	// 执行充值（带幂等性保证）
	return uc.repo.RechargeWithIdempotency(ctx, orderID, paymentID, amount)
}
