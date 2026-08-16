package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"backend-service/pkg/audit"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
	"google.golang.org/grpc/metadata"
)

type testTransport struct {
	khttp.Transporter
	kind      transport.Kind
	operation string
	req       *http.Request
}

func (t *testTransport) Kind() transport.Kind   { return t.kind }
func (t *testTransport) Operation() string      { return t.operation }
func (t *testTransport) Request() *http.Request { return t.req }

func testExtractor(ctx context.Context) audit.UserInfo {
	return audit.UserInfo{TenantID: 100, UserID: 1, UserName: "testuser"}
}

func waitForLogs(c *audit.MemoryClient, want int) {
	for i := 0; i < 20 && c.Count() < want; i++ {
		time.Sleep(5 * time.Millisecond)
	}
}

func TestAuditMiddlewareSkipsGet(t *testing.T) {
	client := audit.NewMemoryClient()
	mw := Server(client, testExtractor, log.DefaultLogger)
	handler := mw(func(_ context.Context, _ interface{}) (interface{}, error) { return "ok", nil })

	tr := &testTransport{kind: transport.KindHTTP, operation: "/evie/v1/users", req: httptest.NewRequest("GET", "/evie/v1/users", nil)}
	ctx := transport.NewServerContext(context.Background(), tr)

	_, err := handler(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if client.Count() != 0 {
		t.Fatalf("expected 0 logs for GET, got %d", client.Count())
	}
}

func TestAuditMiddlewareRecordsPost(t *testing.T) {
	client := audit.NewMemoryClient()
	mw := Server(client, testExtractor, log.DefaultLogger)
	handler := mw(func(_ context.Context, _ interface{}) (interface{}, error) { return "ok", nil })

	tr := &testTransport{kind: transport.KindHTTP, operation: "/evie/v1/users", req: httptest.NewRequest("POST", "/evie/v1/users", nil)}
	ctx := transport.NewServerContext(context.Background(), tr)

	_, err := handler(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	waitForLogs(client, 1)
	if client.Count() != 1 {
		t.Fatalf("expected 1 log for POST, got %d", client.Count())
	}
	r := client.Records[0]
	if r.Method != "POST" {
		t.Fatalf("expected method POST, got %s", r.Method)
	}
	if !r.Success {
		t.Fatal("expected success=true")
	}
	if r.TenantID != 100 || r.OperatorID != 1 || r.OperatorName != "testuser" {
		t.Fatalf("unexpected user info: %+v", r)
	}
}

func TestAuditMiddlewareRecordsError(t *testing.T) {
	client := audit.NewMemoryClient()
	mw := Server(client, testExtractor, log.DefaultLogger)
	handler := mw(func(_ context.Context, _ interface{}) (interface{}, error) { return nil, context.Canceled })

	tr := &testTransport{kind: transport.KindHTTP, operation: "/evie/v1/users", req: httptest.NewRequest("DELETE", "/evie/v1/users/1", nil)}
	ctx := transport.NewServerContext(context.Background(), tr)

	handler(ctx, nil)
	waitForLogs(client, 1)
	if client.Count() != 1 {
		t.Fatalf("expected 1 log for DELETE, got %d", client.Count())
	}
	r := client.Records[0]
	if r.Success {
		t.Fatal("expected success=false for error response")
	}
	if r.ErrorMessage == "" {
		t.Fatal("expected non-empty error_message")
	}
}

type captureClient struct {
	mu     sync.Mutex
	gotCtx context.Context
}

func (c *captureClient) Append(ctx context.Context, _ *audit.Record) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.gotCtx = ctx
	return nil
}

// TestAuditMiddlewareForwardsAuthHeader 验证异步审计 goroutine 丢失 server context 后，
// 中间件仍能在同步阶段提取 Authorization 头并注入出站 metadata，供 platform 跨服务认证。
func TestAuditMiddlewareForwardsAuthHeader(t *testing.T) {
	client := &captureClient{}
	mw := Server(client, testExtractor, log.DefaultLogger)
	handler := mw(func(_ context.Context, _ interface{}) (interface{}, error) { return "ok", nil })

	req := httptest.NewRequest("POST", "/evie/v1/users", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	tr := &testTransport{kind: transport.KindHTTP, operation: "/evie/v1/users", req: req}
	ctx := transport.NewServerContext(context.Background(), tr)

	if _, err := handler(ctx, nil); err != nil {
		t.Fatal(err)
	}

	// 等待异步 goroutine 写入 ctx
	var gotCtx context.Context
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		client.mu.Lock()
		gotCtx = client.gotCtx
		client.mu.Unlock()
		if gotCtx != nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if gotCtx == nil {
		t.Fatal("client.Append not called within deadline")
	}

	md, ok := metadata.FromOutgoingContext(gotCtx)
	if !ok {
		t.Fatal("expected outgoing metadata carrying authorization")
	}
	got := md.Get("authorization")
	if len(got) == 0 || got[0] != "Bearer test-token" {
		t.Fatalf("authorization = %v, want [Bearer test-token]", got)
	}
}
