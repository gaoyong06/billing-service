package service

import (
	"context"

	pb "billing-service/api/billing/v1"
	"billing-service/internal/biz"
	"billing-service/internal/constants"
	billingErrors "billing-service/internal/errors"

	pkgErrors "github.com/gaoyong06/go-pkg/errors"
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
	// 验证必填字段
	if req.UserId == "" {
		s.log.Warnf("GetAccount: userId is empty")
		return nil, pkgErrors.NewBizErrorWithLang(ctx, pkgErrors.ErrCodeMissingRequiredField)
	}

	balance, quotas, err := s.uc.GetAccount(ctx, req.UserId)
	if err != nil {
		s.log.Errorf("GetAccount failed: userId=%s, error=%v", req.UserId, err)
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
		UserId:  balance.UID,
		Balance: balance.Balance,
		Quotas:  pbQuotas,
	}, nil
}

// Recharge 发起充值
func (s *BillingService) Recharge(ctx context.Context, req *pb.RechargeRequest) (*pb.RechargeReply, error) {
	// 将 payment_method 字符串转换为 PaymentMethod 枚举
	// payment_method: "alipay" -> 1, "wechatpay" -> 2, 默认 -> 1 (alipay)
	method := int32(1) // 默认支付宝
	if req.PaymentMethod == constants.PaymentMethodWechat {
		method = 2 // 微信支付
	} else if req.PaymentMethod == constants.PaymentMethodAlipay {
		method = 1 // 支付宝
	}

	// 从 context 中获取客户端 IP（如果有）
	// clientIP := ""
	// if ip := ctx.Value("client_ip"); ip != nil {
	// 	if ipStr, ok := ip.(string); ok {
	// 		clientIP = ipStr
	// 	}
	// }

	// 验证币种必填
	if req.Currency == "" {
		return nil, pkgErrors.NewBizErrorWithLang(ctx, billingErrors.ErrCodeCurrencyRequired)
	}

	// TODO: 从配置中获取 return_url 和 notify_url
	returnURL := "" // 从配置中获取
	notifyURL := "" // 从配置中获取

	orderID, payURL, err := s.uc.Recharge(ctx, req.UserId, req.Amount, method, req.Currency, returnURL, notifyURL)
	if err != nil {
		return nil, err
	}
	return &pb.RechargeReply{
		RechargeOrderId: orderID,
		PaymentUrl:      payURL,
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
		// 将字符串类型转换为 int32（兼容 proto 定义）
		// "free" -> 1, "balance" -> 2
		var typeInt int32
		if r.Type == constants.BillingTypeFree {
			typeInt = 1
		} else if r.Type == constants.BillingTypeBalance {
			typeInt = 2
		}
		pbRecords = append(pbRecords, &pb.BillingRecord{
			Id:          r.ID,
			ServiceName: r.ServiceName,
			Type:        typeInt,
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
		// 记录错误日志，便于排查问题
		s.log.Errorf("DeductQuota failed: user_id=%s, service=%s, count=%d, error=%v",
			req.UserId, req.ServiceName, req.Count, err)
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
	if req.Status != constants.PaymentStatusSuccess {
		return &pb.RechargeCallbackReply{Success: true}, nil // 支付失败，直接返回成功（已处理）
	}

	err := s.uc.RechargeCallback(ctx, req.RechargeOrderId, req.Amount)
	if err != nil {
		return &pb.RechargeCallbackReply{Success: false}, err
	}
	return &pb.RechargeCallbackReply{Success: true}, nil
}

// GetStatsToday 获取今日调用统计
func (s *BillingService) GetStatsToday(ctx context.Context, req *pb.GetStatsTodayRequest) (*pb.GetStatsReply, error) {
	stats, err := s.uc.GetStatsToday(ctx, req.UserId, req.ServiceName)
	if err != nil {
		return nil, err
	}

	return &pb.GetStatsReply{
		UserId:      stats.UID,
		ServiceName: stats.ServiceName,
		TotalCount:  int32(stats.TotalCount),
		TotalCost:   stats.TotalCost,
		FreeCount:   int32(stats.FreeCount),
		PaidCount:   int32(stats.PaidCount),
		Period:      stats.Period,
	}, nil
}

// GetStatsMonth 获取本月调用统计
func (s *BillingService) GetStatsMonth(ctx context.Context, req *pb.GetStatsMonthRequest) (*pb.GetStatsReply, error) {
	stats, err := s.uc.GetStatsMonth(ctx, req.UserId, req.ServiceName)
	if err != nil {
		return nil, err
	}

	return &pb.GetStatsReply{
		UserId:      stats.UID,
		ServiceName: stats.ServiceName,
		TotalCount:  int32(stats.TotalCount),
		TotalCost:   stats.TotalCost,
		FreeCount:   int32(stats.FreeCount),
		PaidCount:   int32(stats.PaidCount),
		Period:      stats.Period,
	}, nil
}

// GetStatsSummary 获取汇总统计（所有服务）
func (s *BillingService) GetStatsSummary(ctx context.Context, req *pb.GetStatsSummaryRequest) (*pb.GetStatsSummaryReply, error) {
	summary, err := s.uc.GetStatsSummary(ctx, req.UserId)
	if err != nil {
		return nil, err
	}

	pbServices := make([]*pb.ServiceStats, 0, len(summary.Services))
	for _, svc := range summary.Services {
		pbServices = append(pbServices, &pb.ServiceStats{
			ServiceName: svc.ServiceName,
			TotalCount:  int32(svc.TotalCount),
			TotalCost:   svc.TotalCost,
			FreeCount:   int32(svc.FreeCount),
			PaidCount:   int32(svc.PaidCount),
		})
	}

	return &pb.GetStatsSummaryReply{
		UserId:     summary.UID,
		TotalCount: int32(summary.TotalCount),
		TotalCost:  summary.TotalCost,
		Services:   pbServices,
	}, nil
}
