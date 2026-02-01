package main

import (
	"flag"
	"fmt"
	"os"

	"billing-service/internal/conf"

	"billing-service/internal/server"

	"github.com/gaoyong06/go-pkg/logger"
	pkgutils "github.com/gaoyong06/go-pkg/utils"
	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"

	_ "go.uber.org/automaxprocs"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	Name     = "billing-service"
	Version  = "v1.0.0"
	flagconf string
	runMode  string
	id, _    = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "", "config path, eg: -conf config.yaml (deprecated, use -mode instead)")
	flag.StringVar(&runMode, "mode", "debug", "Run mode (debug, release)")
}

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server, mq *server.MQConsumerServer) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			gs,
			hs,
			mq,
		),
	)
}

func main() {
	flag.Parse()

	// 根据 mode 自动选择配置文件
	configPath := flagconf
	if configPath == "" {
		// 使用 go-pkg/utils 中的通用配置文件路径解析函数
		// 支持从不同目录运行（项目根目录、cmd/server 目录等）
		configPath = pkgutils.FindConfigFileWithMode(runMode, []string{
			"configs",       // 从项目根目录运行
			"../../configs", // 从 cmd/server 目录运行
			"../configs",    // 从 cmd 目录运行
		})
	}

	// 初始化 Kratos Config
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

	// 验证配置
	if err := bc.Validate(); err != nil {
		panic(fmt.Sprintf("config validation failed: %v", err))
	}

	// 初始化日志 (使用 go-pkg/logger)
	// 优先使用配置文件中的日志配置，如果未配置则使用默认值
	logConfig := &logger.Config{
		Level:         "info",
		Format:        "json",
		Output:        "file", // 默认为 file，生产环境安全
		FilePath:      "logs/billing-service.log",
		MaxSize:       100,
		MaxAge:        30,
		MaxBackups:    10,
		Compress:      true,
		EnableConsole: true,
	}

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
		if bc.Log.ServerFilePath != "" {
			logConfig.FilePath = bc.Log.ServerFilePath
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
		// bool 类型无法区分默认 false 还是显式 false，这里直接赋值，或者认为是可选配置
		logConfig.Compress = bc.Log.Compress
	}

	loggerInstance := logger.NewLogger(logConfig)

	// 添加基本字段
	loggerInstance = log.With(loggerInstance,
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", id,
		"service.name", Name,
		"service.version", Version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)

	app, cleanup, err := wireApp(bc.Server, bc.Data, &bc, loggerInstance)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// start and wait for stop signal
	if err := app.Run(); err != nil {
		panic(err)
	}
}
