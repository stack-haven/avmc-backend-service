// Package server · middleware_integration_test.go
// 端到端集成测试：
//   miniredis（mock Redis）+ httptest qua + Kratos HTTP server + EnhancementService
//   全链路：HTTP POST → Bearer middleware → AuthInfo → usecase → Pipeline → Response
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"backend-service/pkg/textenhance"
	"backend-service/pkg/textenhance/builtins"

	v1 "backend-service/api/evie/tool/v1"
	"github.com/redis/go-redis/v9"
	v1conf "backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/data"
	"backend-service/app/evie/tool/internal/service"
)

// endToEndEnv 集成测试环境：mock redis + mock qua + Kratos HTTP server。
type endToEndEnv struct {
	miniRedis     *miniredis.Miniredis
	quaServer     *httptest.Server
	systemDictDir string
	httpSrv       *httptest.Server
	token        string
}

func (e *endToEndEnv) Close() {
	if e.miniRedis != nil { e.miniRedis.Close() }
	if e.quaServer != nil { e.quaServer.Close() }
	if e.httpSrv != nil { e.httpSrv.Close() }
	if e.systemDictDir != "" { os.RemoveAll(e.systemDictDir) }
}

// setupEndToEnd 构造完整测试环境。
func setupEndToEnd(t *testing.T) *endToEndEnv {
	t.Helper()
	env := &endToEndEnv{}

	// 1. mock Redis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	env.miniRedis = mr

	// 2. 写入合法 token
	env.token = "test-bearer-token-abcdef"
	mr.Set("oauth2_access_token:"+env.token, `{
		"tenantId": "158",
		"id": "2094623134267203585",
		"accessToken": "test-bearer-token-abcdef",
		"refreshToken": "xxx",
		"userId": "2031552504886435841",
		"userType": 2,
		"userInfo": {"nickname": "测试账号", "deptId": "1904450235179954177"},
		"clientId": "default",
		"scopes": null,
		"expiresTime": 1788491296083
	}`)

	// 3. mock qua
	env.quaServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/admin-api/system/dept/list":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "msg": "ok",
				"data": []map[string]any{
					{"id": "1", "parentId": "0", "name": "技术研发部"},
				},
			})
		case "/admin-api/qua/member-extended/page":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0, "msg": "ok",
				"data": map[string]any{
					"list": []map[string]any{
						{"id": "2031552504886435841", "realName": "测试一号", "nickname": "一号", "deptId": "1", "status": 1},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))

	// 4. system.json
	env.systemDictDir = t.TempDir()
	systemDictPath := filepath.Join(env.systemDictDir, "system.json")
	if err := os.WriteFile(systemDictPath, []byte(`{
  "version": "test",
  "entries": [
    {"standard_text": "金种籽", "category": "PRODUCT", "aliases": ["金种子"]}
  ]
}`), 0644); err != nil {
		t.Fatalf("write dict: %v", err)
	}

	// 5. 构造 conf + 业务组件
	conf := &v1conf.Bootstrap{
		Server: &v1conf.Server{Http: &v1conf.Server_HTTP{Addr: ":0"}, Grpc: &v1conf.Server_GRPC{Addr: ":0"}},
		Data:   &v1conf.Data{Redis: &v1conf.Data_Redis{Network: "tcp", Addr: mr.Addr(), TokenKeyPrefix: "oauth2_access_token:"}},
		Qua:    &v1conf.Qua{BaseUrl: env.quaServer.URL, Endpoints: &v1conf.Qua_Endpoints{ListUsers: "/admin-api/qua/member-extended/page", ListDepts: "/admin-api/system/dept/list"}},
		Enhancement: &v1conf.Enhancement{Pipeline: []string{"vocab_matching", "alias_resolution", "deterministic_replacement"}},
		SystemDict:  &v1conf.SystemDict{Path: systemDictPath, HotReload: false},
		VocabRules:  &v1conf.VocabRules{},
	}

	// 6. 构造业务组件（直连 miniredis）
	var rdb = newRedisClient(mr.Addr()).(*redis.Client)
	tc := data.NewTokenCache(rdb, conf.Data.Redis)

	quaClient, _ := data.NewQuaClient(conf.Qua, log.DefaultLogger)
	_ = quaClient // M5 阶段会使用；这里仅验证 config + wire
	vocabBuilder, _ := biz.NewVocabularyBuilder(conf.SystemDict)
	policy := biz.NewPolicyFromConf(conf.Enhancement)
	registry := builtins.NewDefaultRegistry()
	pipeline, _ := textenhance.BuildPipeline(registry, policy)
	usecase := biz.NewEnhancementUsecase(pipeline, vocabBuilder, policy)
	enhService := service.NewEnhancementService(usecase, log.DefaultLogger)

	// 7. 启动 Kratos HTTP server（含 Bearer middleware + EnhancementService）
	skipPaths := []string{} // 所有路径都需 auth
	mws := []middleware.Middleware{recoveryMiddleware(), NewTokenAuthMiddleware(tc, skipPaths)}
	khttpSrv := khttp.NewServer(khttp.Middleware(mws...))
	v1.RegisterEnhancementServiceHTTPServer(khttpSrv, enhService)

	env.httpSrv = httptest.NewServer(khttpSrv)
	return env
}

// newRedisClientAt 保留旧名（兼容）。
func newRedisClientAt(t *testing.T, addr string) redis.UniversalClient {
	t.Helper()
	return newRedisClient(addr)
}

// TestE2E_FullChain 测试完整链路：HTTP POST → Bearer auth → Pipeline → Response。
func TestE2E_FullChain(t *testing.T) {
	env := setupEndToEnd(t)
	defer env.Close()

	// 1. 准备请求（直接构造 JSON bytes，避免 Marshal 拷贝 proto message 的锁）
	bodyBytes := []byte(`{"text":"金种子是新产品"}`)

	httpReq, _ := http.NewRequest("POST",
		env.httpSrv.URL+"/evie/tool/v1/enhance",
		bytes.NewReader(bodyBytes),
	)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+env.token)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("HTTP request: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	t.Logf("status=%d body=%s", resp.StatusCode, string(respBody))

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200, body = %s", resp.StatusCode, string(respBody))
	}

	// 2. 验证 response body（Kratos HTTP gateway 输出 camelCase，proto struct 标签是 snake_case；
	//    这里用 map 解析避免 tag 差异）
	var raw map[string]any
	if err := json.Unmarshal(respBody, &raw); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if got, _ := raw["originalText"].(string); got != "金种子是新产品" {
		t.Errorf("originalText = %q, want %q", got, "金种子是新产品")
	}
	if got, _ := raw["enhancedText"].(string); got != "金种籽是新产品" {
		t.Errorf("enhancedText = %q, want %q", got, "金种籽是新产品")
	}
	if status, _ := raw["status"].(float64); status == 0 {
		t.Errorf("status = 0, want SUCCESS (1)")
	}
	changes, _ := raw["changes"].([]any)
	if len(changes) == 0 {
		t.Error("expected changes to be non-empty")
	}
	t.Logf("✓ E2E full chain works: original=%v enhanced=%v status=%v changes=%d",
		raw["originalText"], raw["enhancedText"], raw["status"], len(changes))
}

// TestE2E_NoAuthHeader 测试无 Bearer 时返回 401。
func TestE2E_NoAuthHeader(t *testing.T) {
	env := setupEndToEnd(t)
	defer env.Close()

	reqBody := []byte(`{"text":"test"}`)
	httpReq, _ := http.NewRequest("POST",
		env.httpSrv.URL+"/evie/tool/v1/enhance",
		bytes.NewReader(reqBody),
	)
	httpReq.Header.Set("Content-Type", "application/json")
	// 故意不设 Authorization

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// TestE2E_InvalidToken 测试无效 Bearer 时返回 401。
func TestE2E_InvalidToken(t *testing.T) {
	env := setupEndToEnd(t)
	defer env.Close()

	reqBody := []byte(`{"text":"test"}`)
	httpReq, _ := http.NewRequest("POST",
		env.httpSrv.URL+"/evie/tool/v1/enhance",
		bytes.NewReader(reqBody),
	)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer invalid-token-xxxxx")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		t.Fatalf("HTTP request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401 (invalid token)", resp.StatusCode)
	}
}

// TestE2E_MultiRequest 测试多个并发请求（验证 Pipeline 不可变性 + ctx 安全）。
func TestE2E_MultiRequest(t *testing.T) {
	env := setupEndToEnd(t)
	defer env.Close()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			reqBody := []byte(`{"text":"金种子"}`)
			httpReq, _ := http.NewRequest("POST",
				env.httpSrv.URL+"/evie/tool/v1/enhance",
				bytes.NewReader(reqBody),
			)
			httpReq.Header.Set("Content-Type", "application/json")
			httpReq.Header.Set("Authorization", "Bearer "+env.token)
			resp, err := http.DefaultClient.Do(httpReq)
			if err != nil { return }
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), "金种籽") {
				t.Errorf("goroutine %d: missing 金种籽: %s", idx, string(body))
			}
		}(i)
	}
	wg.Wait()
}

// recoveryMiddleware panic recover（与 server/middleware.go 一致）。
func recoveryMiddleware() middleware.Middleware {
	return func(h middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			defer func() { _ = recover() }()
			return h(ctx, req)
		}
	}
}

// newRedisClient 构造连接到 addr 的 redis.Client（与 data 包的 NewRedisClientForTest 等价）。
func newRedisClient(addr string) redis.UniversalClient {
	return redis.NewClient(&redis.Options{
		Network:      "tcp",
		Addr:         addr,
		Password:     "",
		DB:           0,
		ReadTimeout:  500 * time.Millisecond,
		WriteTimeout: 500 * time.Millisecond,
	})
}

// 防止 time 包 unused（未来 M5/M7 加 ticker 时用）
var _ = time.Now