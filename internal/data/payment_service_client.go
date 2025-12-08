package data

import (
	"context"
	"strconv"

	"billing-service/internal/biz"
	"billing-service/internal/conf"
	billingErrors "billing-service/internal/errors"
	paymentv1 "xinyuan_tech/payment-service/api/payment/v1"

	pkgErrors "github.com/gaoyong06/go-pkg/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
)

// PaymentServiceClient payment-service 客户端接口（实现 biz.PaymentServiceClient）
// 直接使用 biz.PaymentServiceClient 接口，避免重复定义
type PaymentServiceClient = biz.PaymentServiceClient

// paymentServiceClient payment-service gRPC 客户端实现
type paymentServiceClient struct {
	client paymentv1.PaymentClient
	log    *log.Helper
}

// NewPaymentServiceClient 创建 payment-service 客户端
func NewPaymentServiceClient(c *conf.PaymentService, logger log.Logger) (PaymentServiceClient, error) {
	if c == nil {
		return nil, pkgErrors.NewBizErrorWithLang(context.Background(), billingErrors.ErrCodePaymentServiceConfigNil)
	}

	logHelper := log.NewHelper(logger)

	// 创建 gRPC 连接
	conn, err := grpc.DialInsecure(
		context.Background(),
		grpc.WithEndpoint(c.GrpcAddr),
		grpc.WithTimeout(c.Timeout.AsDuration()),
		grpc.WithMiddleware(
			recovery.Recovery(),
		),
	)
	if err != nil {
		return nil, pkgErrors.WrapErrorWithLang(context.Background(), err, billingErrors.ErrCodePaymentServiceDialFailed)
	}

	// 创建 gRPC 客户端
	client := paymentv1.NewPaymentClient(conn)

	return &paymentServiceClient{
		client: client,
		log:    logHelper,
	}, nil
}

// CreatePayment 创建支付订单（实现 biz.PaymentServiceClient 接口）
func (c *paymentServiceClient) CreatePayment(ctx context.Context, req *biz.CreatePaymentRequest) (*biz.CreatePaymentReply, error) {
	// 将 uid 从 string 转换为 uint64
	userID, err := strconv.ParseUint(req.UID, 10, 64)
	if err != nil {
		return nil, pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodeInvalidUserID)
	}

	// 将金额从元转换为分
	amountCents := int64(req.Amount * 100)

	// 调用 payment-service 的 gRPC 接口
	resp, err := c.client.CreatePayment(ctx, &paymentv1.CreatePaymentRequest{
		OrderId:   req.OrderID,
		UserId:    userID,
		AppId:     req.AppID, // 传递应用ID（充值场景可能为空）
		Source:    "billing", // 标记来源为充值
		Amount:    amountCents,
		Currency:  req.Currency,
		Method:    paymentv1.PaymentMethod(req.Method),
		Subject:   req.Subject,
		ReturnUrl: req.ReturnURL,
		NotifyUrl: req.NotifyURL,
		ClientIp:  req.ClientIP,
	})
	if err != nil {
		c.log.Errorf("CreatePayment failed: order_id=%s, error=%v", req.OrderID, err)
		return nil, pkgErrors.WrapErrorWithLang(ctx, err, billingErrors.ErrCodePaymentCreateFailed)
	}

	c.log.Infof("CreatePayment success: order_id=%s, payment_id=%s, pay_url=%s",
		req.OrderID, resp.PaymentId, resp.PayUrl)

	return &biz.CreatePaymentReply{
		PaymentID: resp.PaymentId,
		Status:    int32(resp.Status),
		PayURL:    resp.PayUrl,
		PayCode:   resp.PayCode,
		PayParams: resp.PayParams,
	}, nil
}
