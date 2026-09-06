package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

// newBareHTTPServer 构造一个最小 Kratos HTTP server，仅用于 pprof 挂载测试。
func newBareHTTPServer(t *testing.T) *khttp.Server {
	t.Helper()
	return khttp.NewServer()
}

func TestMountPProf_OffByDefault(t *testing.T) {
	t.Setenv("EVIE_TOOL_PPROF", "")

	srv := newBareHTTPServer(t)
	mounted := MountPProf(srv)
	if mounted {
		t.Fatal("pprof should be off by default")
	}

	ts := httptest.NewUnstartedServer(srv.Server.Handler)
	ts.Start()
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/debug/pprof/")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status=%d, want 404 when pprof off", resp.StatusCode)
	}
}

func TestMountPProf_OnWithEnv(t *testing.T) {
	t.Setenv("EVIE_TOOL_PPROF", "1")

	srv := newBareHTTPServer(t)
	mounted := MountPProf(srv)
	if !mounted {
		t.Fatal("pprof should be mounted when env=1")
	}

	ts := httptest.NewUnstartedServer(srv.Server.Handler)
	ts.Start()
	defer ts.Close()

	// 索引页
	resp, err := ts.Client().Get(ts.URL + "/debug/pprof/")
	if err != nil {
		t.Fatalf("get index: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("index status=%d, want 200", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !contains(body, "Types of profiles available") {
		t.Errorf("index missing profile list, head=%q", head(body, 120))
	}

	// heap endpoint
	resp2, err := ts.Client().Get(ts.URL + "/debug/pprof/heap")
	if err != nil {
		t.Fatalf("get heap: %v", err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("heap status=%d, want 200", resp2.StatusCode)
	}
	ct := resp2.Header.Get("Content-Type")
	if !hasPrefix(ct, "application/octet-stream") {
		t.Errorf("heap Content-Type=%q", ct)
	}

	// cmdline
	resp3, err := ts.Client().Get(ts.URL + "/debug/pprof/cmdline")
	if err != nil {
		t.Fatalf("get cmdline: %v", err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("cmdline status=%d, want 200", resp3.StatusCode)
	}
}

func TestEnvPProf_Const(t *testing.T) {
	if EnvPProf != "EVIE_TOOL_PPROF" {
		t.Errorf("EnvPProf=%q", EnvPProf)
	}
}

// 兼容 stdlib go1.21 以下没有 strings.Contains/HasPrefix 的小工具
func contains(b []byte, s string) bool {
	for i := 0; i+len(s) <= len(b); i++ {
		if string(b[i:i+len(s)]) == s {
			return true
		}
	}
	return false
}
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
func head(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n])
}
