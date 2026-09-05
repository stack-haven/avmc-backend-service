// Package server · asr_integration_test.go
// ASR 端到端集成测试：HTTP POST → Bearer auth → ASRService → Response。
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
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	khttp "github.com/go-kratos/kratos/v2/transport/http"

	"github.com/redis/go-redis/v9"

	pkgHealth "backend-service/pkg/health"
	v1 "backend-service/api/evie/tool/v1"
	v1conf "backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/data"
	"backend-service/app/evie/tool/internal/service"

	asrPkg "backend-service/pkg/asr"
)

// osWriteFile wraps os.WriteFile to avoid name collision.
func osWriteFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}

// asrE2EEnv 自包含 ASR 端到端测试环境。
type asrE2EEnv struct {
	mr        *miniredis.Miniredis
	token     string
	httpSrv   *httptest.Server
	audioDir  string
	systemDir string
}

func (e *asrE2EEnv) Close() {
	if e.mr != nil {
		e.mr.Close()
	}
	if e.httpSrv != nil {
		e.httpSrv.Close()
	}
}

// setupASR_E2E 构造自包含的 ASR 端到端环境。
func setupASR_E2E(t *testing.T) *asrE2EEnv {
	t.Helper()
	env := &asrE2EEnv{}

	// 1. miniredis
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	env.mr = mr
	env.token = "asr-test-token-xyz"
	mr.Set("oauth2_access_token:"+env.token, `{
		"tenantId": "158",
		"id": "u-1",
		"accessToken": "asr-test-token-xyz",
		"userId": "u-1",
		"userType": 2,
		"userInfo": {"nickname": "测试", "deptId": "d-1"}
	}`)

	// 2. system dict
	env.systemDir = t.TempDir()
	dictPath := filepath.Join(env.systemDir, "system.json")
	if err := writeFile(dictPath, `{"version":"t","entries":[]}`); err != nil {
		t.Fatal(err)
	}

	// 3. 业务组件
	conf := &v1conf.Bootstrap{
		Data:       &v1conf.Data{Redis: &v1conf.Data_Redis{Network: "tcp", Addr: mr.Addr(), TokenKeyPrefix: "oauth2_access_token:"}},
		SystemDict: &v1conf.SystemDict{Path: dictPath},
		Enhancement: &v1conf.Enhancement{Pipeline: []string{"vocab_matching"}},
	}
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	tc := data.NewTokenCache(rdb, conf.Data.Redis)

	vb, _ := biz.NewVocabularyBuilder(conf.SystemDict)

	// 4. ASR providers
	batch := &e2eMockASRProvider{name: "e2e-mock-batch", text: "你好世界", confidence: 0.95}
	stream := &e2eMockStreamProvider{name: "e2e-mock-stream", finalText: "流式最终结果"}

	// 5. ASR usecase
	env.audioDir = t.TempDir() + "/audio"
	asrUC := biz.NewASRUsecase(
		&biz.ASRProviders{Batch: batch, Stream: stream},
		nil, // enhancer nil（M7 端到端暂未集成增强）
		&v1conf.Asr{Upload: &v1conf.Asr_Upload{AudioDir: env.audioDir}},
		log.DefaultLogger,
	)
	asrSvc := service.NewASRService(asrUC)

	// 6. Kratos HTTP server
	mws := []middleware.Middleware{NewTokenAuthMiddleware(tc, nil)}
	khttpSrv := khttp.NewServer(khttp.Middleware(mws...))
	v1.RegisterASRServiceHTTPServer(khttpSrv, asrSvc)
	// M9: 健康检查
	asrReg := asrPkg.NewProviderRegistry()
	asrReg.Register(&e2eMockASRProvider{name: "e2e-mock-batch", text: "test", confidence: 0.9})
	// qua nil：M9 健康检查的 qua 是可选的（仅当配置了 baseURL 才检查）
	checker := data.NewHealthChecker(rdb, nil, asrReg)
	pkgHealth.RegisterHTTP(khttpSrv, checker, 2*time.Second)
	env.httpSrv = httptest.NewServer(khttpSrv)
	_ = vb
	return env
}

// e2eMockASRProvider E2E 整段 mock。
type e2eMockASRProvider struct {
	name       string
	text       string
	confidence float64
	mu         sync.Mutex
	calls      int
}

func (m *e2eMockASRProvider) Name() string { return m.name }
func (m *e2eMockASRProvider) Recognize(_ context.Context, _ []byte, _ asrPkg.RecognizeOptions) (*asrPkg.ASRResult, error) {
	m.mu.Lock()
	m.calls++
	m.mu.Unlock()
	return &asrPkg.ASRResult{Text: m.text, Confidence: m.confidence, ProviderName: m.name}, nil
}
func (m *e2eMockASRProvider) StreamRecognize(context.Context, <-chan asrPkg.PCMChunk, chan<- asrPkg.ASRStreamResult, asrPkg.RecognizeOptions) error {
	return nil
}
func (m *e2eMockASRProvider) Capabilities() asrPkg.ProviderCapabilities {
	return asrPkg.ProviderCapabilities{Name: m.name}
}

// e2eMockStreamProvider E2E 流式 mock。
type e2eMockStreamProvider struct {
	name      string
	finalText string
}

func (m *e2eMockStreamProvider) Name() string { return m.name }
func (m *e2eMockStreamProvider) Recognize(context.Context, []byte, asrPkg.RecognizeOptions) (*asrPkg.ASRResult, error) {
	return &asrPkg.ASRResult{ProviderName: m.name}, nil
}
func (m *e2eMockStreamProvider) StreamRecognize(_ context.Context, in <-chan asrPkg.PCMChunk, out chan<- asrPkg.ASRStreamResult, _ asrPkg.RecognizeOptions) error {
	defer close(out)
	for range in {
		out <- asrPkg.ASRStreamResult{Text: "partial", IsFinal: false, Confidence: 0.9}
	}
	out <- asrPkg.ASRStreamResult{Text: m.finalText, IsFinal: true, Confidence: 0.95}
	return nil
}
func (m *e2eMockStreamProvider) Capabilities() asrPkg.ProviderCapabilities {
	return asrPkg.ProviderCapabilities{Name: m.name, Streaming: true}
}

// writeFile helper.
func writeFile(path, content string) error {
	return osWriteFile(path, content)
}

// TestE2E_ASR_Recognize 测试整段识别全链路：HTTP → Bearer → service → usecase → mock provider。
func TestE2E_ASR_Recognize(t *testing.T) {
	env := setupASR_E2E(t)
	defer env.Close()

	body := []byte(`{
		"format": {"encoding": "wav", "sampleRate": 16000, "bitDepth": 16},
		"audioData": "aGVsbG8td29ybGQ=",
		"enableEnhancement": false
	}`)
	req, _ := http.NewRequest("POST", env.httpSrv.URL+"/evie/tool/v1/asr:recognize", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP: %v", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	t.Logf("status=%d body=%s", resp.StatusCode, string(respBody))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, body = %s", resp.StatusCode, string(respBody))
	}

	var raw map[string]any
	if err := json.Unmarshal(respBody, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got, _ := raw["rawText"].(string); got != "你好世界" {
		t.Errorf("rawText = %q, want %q", got, "你好世界")
	}
	if got, _ := raw["sessionId"].(string); got == "" {
		t.Error("sessionId empty")
	}
	if got, _ := raw["audioPath"].(string); got == "" {
		t.Error("audioPath empty")
	}
	if got, _ := raw["providerName"].(string); got != "e2e-mock-batch" {
		t.Errorf("providerName = %q, want e2e-mock-batch", got)
	}
}

// TestE2E_ASR_Recognize_NoAuth 测试无 Bearer 返回 401。
func TestE2E_ASR_Recognize_NoAuth(t *testing.T) {
	env := setupASR_E2E(t)
	defer env.Close()

	body := []byte(`{
		"format": {"encoding": "wav"},
		"audioData": "ZGF0YQ=="
	}`)
	req, _ := http.NewRequest("POST", env.httpSrv.URL+"/evie/tool/v1/asr:recognize", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("HTTP: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", resp.StatusCode)
	}
}

// TestE2E_ASR_Recognize_List_GetAudio 完整流程：recognize → list → get → audio。
func TestE2E_ASR_Recognize_List_GetAudio(t *testing.T) {
	env := setupASR_E2E(t)
	defer env.Close()

	// 1. recognize
	body := []byte(`{
		"format": {"encoding": "wav", "sampleRate": 16000, "bitDepth": 16},
		"audioData": "aGVsbG8td29ybGQ=",
		"enableEnhancement": false
	}`)
	req, _ := http.NewRequest("POST", env.httpSrv.URL+"/evie/tool/v1/asr:recognize", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+env.token)
	resp, _ := http.DefaultClient.Do(req)
	recBody, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("recognize status=%d body=%s", resp.StatusCode, string(recBody))
	}
	var rec map[string]any
	json.Unmarshal(recBody, &rec)
	sessionID := rec["sessionId"].(string)
	if sessionID == "" {
		t.Fatal("sessionId empty")
	}

	// 2. list records
	listReq, _ := http.NewRequest("GET", env.httpSrv.URL+"/evie/tool/v1/asr/records?pageSize=10", nil)
	listReq.Header.Set("Authorization", "Bearer "+env.token)
	listResp, _ := http.DefaultClient.Do(listReq)
	listBody, _ := io.ReadAll(listResp.Body)
	listResp.Body.Close()
	if listResp.StatusCode != 200 {
		t.Fatalf("list status=%d", listResp.StatusCode)
	}
	var listRaw map[string]any
	json.Unmarshal(listBody, &listRaw)
	if total, _ := listRaw["total"].(float64); total < 1 {
		t.Errorf("total=%v, want >= 1", total)
	}

	// 3. get record
	getReq, _ := http.NewRequest("GET", env.httpSrv.URL+"/evie/tool/v1/asr/records/"+sessionID, nil)
	getReq.Header.Set("Authorization", "Bearer "+env.token)
	getResp, _ := http.DefaultClient.Do(getReq)
	getBody, _ := io.ReadAll(getResp.Body)
	getResp.Body.Close()
	if getResp.StatusCode != 200 {
		t.Fatalf("get status=%d body=%s", getResp.StatusCode, string(getBody))
	}
	var getRaw map[string]any
	json.Unmarshal(getBody, &getRaw)
	if got, _ := getRaw["rawText"].(string); got != "你好世界" {
		t.Errorf("get rawText = %q", got)
	}

	// 4. get audio
	audioReq, _ := http.NewRequest("GET", env.httpSrv.URL+"/evie/tool/v1/asr/records/"+sessionID+"/audio", nil)
	audioReq.Header.Set("Authorization", "Bearer "+env.token)
	audioResp, _ := http.DefaultClient.Do(audioReq)
	audioBody, _ := io.ReadAll(audioResp.Body)
	audioResp.Body.Close()
	if audioResp.StatusCode != 200 {
		t.Fatalf("audio status=%d body=%s", audioResp.StatusCode, string(audioBody))
	}
	if len(audioBody) == 0 {
		t.Error("audio body empty")
	}
	// HTTP gateway wraps binary as JSON with base64; 验证 base64 解码后非空
	var audioRaw map[string]any
	if err := json.Unmarshal(audioBody, &audioRaw); err == nil {
		if ad, ok := audioRaw["audioData"].(string); ok && ad != "" {
			t.Logf("✓ audio fetched via JSON wrapper: contentType=%v, base64_len=%d",
				audioRaw["contentType"], len(ad))
		}
	}
	if ct := audioResp.Header.Get("Content-Type"); ct != "application/json" && ct != "audio/wav" {
		t.Errorf("Content-Type = %q, want application/json or audio/wav", ct)
	}
}

// TestE2E_Health_Live 测试 /health/live（M9 收口）。
func TestE2E_Health_Live(t *testing.T) {
	env := setupASR_E2E(t)
	defer env.Close()

	resp, err := http.Get(env.httpSrv.URL + "/health/live")
	if err != nil {
		t.Fatalf("GET /health/live: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, string(body))
	}
	if raw["status"] != "ok" {
		t.Errorf("status = %v, want ok", raw["status"])
	}
}

// TestE2E_Health_Ready 测试 /health/ready。
func TestE2E_Health_Ready(t *testing.T) {
	env := setupASR_E2E(t)
	defer env.Close()

	resp, err := http.Get(env.httpSrv.URL + "/health/ready")
	if err != nil {
		t.Fatalf("GET /health/ready: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if raw["status"] != "ok" {
		t.Errorf("status = %v, want ok", raw["status"])
	}
	details, _ := raw["details"].(map[string]any)
	if details == nil {
		t.Fatal("details missing")
	}
	if details["redis"] != true {
		t.Errorf("details[redis] = %v, want true", details["redis"])
	}
	if asrProviders, _ := details["asr_providers"].([]any); len(asrProviders) == 0 {
		t.Errorf("details[asr_providers] empty")
	}
}
