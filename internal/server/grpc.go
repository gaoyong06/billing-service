package server

import (
	"billing-service/internal/conf"
	"billing-service/internal/service"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"

	v1 "billing-service/api/billing/v1"
)

// NewGRPCServer new a gRPC server.
func NewGRPCServer(c *conf.Server, billing *service.BillingService, logger log.Logger) *grpc.Server {
	var opts = []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
		),
	}
	if c.Grpc.Network != "" {
		opts = append(opts, grpc.Network(c.Grpc.Network))
	}
	if c.Grpc.Addr != "" {
		opts = append(opts, grpc.Address(c.Grpc.Addr))
	}
	if c.Grpc.Timeout != nil {
		opts = append(opts, grpc.Timeout(c.Grpc.Timeout.AsDuration()))
	}
	srv := grpc.NewServer(opts...)
	
	// 注册外部服务（面向前端/开发者）
	v1.RegisterBillingServiceServer(srv, billing)
	
	// 注册内部服务（面向 Gateway/Payment）
	v1.RegisterBillingInternalServiceServer(srv, billing)
	
	return srv
}
