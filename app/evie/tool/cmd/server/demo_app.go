package main

import (
	"context"
	"errors"
	"os"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/internal/data"
	"backend-service/app/evie/tool/internal/server"
	"backend-service/app/evie/tool/internal/service"
	pkgHealth "backend-service/pkg/health"
)

// runDemoApp 构造零外部依赖的 demo 进程。
//
// 触发条件：环境变量 EVIE_TOOL_DEMO=1。
// 行为：
//   - Token 走 static（conf.Credential.Static）；
//   - 系统词条走 conf.SystemDict.path；
//   - 词库数据走 conf.Vocabulary.File.path；
//   - 不连接 Redis、不调用 qua；
//   - 仅暴露 EnhancementService（文本增强）和基础健康检查。
//
// 注意：当前未注册 ASRService；调用 ASR 端点会得到 404。
func runDemoApp(bc *conf.Bootstrap, logger log.Logger) (*kratos.App, func(), error) {
	if bc.Credential == nil || bc.Credential.Provider != "static" || bc.Credential.Static == nil {
		return nil, nil, errDemoCredentialMissing
	}
	if bc.Vocabulary == nil || bc.Vocabulary.Source != "file" || bc.Vocabulary.File == nil {
		return nil, nil, errDemoVocabularyMissing
	}

	tokenCache, err := data.NewStaticTokenCache(bc.Credential.Static)
	if err != nil {
		return nil, nil, err
	}

	// 构造系统词库 + lexnorm 引擎
	vb, err := biz.NewVocabularyBuilder(bc.SystemDict)
	if err != nil {
		return nil, nil, err
	}
	engine, err := biz.NewLexnormEngine(bc.Enhancement, vb, logger)
	if err != nil {
		return nil, nil, err
	}
	enhancementUC := biz.NewEnhancementUsecase(engine)
	enhancementSvc := service.NewEnhancementService(enhancementUC, logger)

	checker := &trivialChecker{}
	httpSrv, err := buildDemoHTTP(bc.Server, tokenCache, enhancementSvc, checker, logger)
	if err != nil {
		return nil, nil, err
	}

	app := kratos.New(
		kratos.ID("evie-tool-demo"),
		kratos.Name(Name),
		kratos.Version(Version),
		kratos.Metadata(map[string]string{"service.group": "evie", "mode": "demo"}),
		kratos.Logger(logger),
		kratos.Server(httpSrv),
	)
	return app, func() {}, nil
}

var (
	errDemoCredentialMissing = errors.New("demo mode requires conf.Credential.provider=static")
	errDemoVocabularyMissing = errors.New("demo mode requires conf.Vocabulary.source=file")
)

// buildDemoHTTP 复用 server.NewHTTPServer 构造 demo 路由。
//
// ASRService 传 nil，server 在 nil 时跳过注册。
func buildDemoHTTP(
	c *conf.Server,
	cache data.TokenLookup,
	enhSvc *service.EnhancementService,
	checker pkgHealth.Checker,
	logger log.Logger,
) (*khttp.Server, error) {
	return server.NewHTTPServer(c, cache, enhSvc, nil, checker, logger), nil
}

// trivialChecker demo 用健康检查器：永远 ready。
type trivialChecker struct{}

// Ready 总是返回 nil。
func (trivialChecker) Ready(ctx context.Context) error { return nil }

// Details 返回 demo 标志。
func (trivialChecker) Details(ctx context.Context) map[string]any {
	return map[string]any{"demo": true}
}

// compile-time 断言。
var _ pkgHealth.Checker = (*trivialChecker)(nil)
var _ = os.Getenv // keep os imported (used by caller)
