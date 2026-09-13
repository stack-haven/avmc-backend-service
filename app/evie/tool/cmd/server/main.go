// Package main · main.go
// evie/tool 服务入口。
//
// 启动流程：
//  1. 加载 config.yaml
//  2. 构造 Kratos logger
//  3. wireApp 装配所有组件 + 启动 VocabSyncer（BeforeStart 钩子）
//  4. app.Run() 阻塞直到 ctx cancel 或信号
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
	"backend-service/app/evie/tool/internal/logging"

	_ "go.uber.org/automaxprocs"
)

// 编译期 ldflags 可注入
var (
	Name     = "evie-tool"
	Version  = "0.1.0"
	flagconf string
	id, _    = os.Hostname()
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf config.yaml")
}

// newApp 装配 Kratos App；通过 BeforeStart 根据 syncer.SyncMode() 决定启动行为。
//
// v1.2 sync 模式：
//   - "admin"（qua.admin_token 有）：启动 goroutine 调 Warmup + Run（后台周期拉）
//   - "lazy_only"（qua.admin_token 无）：不启动后台 goroutine，靠请求路径 cache miss/TTL 过期触发 sync
func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server, syncer *biz.VocabSyncer) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{"service.group": "evie"}),
		kratos.Logger(logger),
		kratos.Server(gs, hs),
		kratos.BeforeStart(func(ctx context.Context) error {
			// v1.2: 只在 admin_token 模式下启动后台 goroutine
			if syncer.SyncMode() != "admin" {
				logger.Log(log.LevelInfo, "msg", "vocab sync: lazy_only mode (no admin_token), skip background sync")
				return nil
			}
			// admin_token 模式：Warmup（10s）+ 后台 ticker
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
	logger := logging.New(logging.DetectFormat(),
		"ts", log.DefaultTimestamp,
		"caller", log.DefaultCaller,
		"service.id", id,
		"service.name", Name,
		"service.version", Version,
		"trace.id", tracing.TraceID(),
		"span.id", tracing.SpanID(),
	)
	// 日志脱敏（默认启用）；EVIE_TOOL_LOG_REDACT=off 关闭（仅调试）
	if logging.ShouldRedact() {
		logger = logging.NewRedactingLogger(logger)
	}

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
	// Demo 模式：环境变量 EVIE_TOOL_DEMO=1 触发；不连 Redis/qua，仅文本增强。
	if os.Getenv("EVIE_TOOL_DEMO") == "1" {
		app, cleanup, err := runDemoApp(&bc, logger)
		if err != nil {
			panic(err)
		}
		defer cleanup()
		if err := app.Run(); err != nil {
			panic(err)
		}
		return
	}

	// 启动期配置校验：尽早失败，避免半配置状态运行。
	if err := conf.Validate(&bc); err != nil {
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
