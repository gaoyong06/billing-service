package service

import (
	"context"

	"billing-service/api/billing/v1"
	"billing-service/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

// BillingInternalService 面向 Gateway/Payment 的内部服务
type BillingInternalService struct {
	v1.UnimplementedBillingInternalServiceServer

	uc  *biz.BillingUseCase
	log *log.Helper
}

// NewBillingInternalService 创建 BillingInternalService
func NewBillingInternalService(uc *biz.BillingUseCase, logger log.Logger) *BillingInternalService {
	return &BillingInternalService{
		uc:  uc,
		log: log.NewHelper(logger),
	}
}

// CheckQuota 检查配额
func (s *BillingInternalService) CheckQuota(ctx context.Context, req *v1.CheckQuotaRequest) (*v1.CheckQuotaReply, error) {
	allowed, reason, err := s.uc.CheckQuota(ctx, req.UserId, req.ServiceName, int(req.Count))
	if err != nil {
		s.log.Errorf("CheckQuota failed: %v", err)
		return &v1.CheckQuotaReply{
			Allowed: false,
			Reason:  err.Error(),
		}, nil
	}

	return &v1.CheckQuotaReply{
		Allowed: allowed,
		Reason:  reason,
	}, nil
}

// DeductQuota 扣减配额
func (s *BillingInternalService) DeductQuota(ctx context.Context, req *v1.DeductQuotaRequest) (*v1.DeductQuotaReply, error) {
	recordID, err := s.uc.DeductQuota(ctx, req.UserId, req.ServiceName, int(req.Count))
	if err != nil {
		s.log.Errorf("DeductQuota failed: %v", err)
		return &v1.DeductQuotaReply{
			Success: false,
		}, err
	}

	return &v1.DeductQuotaReply{
		Success:  true,
		RecordId: recordID,
	}, nil
}

// RechargeCallback 充值回调
func (s *BillingInternalService) RechargeCallback(ctx context.Context, req *v1.RechargeCallbackRequest) (*v1.RechargeCallbackReply, error) {
	if req.Status != "success" {
		s.log.Warnf("RechargeCallback: payment status is not success, order_id: %s, status: %s", req.OrderId, req.Status)
		return &v1.RechargeCallbackReply{
			Success: false,
		}, nil
	}

	err := s.uc.RechargeCallback(ctx, req.OrderId, req.Amount)
	if err != nil {
		s.log.Errorf("RechargeCallback failed: %v", err)
		return &v1.RechargeCallbackReply{
			Success: false,
		}, err
	}

	return &v1.RechargeCallbackReply{
		Success: true,
	}, nil
}

