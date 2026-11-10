package main

import (
	"flag"
	"os"

	"backend-service/app/edai/service/internal/runtimeconfig"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	_ "go.uber.org/automaxprocs"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name = "edai-service"
	// Version is the version of the compiled software.
	Version = "dev"

	// flagconf is the config flag.
	flagconf string
	hostID   string
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf config.yaml")
	var err error
	hostID, err = os.Hostname()
	if err != nil {
		hostID = "unknown"
	}
}

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server) *kratos.App {
	return kratos.New(
		kratos.ID(hostID),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(gs, hs),
	)
}

func main() {
	flag.Parse()
	logger := log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", hostID,
		"service.name", Name,
		"service.version", Version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)
	if err := run(logger); err != nil {
		log.NewHelper(logger).Fatalf("edai service stopped: %v", err)
	}
}

func run(logger log.Logger) error {
	bc, err := runtimeconfig.Load(flagconf)
	if err != nil {
		return err
	}
	app, cleanup, err := wireApp(bc, logger)
	if err != nil {
		return err
	}
	defer cleanup()

	// 启动即收口上次进程遗留的任务状态（4-2 ADR-013 决策 5）。
	// 不做「假装续跑」：中断点未知，标失败让用户重跑更可解释。
	if n, rerr := recoverInterruptedTasks(bc, logger); rerr != nil {
		log.NewHelper(logger).Warnf("recover interrupted scan tasks: %v", rerr)
	} else if n > 0 {
		log.NewHelper(logger).Infof("recovered %d interrupted scan task(s)", n)
	}

	return app.Run()
}
