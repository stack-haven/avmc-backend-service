// Package main · main.go
// evie/tool 服务入口。
//
// 启动流程：
//   1. 加载 config.yaml
//   2. 构造 Kratos logger
//   3. wireApp 装配所有组件 + 启动 VocabSyncer（BeforeStart 钩子）
//   4. app.Run() 阻塞直到 ctx cancel 或信号
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"

	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/conf"

	_ "go.uber.org/automaxprocs"
)

// 编译期 ldflags 可注入
var (
	Name    = "evie-tool"
	Version = "0.1.0"
	flagconf string
	id, _   = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf config.yaml")
}

// newApp 装配 Kratos App；通过 BeforeStart 启动 VocabSyncer。
func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server, syncer *biz.VocabSyncer) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{"service.group": "evie"}),
		kratos.Logger(logger),
		kratos.Server(gs, hs),
		kratos.BeforeStart(func(ctx context.Context) error {
			// 启动 VocabSyncer 后台 worker
			// 1) 立即 warmup（10s timeout）
			// 2) 后台 ticker 循环
			go func() {
				warmupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
				syncer.Warmup(warmupCtx)
				cancel()
				// ticker 循环（内部监听 ctx.Done）
				syncer.Run(ctx)
			}()
			return nil
		}),
		kratos.AfterStop(func(_ context.Context) error {
			// syncer.Run 内部监听 ctx.Done；Kratos 关闭 ctx 时会优雅退出
			return nil
		}),
	)
}

func main() {
	flag.Parse()
	logger := log.With(log.NewStdLogger(os.Stdout),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", id,
		"service.name", Name,
		"service.version", Version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)

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

	// 装配所有组件（含 VocabSyncer）；通过 BeforeStart 钩子启动
	app, cleanup, err := wireApp(bc.Server, bc.Data, bc.Asr, bc.Qua, bc.Enhancement, bc.TenantVocab, bc.SystemDict, bc.TenantRegistry, bc.VocabRules, logger)
	if err != nil {
		panic(err)
	}
	defer cleanup()

	// 信号处理（优雅关闭）
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	if err := app.Run(); err != nil {
		panic(err)
	}
	_ = sigCh
}