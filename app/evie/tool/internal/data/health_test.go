// Package data · health_test.go
// 健康检查器单测（M9）。
package data

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"

	v1conf "backend-service/app/evie/tool/internal/conf"
	asrPkg "backend-service/pkg/asr"
)

// mockProvider ASR provider mock（实现 asrPkg.ASRProvider）。
type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Recognize(_ context.Context, _ []byte, _ asrPkg.RecognizeOptions) (*asrPkg.ASRResult, error) {
	return &asrPkg.ASRResult{ProviderName: m.name}, nil
}
func (m *mockProvider) StreamRecognize(context.Context, <-chan asrPkg.PCMChunk, chan<- asrPkg.ASRStreamResult, asrPkg.RecognizeOptions) error {
	return nil
}
func (m *mockProvider) Capabilities() asrPkg.ProviderCapabilities {
	return asrPkg.ProviderCapabilities{Name: m.name}
}

func newTestRegistry() *asrPkg.ProviderRegistry {
	r := asrPkg.NewProviderRegistry()
	r.Register(&mockProvider{name: "test-batch"})
	return r
}

// TestHealthChecker_Ready_AllOK 全依赖正常。
func TestHealthChecker_Ready_AllOK(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// mock qua server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	qua, _ := NewQuaClient(&v1conf.Qua{BaseUrl: ts.URL}, log.DefaultLogger)
	reg := newTestRegistry()

	c := NewHealthChecker(rdb, qua, reg).(*HealthChecker)
	if err := c.Ready(context.Background()); err != nil {
		t.Errorf("Ready: %v", err)
	}
}

// TestHealthChecker_Ready_RedisDown Redis 不可达时返回错误。
func TestHealthChecker_Ready_RedisDown(t *testing.T) {
	// 启动后立即关闭 → 连接失败
	mr := miniredis.RunT(t)
	addr := mr.Addr()
	mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: addr})

	c := NewHealthChecker(rdb, nil, nil).(*HealthChecker)
	err := c.Ready(context.Background())
	if err == nil {
		t.Error("expected redis error")
	}
}

// TestHealthChecker_Ready_NilComponents nil 情况下应该不 panic。
func TestHealthChecker_Ready_NilComponents(t *testing.T) {
	var c *HealthChecker
	if err := c.Ready(context.Background()); err == nil || err.Error() != "health: nil checker" {
		t.Errorf("nil checker should error, got %v", err)
	}

	c = NewHealthChecker(nil, nil, nil).(*HealthChecker)
	if err := c.Ready(context.Background()); err == nil {
		t.Error("nil rdb should error")
	}
}

// TestHealthChecker_Details 验证 Details 输出结构。
func TestHealthChecker_Details(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	reg := newTestRegistry()

	c := NewHealthChecker(rdb, nil, reg).(*HealthChecker)
	c.SetSyncState(time.Now(), "")
	details := c.Details(context.Background())
	if details["redis"] != true {
		t.Errorf("details[redis] = %v", details["redis"])
	}
	if _, ok := details["asr_providers"]; !ok {
		t.Error("details[asr_providers] missing")
	}
	if _, ok := details["vocab_last_sync"]; !ok {
		t.Error("details[vocab_last_sync] missing")
	}
}

// TestHealthChecker_Ready_ASRMissing ASR reg 含 nil entry 时不 panic。
func TestHealthChecker_Ready_ASRMissing(t *testing.T) {
	mr := miniredis.RunT(t)
	defer mr.Close()
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// ASR reg 有有效 provider + nil reg 的混合场景——构造一个空 reg
	reg := asrPkg.NewProviderRegistry()
	c := NewHealthChecker(rdb, nil, reg).(*HealthChecker)
	err := c.Ready(context.Background())
	// redis OK + qua nil + asr 空 → 应通过
	if err != nil {
		t.Errorf("Ready with valid redis + empty asr: %v", err)
	}
}