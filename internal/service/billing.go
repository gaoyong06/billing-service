package service

import (
	"context"

	pb "billing-service/api/billing/v1"
	"billing-service/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type BillingService struct {
	pb.UnimplementedBillingServiceServer
	pb.UnimplementedBillingInternalServiceServer

	uc  *biz.BillingUseCase
	log *log.Helper
}

func NewBillingService(uc *biz.BillingUseCase, logger log.Logger) *BillingService {
	return &BillingService{
		uc:  uc,
		log: log.NewHelper(logger),
	}
}

// GetAccount 获取账户资产信息
func (s *BillingService) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.GetAccountReply, error) {
	balance, quotas, err := s.uc.GetAccount(ctx, req.UserId)
	if err != nil {
		return nil, err
	}

	pbQuotas := make([]*pb.FreeQuota, 0, len(quotas))
	for _, q := range quotas {
		pbQuotas = append(pbQuotas, &pb.FreeQuota{
			ServiceName: q.ServiceName,
			TotalQuota:  int32(q.TotalQuota),
			UsedQuota:   int32(q.UsedQuota),
			ResetMonth:  q.ResetMonth,
		})
	}

	return &pb.GetAccountReply{
		UserId:  balance.UserID,
		Balance: balance.Balance,
		Quotas:  pbQuotas,
	}, nil
}

// Recharge 发起充值
func (s *BillingService) Recharge(ctx context.Context, req *pb.RechargeRequest) (*pb.RechargeReply, error) {
	orderID, payURL, err := s.uc.Recharge(ctx, req.UserId, req.Amount)
	if err != nil {
		return nil, err
	}
	return &pb.RechargeReply{
		OrderId:    orderID,
		PaymentUrl: payURL,
	}, nil
}

// ListRecords 获取消费流水
func (s *BillingService) ListRecords(ctx context.Context, req *pb.ListRecordsRequest) (*pb.ListRecordsReply, error) {
	records, total, err := s.uc.ListRecords(ctx, req.UserId, int(req.Page), int(req.PageSize))
	if err != nil {
		return nil, err
	}

	pbRecords := make([]*pb.BillingRecord, 0, len(records))
	for _, r := range records {
		pbRecords = append(pbRecords, &pb.BillingRecord{
			Id:          r.ID,
			ServiceName: r.ServiceName,
			Type:        int32(r.Type),
			Amount:      r.Amount,
			Count:       int32(r.Count),
			CreatedAt:   timestamppb.New(r.CreatedAt),
		})
	}

	return &pb.ListRecordsReply{
		Records: pbRecords,
		Total:   int32(total),
	}, nil
}

// CheckQuota 检查并预扣费
func (s *BillingService) CheckQuota(ctx context.Context, req *pb.CheckQuotaRequest) (*pb.CheckQuotaReply, error) {
	allowed, reason, err := s.uc.CheckQuota(ctx, req.UserId, req.ServiceName, int(req.Count))
	if err != nil {
		return nil, err
	}
	return &pb.CheckQuotaReply{
		Allowed: allowed,
		Reason:  reason,
	}, nil
}

// DeductQuota 确认扣费
func (s *BillingService) DeductQuota(ctx context.Context, req *pb.DeductQuotaRequest) (*pb.DeductQuotaReply, error) {
	recordID, err := s.uc.DeductQuota(ctx, req.UserId, req.ServiceName, int(req.Count))
	if err != nil {
		return &pb.DeductQuotaReply{Success: false}, err
	}
	return &pb.DeductQuotaReply{
		Success:  true,
		RecordId: recordID,
	}, nil
}

// RechargeCallback 充值回调
func (s *BillingService) RechargeCallback(ctx context.Context, req *pb.RechargeCallbackRequest) (*pb.RechargeCallbackReply, error) {
	// 验证支付状态
	if req.Status != "SUCCESS" {
		return &pb.RechargeCallbackReply{Success: true}, nil // 支付失败，直接返回成功（已处理）
	}

	err := s.uc.RechargeCallback(ctx, req.OrderId, req.Amount)
	if err != nil {
		return &pb.RechargeCallbackReply{Success: false}, err
	}
	return &pb.RechargeCallbackReply{Success: true}, nil
}
