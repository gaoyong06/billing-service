package server

import (
	v1 "billing-service/api/billing/v1"
	"billing-service/internal/conf"
	"billing-service/internal/service"

	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/google/wire"
)

// ProviderSet is server providers.
var ProviderSet = wire.NewSet(
	NewHTTPServer,
	NewGRPCServer,
)

// NewGRPCServer 创建 gRPC 服务器
func NewGRPCServer(c *conf.Bootstrap, billingService *service.BillingService, billingInternalService *service.BillingInternalService) *grpc.Server {
	var opts = []grpc.ServerOption{
		grpc.Middleware(
			recovery.Recovery(),
		),
	}
	if c.Server != nil && c.Server.Grpc != nil {
		if c.Server.Grpc.Network != "" {
			opts = append(opts, grpc.Network(c.Server.Grpc.Network))
		}
		if c.Server.Grpc.Addr != "" {
			opts = append(opts, grpc.Address(c.Server.Grpc.Addr))
		}
		if c.Server.Grpc.Timeout != nil {
			opts = append(opts, grpc.Timeout(c.Server.Grpc.Timeout.AsDuration()))
		}
	}
	srv := grpc.NewServer(opts...)
	v1.RegisterBillingServiceServer(srv, billingService)
	v1.RegisterBillingInternalServiceServer(srv, billingInternalService)
	return srv
}
