// Package main · main.go
// evie/tool 服务入口（v1.6 减法后）。
//
// 启动流程：
//  1. 加载 config.yaml
//  2. 构造 Kratos logger
//  3. wireApp 装配所有组件
//  4. app.Run() 阻塞直到 ctx cancel 或信号
//
// v1.6 减法：
//   - 删除 BeforeStart 中的 Warmup/Run 钩子（无后台 sync）
//   - 删除 TenantRegistry conf 引用
//   - 删除 wireApp 的 tenantRegistry 参数
//
// sync 策略（纯 lazy + TTL）：
//   - 首次带 token 请求 → VocabSyncer.EnsureTenant 同步拉 qua
//   - TTL 默认 5min（vocabSnapshotTTL）→ 过期后下次请求重新拉
package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware/tracing"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"github.com/go-kratos/kratos/v2/transport/http"

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

// newApp 装配 Kratos App。
//
// 无后台 sync 钩子：所有 vocab 同步都在请求路径（EnsureTenant）按需触发。
func newApp(logger log.Logger, gs *grpc.Server, hs *http.Server) *kratos.App {
	return kratos.New(
		kratos.ID(id),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{"service.group": "evie"}),
		kratos.Logger(logger),
		kratos.Server(gs, hs),
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

	// 装配所有组件（无 BeforeStart 钩子 — 所有同步走请求路径）
	app, cleanup, err := wireApp(bc.Server, bc.Data, bc.Asr, bc.Qua, bc.Enhancement, bc.TenantVocab, bc.SystemDict, bc.VocabRules, logger)
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
