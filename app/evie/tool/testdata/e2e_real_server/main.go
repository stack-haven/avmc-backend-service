// e2e_real_server.go
//
// 真实后台 HTTP server：miniredis + 真实 qua + 真实 funasr
// 启动后监听 127.0.0.1:5599，可用真实 curl 请求。
//
// 运行：go run ./app/evie/tool/testdata/e2e_real_server/main.go
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/types/known/durationpb"

	v1 "backend-service/api/evie/tool/v1"
	pkgHealth "backend-service/pkg/health"
	v1conf "backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/data"
	"backend-service/app/evie/tool/internal/server"
	"backend-service/app/evie/tool/internal/service"
	"backend-service/pkg/textenhance"
	"backend-service/pkg/textenhance/builtins"
)

const (
	testToken    = "6dcd5e06b0284b3eb572322c5ac71e50"
	testTenantID = "1889501240003497986"
	listenAddr   = "127.0.0.1:5599"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mr, err := run(ctx, cancel)
	if err != nil {
		fmt.Printf("✗ failed: %v\n", err)
		os.Exit(1)
	}
	if mr != nil {
		defer mr.Close()
	}

	// 等待 SIGINT/SIGTERM
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println("\nshutting down...")
	cancel()
}

func run(ctx context.Context, cancel context.CancelFunc) (*miniredis.Miniredis, error) {
	// 1. miniredis
	mr, err := miniredis.Run()
	if err != nil {
		return nil, fmt.Errorf("miniredis: %w", err)
	}
	mr.Set("oauth2_access_token:"+testToken, fmt.Sprintf(`{
		"tenantId": "%s",
		"id": "u-prod-1",
		"accessToken": "%s",
		"userId": "u-prod-1",
		"userType": 2,
		"userInfo": {"nickname": "生产验证", "deptId": "1904450235179954177"}
	}`, testTenantID, testToken))
	fmt.Printf("✓ miniredis up at %s\n", mr.Addr())

	// 2. configs
	// 重要：业务 binary 启动时 cwd 是 backend-service 根目录
	dictPath, _ := filepath.Abs("./app/evie/tool/configs/dictionaries/system.json")
	audioDir, err := os.MkdirTemp("", "evie-tool-audio-")
	if err != nil {
		return mr, err
	}
	defer os.RemoveAll(audioDir)
	fmt.Printf("✓ audio dir: %s\n", audioDir)

	conf := &v1conf.Bootstrap{
		Data: &v1conf.Data{Redis: &v1conf.Data_Redis{
			Network: "tcp", Addr: mr.Addr(),
			TokenKeyPrefix: "oauth2_access_token:",
		}},
		SystemDict: &v1conf.SystemDict{Path: dictPath},
		Qua: &v1conf.Qua{
			BaseUrl: "http://api.bdksim-pro.test.bedoke.com",
			Timeout: durationpb.New(10 * time.Second),
			Endpoints: &v1conf.Qua_Endpoints{
				ListUsers: "/admin-api/qua/member-extended/page",
				ListDepts: "/admin-api/system/dept/list",
			},
			ExtraHeaders: map[string]string{"zone": "Asia/Shanghai"},
		},
		Enhancement: &v1conf.Enhancement{
			Pipeline: []string{
				"cleaning", "filler", "vocab_matching", "alias_resolution",
				"deterministic_replacement", "phrase_standardization",
				"pinyin_correction", "fuzzy_matching", "context_correction",
			},
		},
		Asr: &v1conf.Asr{
			DefaultBatchProvider: "funasr",
			Upload:               &v1conf.Asr_Upload{AudioDir: audioDir},
			Providers: &v1conf.Asr_Providers{
				Funasr: &v1conf.Asr_Provider{
					Enabled:    true,
					Addr:       "http://127.0.0.1:18000",
					SampleRate: 16000,
					Language:   "zh",
				},
			},
		},
	}

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	tc := data.NewTokenCache(rdb, conf.Data.Redis)

	// 3. qua client（真实接口）
	quaClient, err := data.NewQuaClient(conf.Qua, log.DefaultLogger)
	if err != nil {
		return mr, err
	}
	quaSource := data.NewQuaVocabularySource(quaClient)

	// 4. vocab builder + normalizer + syncer
	vb, err := biz.NewVocabularyBuilder(conf.SystemDict)
	if err != nil {
		return mr, err
	}
	normalizer := biz.NewNormalizer(defaultRuleSet())

	registry := biz.NewTenantRegistry(conf.TenantRegistry)
	registry.Ensure(testTenantID)
	syncer := biz.NewVocabSyncer(registry, vb, normalizer, quaSource, &v1conf.TenantVocab{
		SyncInterval:        durationpb.New(60 * time.Second),
		InitialWarmup:       false,
		Concurrency:         1,
		IncludeDepartments:  true,
		IncludeUserNickname: true,
		IncludeUserRealname: true,
		CustomAliasField:    "alias",
	}, log.DefaultLogger)

	// 5. 同步一次 qua 词库
	syncCtx := data.WithAuthInfo(ctx, &data.AuthInfo{
		TenantID: testTenantID, ID: "u-prod-1", AccessToken: testToken,
		RefreshToken: "", UserID: "u-prod-1", UserType: 2,
	})
	if _, err := quaSource.Fetch(syncCtx); err != nil {
		fmt.Printf("⚠ qua fetch partial error: %v\n", err)
	}
	if err := syncer.SyncTenant(syncCtx, testTenantID); err != nil {
		fmt.Printf("⚠ sync error: %v\n", err)
	}
	snap := vb.Build(syncCtx, testTenantID)
	fmt.Printf("✓ vocab snapshot: %d entries, %d relations\n", snap.EntryCount(), snap.RelationCount())

	// 6. ASR + enhancement
	asrReg, err := data.NewASRRegistry(conf.Asr, log.DefaultLogger)
	if err != nil {
		return mr, err
	}
	asrProviders := data.NewASRProviders(asrReg, conf.Asr)
	policy := biz.NewPolicyFromConf(conf.Enhancement)
	procReg := builtins.NewDefaultRegistry()
	pipeline, err := textenhance.BuildPipeline(procReg, policy)
	if err != nil {
		return mr, err
	}
	enhancerUC := biz.NewEnhancementUsecase(pipeline, vb, policy)
	asrUC := biz.NewASRUsecase(asrProviders, enhancerUC, conf.Asr, log.DefaultLogger)
	_ = service.NewASRService(asrUC)

	// 7. HTTP server
	mws := []middleware.Middleware{server.NewTokenAuthMiddleware(tc, nil)}
	khttpSrv := khttp.NewServer(
		khttp.Middleware(mws...),
		khttp.Timeout(120*time.Second),
		khttp.Address(listenAddr),
	)
	v1.RegisterASRServiceHTTPServer(khttpSrv, service.NewASRService(asrUC))

	checker := data.NewHealthChecker(rdb, quaClient, asrReg)
	pkgHealth.RegisterHTTP(khttpSrv, checker, 3*time.Second)

	// 启动
	fmt.Printf("\n🟢 starting Kratos HTTP server at %s\n", listenAddr)
	go func() {
		if err := khttpSrv.Start(ctx); err != nil {
			fmt.Printf("khttp start err: %v\n", err)
			cancel()
		}
	}()

	// 等服务起来
	time.Sleep(2 * time.Second)

	fmt.Println("\nTry:")
	fmt.Printf("  curl -sS http://%s/health/live\n", listenAddr)
	fmt.Printf("  curl -sS http://%s/health/ready\n", listenAddr)
	fmt.Printf("  curl -sS -X POST http://%s/evie/tool/v1/asr:recognize \\\n", listenAddr)
	fmt.Printf("    -H 'Authorization: Bearer %s' \\\n", testToken)
	fmt.Printf("    -H 'Content-Type: application/json' \\\n")
	fmt.Printf("    -d '{\"format\":{\"encoding\":\"mp3\",\"sampleRate\":16000,\"language\":\"zh\"},\"audioData\":\"<base64>\",\"language\":\"zh\",\"providerName\":\"funasr\",\"enableEnhancement\":true}'\n")

	// 健康检查 ping
	resp, err := http.Get("http://" + listenAddr + "/health/live")
	if err == nil {
		fmt.Printf("\nhealth/live → %s\n", resp.Status)
		resp.Body.Close()
	}
	resp, err = http.Get("http://" + listenAddr + "/health/ready")
	if err == nil {
		fmt.Printf("health/ready → %s\n", resp.Status)
		resp.Body.Close()
	}

	return mr, nil
}

func defaultRuleSet() *biz.RuleSet {
	quaRules := &biz.SourceRules{
		Source: "qua",
		EntityMappings: []biz.EntityMapping{
			{
				Match: biz.MatchCondition{EntityType: "user"},
				Emit: biz.EmitSpec{
					StandardText: "name", Category: "PERSON", Priority: "50",
					IncludeWhen: "status==1",
				},
			},
			{
				Match: biz.MatchCondition{EntityType: "department"},
				Emit: biz.EmitSpec{
					StandardText: "name", Category: "ORGANIZATION", Priority: "30",
				},
			},
		},
	}
	return &biz.RuleSet{Sources: map[string]*biz.SourceRules{"qua": quaRules}}
}
