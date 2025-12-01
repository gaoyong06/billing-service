package service

import (
	"context"

	pb "billing-service/api/billing/v1"
)

type BillingServiceService struct {
	pb.UnimplementedBillingServiceServer
}

func NewBillingServiceService() *BillingServiceService {
	return &BillingServiceService{}
}

func (s *BillingServiceService) GetAccount(ctx context.Context, req *pb.GetAccountRequest) (*pb.GetAccountReply, error) {
    return &pb.GetAccountReply{}, nil
}
func (s *BillingServiceService) Recharge(ctx context.Context, req *pb.RechargeRequest) (*pb.RechargeReply, error) {
    return &pb.RechargeReply{}, nil
}
func (s *BillingServiceService) ListRecords(ctx context.Context, req *pb.ListRecordsRequest) (*pb.ListRecordsReply, error) {
    return &pb.ListRecordsReply{}, nil
}
