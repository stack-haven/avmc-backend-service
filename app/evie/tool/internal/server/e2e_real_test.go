package server_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kratos/kratos/v2/log"

	v1conf "backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/data"
	"backend-service/app/evie/tool/internal/server"
	"backend-service/app/evie/tool/internal/service"
	asrPkg "backend-service/pkg/asr"
)

// trivialChecker 在 E2E 测试中充当零依赖健康检查器。
type trivialChecker struct{}

func (trivialChecker) Ready(ctx context.Context) error { return nil }
func (trivialChecker) Details(ctx context.Context) map[string]any {
	return map[string]any{"e2e": true}
}

// mockASRProvider 在 E2E 测试中代替真实 funasr/xunfei provider。
type mockASRProvider struct {
	name string
	text string
}

func (m *mockASRProvider) Name() string { return m.name }
func (m *mockASRProvider) Recognize(_ context.Context, audio []byte, _ asrPkg.RecognizeOptions) (*asrPkg.ASRResult, error) {
	return &asrPkg.ASRResult{
		Text:         m.text,
		Confidence:   0.99,
		ProviderName: m.name,
		DurationMs:   int64(len(audio) / 32),
	}, nil
}
func (m *mockASRProvider) StreamRecognize(context.Context, <-chan asrPkg.PCMChunk, chan<- asrPkg.ASRStreamResult, asrPkg.RecognizeOptions) error {
	return nil
}
func (m *mockASRProvider) Capabilities() asrPkg.ProviderCapabilities {
	return asrPkg.ProviderCapabilities{Name: m.name}
}

// TestE2E_RealRecording 启动一个零外部依赖的 Kratos HTTP 服务，注入 mock ASR，
// 读取 testdata/晨会录音.mp3 进行端到端 HTTP 测试。
func TestE2E_RealRecording(t *testing.T) {
	// 1. 系统词条（空条目即可）
	sysDir := t.TempDir()
	sysPath := filepath.Join(sysDir, "system.json")
	if err := os.WriteFile(sysPath, []byte(`{"version":"t","entries":[]}`), 0o644); err != nil {
		t.Fatalf("write system: %v", err)
	}
	sysConf := &v1conf.SystemDict{Path: sysPath}

	// 2. 构造增强链路
	vb, err := biz.NewVocabularyBuilder(sysConf)
	if err != nil {
		t.Fatalf("vocab builder: %v", err)
	}
	engine, err := biz.NewLexnormEngine(&v1conf.Enhancement{}, vb, log.DefaultLogger)
	if err != nil {
		t.Fatalf("lexnorm engine: %v", err)
	}
	enhUC := biz.NewEnhancementUsecase(engine)
	enhSvc := service.NewEnhancementService(enhUC, log.DefaultLogger)

	// 3. 注入 mock ASR provider
	mock := &mockASRProvider{name: "mock-asr", text: "晨会录音识别结果"}
	asrUC := biz.NewASRUsecase(
		&biz.ASRProviders{Batch: mock},
		enhUC,
		&v1conf.Asr{Upload: &v1conf.Asr_Upload{AudioDir: t.TempDir() + "/audio"}},
		log.DefaultLogger,
	)
	asrSvc := service.NewASRService(asrUC)

	// 4. 静态 token 缓存（demo 模式）
	tokenCache, err := data.NewStaticTokenCache(&v1conf.StaticCredential{
		Users: []*v1conf.StaticUser{
			{Token: "test-token", TenantId: "t1", UserId: "u1"},
		},
	})
	if err != nil {
		t.Fatalf("static token: %v", err)
	}

	// 5. 健康检查（无依赖）
	checker := &trivialChecker{}

	// 6. 启动 HTTP 服务（监听随机端口）
	ksrv := server.NewHTTPServer(
		&v1conf.Server{Http: &v1conf.Server_HTTP{Addr: ":0", Timeout: nil}},
		tokenCache, enhSvc, asrSvc, checker, log.DefaultLogger,
	)
	// 把 kratos http.Server 内部的 http.Handler 暴露给 httptest。
	ts := httptest.NewUnstartedServer(ksrv.Server.Handler)
	ts.Start()
	defer ts.Close()

	// 7. 读取真实录音（mp3）
	mp3Path := filepath.Join("..", "..", "testdata", "晨会录音.mp3")
	mp3, err := os.ReadFile(mp3Path)
	if err != nil {
		t.Fatalf("read mp3 %s: %v", mp3Path, err)
	}
	t.Logf("mp3 size = %d bytes", len(mp3))
	if len(mp3) == 0 {
		t.Fatal("mp3 file is empty")
	}
	audioB64 := base64.StdEncoding.EncodeToString(mp3)

	// 8. /evie/tool/v1/asr:recognize（带 token）
	body := map[string]any{
		"format":            map[string]any{"encoding": "mp3", "sampleRate": 16000, "bitDepth": 16},
		"audioData":         audioB64,
		"enableEnhancement": false,
	}
	jsonBody, _ := json.Marshal(body)
	doRequest := func(method, path, token string, payload []byte) (*http.Response, []byte) {
		t.Helper()
		req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader(payload))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("do: %v", err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp, b
	}

	t.Run("recognize_with_token", func(t *testing.T) {
		resp, body := doRequest("POST", "/evie/tool/v1/asr:recognize", "test-token", jsonBody)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d body %s", resp.StatusCode, body)
		}
		var raw map[string]any
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if raw["rawText"] != "晨会录音识别结果" {
			t.Errorf("rawText = %v, want mock text", raw["rawText"])
		}
		if raw["sessionId"] == "" {
			t.Error("sessionId empty")
		}
		if raw["audioPath"] == "" {
			t.Error("audioPath empty")
		}
		t.Logf("recognize ok: session=%v", raw["sessionId"])
	})

	// 取 sessionId 用于后续 list / get audio
	listResp, listBody := doRequest("GET", "/evie/tool/v1/asr/records?pageSize=10", "test-token", nil)
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("list status %d body %s", listResp.StatusCode, listBody)
	}
	var listRaw map[string]any
	_ = json.Unmarshal(listBody, &listRaw)
	if total, _ := listRaw["total"].(float64); total < 1 {
		t.Errorf("list total = %v, want >=1", total)
	}
	records, _ := listRaw["records"].([]any)
	if len(records) == 0 {
		t.Fatal("list records empty")
	}
	first := records[0].(map[string]any)
	sid, _ := first["id"].(string)
	if sid == "" {
		t.Fatal("record id empty")
	}

	t.Run("get_record", func(t *testing.T) {
		resp, body := doRequest("GET", "/evie/tool/v1/asr/records/"+sid, "test-token", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d body %s", resp.StatusCode, body)
		}
		var raw map[string]any
		_ = json.Unmarshal(body, &raw)
		if raw["id"] != sid {
			t.Errorf("get id = %v, want %s", raw["id"], sid)
		}
	})

	t.Run("get_record_audio", func(t *testing.T) {
		resp, body := doRequest("GET", "/evie/tool/v1/asr/records/"+sid+"/audio", "test-token", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d body %s", resp.StatusCode, body)
		}
		// HTTP gateway 把它包成 JSON {"audioData": "..."}.
		var raw map[string]any
		if err := json.Unmarshal(body, &raw); err == nil {
			if ad, ok := raw["audioData"].(string); ok && ad != "" {
				decoded, _ := base64.StdEncoding.DecodeString(ad)
				t.Logf("audio fetched: content-type=%v, base64_len=%d, decoded_len=%d",
					raw["contentType"], len(ad), len(decoded))
			}
		}
	})

	// 9. 401：缺 token / 错 token
	t.Run("recognize_no_token", func(t *testing.T) {
		resp, _ := doRequest("POST", "/evie/tool/v1/asr:recognize", "", jsonBody)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status %d, want 401", resp.StatusCode)
		}
	})
	t.Run("recognize_bad_token", func(t *testing.T) {
		resp, _ := doRequest("POST", "/evie/tool/v1/asr:recognize", "bad", jsonBody)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status %d, want 401", resp.StatusCode)
		}
	})

	// 10. /enhance 也跑一次
	t.Run("enhance", func(t *testing.T) {
		body := []byte(`{"text":"金种子是新产品"}`)
		resp, b := doRequest("POST", "/evie/tool/v1/enhance", "test-token", body)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d body %s", resp.StatusCode, b)
		}
		var raw map[string]any
		_ = json.Unmarshal(b, &raw)
		if raw["originalText"] != "金种子是新产品" {
			t.Errorf("originalText = %v", raw["originalText"])
		}
		if raw["enhancedText"] == "" {
			t.Error("enhancedText empty")
		}
	})

	// 11. metrics 端点（不需要 token）
	t.Run("metrics", func(t *testing.T) {
		resp, body := doRequest("GET", "/metrics", "", nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d", resp.StatusCode)
		}
		if !bytes.Contains(body, []byte("evie_http_requests_total")) {
			t.Errorf("missing evie_http_requests_total in metrics output")
		}
	})

	// 12. health
	t.Run("health_live", func(t *testing.T) {
		resp, _ := doRequest("GET", "/health/live", "", nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status %d", resp.StatusCode)
		}
	})
	t.Run("health_ready", func(t *testing.T) {
		resp, _ := doRequest("GET", "/health/ready", "", nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status %d", resp.StatusCode)
		}
	})
}
