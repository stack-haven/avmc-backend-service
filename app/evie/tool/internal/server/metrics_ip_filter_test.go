// Package server · metrics_ip_filter_test.go
// /metrics IP 白名单单元测试。
package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewMetricsIPFilter(t *testing.T) {
	tests := []struct {
		name        string
		allowedIPs  string
		checkIP     string
		wantAllowed bool
	}{
		// 空配置 = 允许所有
		{"empty allows all", "", "1.2.3.4", true},
		{"empty allows localhost", "", "127.0.0.1", true},

		// 精确 IP
		{"exact match", "127.0.0.1", "127.0.0.1", true},
		{"exact not match", "127.0.0.1", "127.0.0.2", false},

		// CIDR
		{"CIDR /32 match", "10.0.0.0/32", "10.0.0.0", true},
		{"CIDR /24 match", "10.0.0.0/24", "10.0.0.42", true},
		{"CIDR /24 no match", "10.0.0.0/24", "10.0.1.0", false},
		{"CIDR /8 wide match", "10.0.0.0/8", "10.255.255.254", true},

		// 多 IP 混合格式
		{"mixed: exact + cidr", "127.0.0.1,10.0.0.0/8", "10.5.5.5", true},
		{"mixed: second hits", "127.0.0.1,10.0.0.0/8", "127.0.0.2", false},

		// 无效 IP 处理
		{"invalid IP rejected", "not-an-ip", "1.2.3.4", false},

		// IPv6
		{"ipv6 exact", "::1", "::1", true},
		{"ipv6 not match", "::1", "::2", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newMetricsIPFilter(tt.allowedIPs)
			if got := f.allow(tt.checkIP); got != tt.wantAllowed {
				t.Errorf("allow(%q) with allowed=%q = %v, want %v",
					tt.checkIP, tt.allowedIPs, got, tt.wantAllowed)
			}
		})
	}
}

func TestMetricsIPFilterMiddleware(t *testing.T) {
	// 仅 127.0.0.1 允许
	f := newMetricsIPFilter("127.0.0.1")
	mw := f.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("metrics"))
	}))

	tests := []struct {
		name       string
		remoteAddr string
		wantStatus int
	}{
		{"localhost allowed", "127.0.0.1:12345", http.StatusOK},
		{"other denied", "10.0.0.1:12345", http.StatusForbidden},
		{"ipv6-mapped v4 denied", "[::ffff:10.0.0.1]:12345", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			req.RemoteAddr = tt.remoteAddr
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
		})
	}
}

func TestClientIP_XForwardedFor(t *testing.T) {
	tests := []struct {
		name       string
		xff        string
		remoteAddr string
		wantIP     string
	}{
		{"XFF single", "203.0.113.1", "127.0.0.1:1234", "203.0.113.1"},
		{"XFF multiple, first is client", "203.0.113.1,10.0.0.1,10.0.0.2", "127.0.0.1:1234", "203.0.113.1"},
		{"no XFF, use RemoteAddr", "", "127.0.0.1:1234", "127.0.0.1"},
		{"RemoteAddr without port", "", "127.0.0.1", "127.0.0.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
			if tt.xff != "" {
				req.Header.Set("X-Forwarded-For", tt.xff)
			}
			req.RemoteAddr = tt.remoteAddr
			if got := clientIP(req); got != tt.wantIP {
				t.Errorf("clientIP = %q, want %q", got, tt.wantIP)
			}
		})
	}
}
