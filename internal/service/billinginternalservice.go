package service

import (
	"context"

	pb "billing-service/api/billing/v1"
)

type BillingInternalServiceService struct {
	pb.UnimplementedBillingInternalServiceServer
}

func NewBillingInternalServiceService() *BillingInternalServiceService {
	return &BillingInternalServiceService{}
}

func (s *BillingInternalServiceService) CheckQuota(ctx context.Context, req *pb.CheckQuotaRequest) (*pb.CheckQuotaReply, error) {
    return &pb.CheckQuotaReply{}, nil
}
func (s *BillingInternalServiceService) DeductQuota(ctx context.Context, req *pb.DeductQuotaRequest) (*pb.DeductQuotaReply, error) {
    return &pb.DeductQuotaReply{}, nil
}
func (s *BillingInternalServiceService) RechargeCallback(ctx context.Context, req *pb.RechargeCallbackRequest) (*pb.RechargeCallbackReply, error) {
    return &pb.RechargeCallbackReply{}, nil
}
