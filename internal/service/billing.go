package service

import (
	"context"

	"billing-service/api/billing/v1"
	"billing-service/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// BillingService 面向前端/开发者的服务
type BillingService struct {
	v1.UnimplementedBillingServiceServer

	uc  *biz.BillingUseCase
	log *log.Helper
}

// NewBillingService 创建 BillingService
func NewBillingService(uc *biz.BillingUseCase, logger log.Logger) *BillingService {
	return &BillingService{
		uc:  uc,
		log: log.NewHelper(logger),
	}
}

// GetAccount 获取账户信息
func (s *BillingService) GetAccount(ctx context.Context, req *v1.GetAccountRequest) (*v1.GetAccountReply, error) {
	balance, quotas, err := s.uc.GetAccount(ctx, req.UserId)
	if err != nil {
		s.log.Errorf("GetAccount failed: %v", err)
		return nil, err
	}

	reply := &v1.GetAccountReply{
		UserId:  req.UserId,
		Balance: balance.Balance,
		Quotas:  make([]*v1.FreeQuota, 0, len(quotas)),
	}

	for _, q := range quotas {
		reply.Quotas = append(reply.Quotas, &v1.FreeQuota{
			ServiceName: q.ServiceName,
			TotalQuota: int32(q.TotalQuota),
			UsedQuota:  int32(q.UsedQuota),
			ResetMonth: q.ResetMonth,
		})
	}

	return reply, nil
}

// Recharge 发起充值
func (s *BillingService) Recharge(ctx context.Context, req *v1.RechargeRequest) (*v1.RechargeReply, error) {
	orderID, payURL, err := s.uc.Recharge(ctx, req.UserId, req.Amount)
	if err != nil {
		s.log.Errorf("Recharge failed: %v", err)
		return nil, err
	}

	return &v1.RechargeReply{
		OrderId:     orderID,
		PaymentUrl:  payURL,
	}, nil
}

// ListRecords 获取消费流水
func (s *BillingService) ListRecords(ctx context.Context, req *v1.ListRecordsRequest) (*v1.ListRecordsReply, error) {
	page := int(req.Page)
	if page <= 0 {
		page = 1
	}
	pageSize := int(req.PageSize)
	if pageSize <= 0 {
		pageSize = 20
	}

	records, total, err := s.uc.ListRecords(ctx, req.UserId, page, pageSize)
	if err != nil {
		s.log.Errorf("ListRecords failed: %v", err)
		return nil, err
	}

	reply := &v1.ListRecordsReply{
		Total:    int32(total),
		Records:  make([]*v1.BillingRecord, 0, len(records)),
	}

	for _, r := range records {
		reply.Records = append(reply.Records, &v1.BillingRecord{
			Id:          r.ID,
			ServiceName: r.ServiceName,
			Type:        int32(r.Type),
			Amount:      r.Amount,
			Count:       int32(r.Count),
			CreatedAt:   timestamppb.New(r.CreatedAt),
		})
	}

	return reply, nil
}

