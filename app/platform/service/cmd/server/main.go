package main

import (
	"flag"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"
	_ "go.uber.org/automaxprocs"

	"backend-service/app/platform/service/internal/runtimeconfig"
	"backend-service/app/platform/service/internal/server"
)

// go build -ldflags "-X main.Version=x.y.z"
var (
	// Name is the name of the compiled software.
	Name string
	// Version is the version of the compiled software.
	Version string
	// flagconf is the config flag.
	flagconf string

	hostID string
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf config.yaml")

	var err error
	hostID, err = os.Hostname()
	if err != nil {
		hostID = "unknown"
	}
}

func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server, worker *server.AsyncTaskWorker) *kratos.App {
	return kratos.New(
		kratos.ID(hostID),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{}),
		kratos.Logger(logger),
		kratos.Server(
			gs,
			hs,
			worker,
		),
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
		log.NewHelper(logger).Fatalf("admin service stopped: %v", err)
	}
}

func run(logger log.Logger) error {
	bc, err := runtimeconfig.Load(flagconf)
	if err != nil {
		return err
	}
	app, cleanup, err := wireApp(bc.Server, bc.Data, bc.Oss, logger)
	if err != nil {
		return err
	}
	defer cleanup()

	// start and wait for stop signal
	return app.Run()
}
