//go:build wireinject
// +build wireinject

package main

import (
	"os"

	"billing-service/internal/biz"
	"billing-service/internal/conf"
	"billing-service/internal/data"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/google/wire"
)

// CronApp Cron 应用结构
type CronApp struct {
	billingUsecase *biz.BillingUseCase
}

// wireApp 初始化应用
func wireApp(*conf.Bootstrap) (*CronApp, func(), error) {
	panic(wire.Build(
		// Logger
		newLogger,

		// Data 层（需要 conf.Data 和 logger）
		wire.FieldsOf(new(*conf.Bootstrap), "Data"),
		data.ProviderSet,

		// Biz 层（需要 repo, logger, config）
		// NewBillingConfig 需要 *conf.Bootstrap
		biz.ProviderSet,

		// App 结构
		wire.Struct(new(CronApp), "*"),
	))
}

// newLogger 创建 logger
func newLogger() log.Logger {
	return log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.name", "billing-cron",
	)
}
