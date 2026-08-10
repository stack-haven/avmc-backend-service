package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"backend-service/pkg/audit"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

type testTransport struct {
	khttp.Transporter
	kind      transport.Kind
	operation string
	req       *http.Request
}

func (t *testTransport) Kind() transport.Kind  { return t.kind }
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
