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
	pkgutils "github.com/gaoyong06/go-pkg/utils"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/robfig/cron/v3"
	_ "go.uber.org/automaxprocs"
)

var (
	flagconf string
	runMode  string
)

func init() {
	flag.StringVar(&flagconf, "conf", "", "config path, eg: -conf config.yaml (deprecated, use -mode instead)")
	flag.StringVar(&runMode, "mode", "debug", "Run mode (debug, release)")
}

func main() {
	flag.Parse()

	// 根据 mode 自动选择配置文件
	configPath := flagconf
	if configPath == "" {
		// 使用 go-pkg/utils 中的通用配置文件路径解析函数
		// 支持从不同目录运行（项目根目录、cmd/scheduler 目录等）
		configPath = pkgutils.FindConfigFileWithMode(runMode, []string{
			"configs",       // 从项目根目录运行
			"../../configs", // 从 cmd/scheduler 目录运行
			"../configs",    // 从 cmd 目录运行
		})
	}

	// 初始化配置
	c := config.New(
		config.WithSource(
			file.NewSource(configPath),
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
	// 将启动参数中的 mode 注入到配置中
	bc.RunMode = runMode

	// 初始化日志 (使用 go-pkg/logger)
	logConfig := &logger.Config{
		Level:         "info",
		Format:        "json",
		Output:        "file", // 默认为 file
		FilePath:      "logs/billing-scheduler.log",
		MaxSize:       100,
		MaxAge:        30,
		MaxBackups:    10,
		Compress:      true,
		EnableConsole: true,
	}

	// 如果配置文件中有日志配置，则覆盖默认配置
	// 注意：为了区分日志文件，我们保留 FilePath 为 billing-scheduler.log，不使用配置文件中的 file_path
	if bc.Log != nil {
		if bc.Log.Level != "" {
			logConfig.Level = bc.Log.Level
		}
		if bc.Log.Format != "" {
			logConfig.Format = bc.Log.Format
		}
		if bc.Log.Output != "" {
			logConfig.Output = bc.Log.Output
		}
		if bc.Log.SchedulerFilePath != "" {
			logConfig.FilePath = bc.Log.SchedulerFilePath
		}
		if bc.Log.MaxSize != 0 {
			logConfig.MaxSize = int(bc.Log.MaxSize)
		}
		if bc.Log.MaxAge != 0 {
			logConfig.MaxAge = int(bc.Log.MaxAge)
		}
		if bc.Log.MaxBackups != 0 {
			logConfig.MaxBackups = int(bc.Log.MaxBackups)
		}
		logConfig.Compress = bc.Log.Compress
	}

	loggerInstance := logger.NewLogger(logConfig)

	// 添加基本字段
	loggerInstance = log.With(loggerInstance,
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.name", "billing-scheduler",
	)

	logHelper := log.NewHelper(loggerInstance)

	// Log that the service is starting and log file path
	_ = loggerInstance.Log(log.LevelInfo, "msg", "billing-scheduler starting", "log_file", logConfig.FilePath, "run_mode", runMode)

	// 初始化应用
	app, cleanup, err := wireApp(&bc)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// 创建定时任务调度器（支持秒级调度）
	cronScheduler := cron.New(cron.WithSeconds())

	// 注册免费额度重置任务
	if bc.Scheduler != nil && bc.Scheduler.FreeQuotaResetTask != nil {
		task := bc.Scheduler.FreeQuotaResetTask
		if task.Enabled {
			cronExpr := task.Cron
			if cronExpr == "" {
				cronExpr = "0 0 0 1 * *" // 默认每月1日 00:00 执行
			}

			_, err := cronScheduler.AddFunc(cronExpr, func() {
				logHelper.Info("[SCHEDULER] Starting free quota reset...")
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
				defer cancel()

				count, userIDs, err := app.billingUsecase.ResetFreeQuotas(ctx)
				if err != nil {
					logHelper.Errorf("[SCHEDULER] Error resetting free quotas: %v", err)
				} else {
					logHelper.Infof("[SCHEDULER] Reset free quotas completed: count=%d, users=%d", count, len(userIDs))
					if len(userIDs) > 0 && len(userIDs) <= 10 {
						logHelper.Infof("[SCHEDULER] Reset users: %v", userIDs)
					} else if len(userIDs) > 10 {
						logHelper.Infof("[SCHEDULER] Reset users (first 10): %v", userIDs[:10])
					}
					logHelper.Info("[SCHEDULER] Finished free quota reset")
				}
			})
			if err != nil {
				logHelper.Errorf("Failed to add free quota reset job: %v", err)
				panic(err)
			}

			logHelper.Infof("Free quota reset task registered: cron=%s", cronExpr)
		} else {
			logHelper.Info("Free quota reset task is disabled")
		}
	} else {
		logHelper.Warn("Scheduler configuration not found, using default cron expression")
		// 如果没有配置，使用默认值
		_, err := cronScheduler.AddFunc("0 0 0 1 * *", func() {
			logHelper.Info("[SCHEDULER] Starting free quota reset...")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			defer cancel()

			count, userIDs, err := app.billingUsecase.ResetFreeQuotas(ctx)
			if err != nil {
				logHelper.Errorf("[SCHEDULER] Error resetting free quotas: %v", err)
			} else {
				logHelper.Infof("[SCHEDULER] Reset free quotas completed: count=%d, users=%d", count, len(userIDs))
				if len(userIDs) > 0 && len(userIDs) <= 10 {
					logHelper.Infof("[SCHEDULER] Reset users: %v", userIDs)
				} else if len(userIDs) > 10 {
					logHelper.Infof("[SCHEDULER] Reset users (first 10): %v", userIDs[:10])
				}
				logHelper.Info("[SCHEDULER] Finished free quota reset")
			}
		})
		if err != nil {
			logHelper.Errorf("Failed to add free quota reset job: %v", err)
			panic(err)
		}
	}

	// 启动定时任务
	cronScheduler.Start()
	logHelper.Info("========================================")
	logHelper.Info("Scheduler started successfully")
	logHelper.Info("Scheduled jobs:")
	if bc.Scheduler != nil && bc.Scheduler.FreeQuotaResetTask != nil && bc.Scheduler.FreeQuotaResetTask.Enabled {
		logHelper.Infof("  - Free quota reset: %s", bc.Scheduler.FreeQuotaResetTask.Cron)
	}
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
		logHelper.Info("Scheduler stopped gracefully")
	case <-time.After(5 * time.Second):
		logHelper.Info("Scheduler forced to stop after timeout")
	}
}
