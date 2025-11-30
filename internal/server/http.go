package server

import (
	v1 "billing-service/api/billing/v1"
	"billing-service/internal/conf"
	"billing-service/internal/service"

	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"
)

// NewHTTPServer 创建 HTTP 服务器
func NewHTTPServer(c *conf.Bootstrap, billingService *service.BillingService) *http.Server {
	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
		),
	}
	if c.Server != nil && c.Server.Http != nil {
		if c.Server.Http.Network != "" {
			opts = append(opts, http.Network(c.Server.Http.Network))
		}
		if c.Server.Http.Addr != "" {
			opts = append(opts, http.Address(c.Server.Http.Addr))
		}
		if c.Server.Http.Timeout != nil {
			opts = append(opts, http.Timeout(c.Server.Http.Timeout.AsDuration()))
		}
	}
	srv := http.NewServer(opts...)
	v1.RegisterBillingServiceHTTPServer(srv, billingService)
	return srv
}
