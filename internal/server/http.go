package server

import (
	"billing-service/internal/conf"
	"billing-service/internal/service"

	"github.com/gaoyong06/go-pkg/health"
	"github.com/gaoyong06/go-pkg/middleware/cors"
	"github.com/gaoyong06/go-pkg/middleware/i18n"
	"github.com/gaoyong06/go-pkg/middleware/response"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/http"

	v1 "billing-service/api/billing/v1"
)

// NewHTTPServer new an HTTP server.
func NewHTTPServer(c *conf.Server, billing *service.BillingService, logger log.Logger) *http.Server {
	// 响应中间件配置
	responseConfig := &response.Config{
		EnableUnifiedResponse: true,
		IncludeDetailedError:  true, // 开发环境可以为 true
		IncludeHost:           true,
		IncludeTraceId:        true,
	}

	// 使用默认错误处理器（已支持 Kratos errors 的 HTTP 状态码映射）
	errorHandler := response.NewDefaultErrorHandler()

	var opts = []http.ServerOption{
		http.Middleware(
			recovery.Recovery(),
			// 添加 CORS 中间件
			cors.Middleware(cors.DefaultConfig()),
			// 添加 i18n 中间件
			i18n.Middleware(),
		),
		// 使用自定义响应编码器统一响应格式
		http.ResponseEncoder(response.NewResponseEncoder(errorHandler, responseConfig)),
		// 使用支持 gRPC status 的错误编码器
		http.ErrorEncoder(response.NewErrorEncoder(errorHandler)),
	}
	if c.Http.Network != "" {
		opts = append(opts, http.Network(c.Http.Network))
	}
	if c.Http.Addr != "" {
		opts = append(opts, http.Address(c.Http.Addr))
	}
	if c.Http.Timeout != nil {
		opts = append(opts, http.Timeout(c.Http.Timeout.AsDuration()))
	}
	srv := http.NewServer(opts...)

	// 注册外部服务路由（面向前端/开发者）
	v1.RegisterBillingServiceHTTPServer(srv, billing)
	
	// 注册内部服务路由（面向 Gateway/Payment）
	v1.RegisterBillingInternalServiceHTTPServer(srv, billing)

	// 注册健康检查端点
	srv.Route("/").GET("/health", func(ctx http.Context) error {
		return ctx.Result(200, health.NewResponse("billing-service"))
	})

	// 注册 Prometheus metrics 端点
	srv.Route("/").GET("/metrics", func(ctx http.Context) error {
		// Prometheus metrics 端点由 kratos 的 metrics 中间件自动处理
		// 如果需要自定义，可以在这里实现
		return nil
	})

	return srv
}
