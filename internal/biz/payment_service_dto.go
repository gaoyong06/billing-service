package biz

import "context"

// PaymentServiceClient payment-service 客户端接口
type PaymentServiceClient interface {
	CreatePayment(ctx context.Context, req *CreatePaymentRequest) (*CreatePaymentReply, error)
}

// CreatePaymentRequest 创建支付请求
type CreatePaymentRequest struct {
	OrderID   string // 充值订单ID（billing-service生成，传给payment-service作为业务订单号）
	UID       string
	AppID     string // 应用ID（开发者充值时使用开发者的 app_id）
	Amount    float64
	Currency  string
	Method    int32
	Subject   string
	ReturnURL string
	NotifyURL string
	ClientIP  string
}

// CreatePaymentReply 创建支付响应
type CreatePaymentReply struct {
	PaymentID string
	Status    int32
	PayURL    string
	PayCode   string
	PayParams string
}
