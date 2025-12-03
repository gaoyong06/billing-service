package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"billing-service/internal/conf"

	"github.com/gaoyong06/go-pkg/logger"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/robfig/cron/v3"
	_ "go.uber.org/automaxprocs"
)

var (
	flagconf string
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs/config.yaml", "config path, eg: -conf config.yaml")
}

func main() {
	flag.Parse()

	// 初始化配置
	c := config.New(
		config.WithSource(
			file.NewSource(flagconf),
		),
	)
	defer c.Close()

	if err := c.Load(); err != nil {
		panic(err)
	}

	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		panic(err)
	}

	// 初始化日志 (使用 go-pkg/logger)
	logConfig := &logger.Config{
		Level:        "info",
		Format:       "json",
		Output:       "stdout",
		FilePath:     "logs/billing-cron.log",
		MaxSize:     100,
		MaxAge:       30,
		MaxBackups:   10,
		Compress:     true,
		EnableConsole: true,
	}

	loggerInstance := logger.NewLogger(logConfig)

	// 添加基本字段
	loggerInstance = log.With(loggerInstance,
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.name", "billing-cron",
	)

	logHelper := log.NewHelper(loggerInstance)

	// 初始化应用
	app, cleanup, err := wireApp(&bc)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// 创建定时任务调度器（支持秒级调度）
	cronScheduler := cron.New(cron.WithSeconds())

	// 免费额度重置 - 每月1日 00:00 执行
	_, err = cronScheduler.AddFunc("0 0 0 1 * *", func() {
		logHelper.Info("[CRON] Starting free quota reset...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()

		count, userIDs, err := app.billingUsecase.ResetFreeQuotas(ctx)
		if err != nil {
			logHelper.Errorf("[CRON] Error resetting free quotas: %v", err)
		} else {
			logHelper.Infof("[CRON] Reset free quotas completed: count=%d, users=%d", count, len(userIDs))
			if len(userIDs) > 0 && len(userIDs) <= 10 {
				logHelper.Infof("[CRON] Reset users: %v", userIDs)
			} else if len(userIDs) > 10 {
				logHelper.Infof("[CRON] Reset users (first 10): %v", userIDs[:10])
			}
			logHelper.Info("[CRON] Finished free quota reset")
		}
	})
	if err != nil {
		logHelper.Errorf("Failed to add free quota reset job: %v", err)
	}

	// 启动定时任务
	cronScheduler.Start()
	logHelper.Info("========================================")
	logHelper.Info("Cron jobs started successfully")
	logHelper.Info("Scheduled jobs:")
	logHelper.Info("  - Free quota reset: Every month on the 1st at 00:00")
	logHelper.Info("========================================")

	// 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logHelper.Info("Shutting down gracefully...")

	// 停止定时任务
	ctx := cronScheduler.Stop()
	select {
	case <-ctx.Done():
		logHelper.Info("Cron jobs stopped gracefully")
	case <-time.After(5 * time.Second):
		logHelper.Info("Cron jobs forced to stop after timeout")
	}
}
