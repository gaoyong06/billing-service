package main

import (
	"flag"
	"os"

	"billing-service/internal/conf"

	"billing-service/internal/server"

	"github.com/gaoyong06/go-pkg/logger"
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
	id, _    = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf config.yaml")
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

	// 初始化 Kratos Config
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
		Level:         "info",
		Format:        "json",
		Output:        "stdout",
		FilePath:      "logs/billing-service.log",
		MaxSize:       100,
		MaxAge:        30,
		MaxBackups:    10,
		Compress:      true,
		EnableConsole: true,
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
