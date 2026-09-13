// Package server · ratelimit_test.go
// IP-based rate limiter 单元测试。
package server

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

func newTestRateLimiter(maxPer int) *rateLimiter {
	logger := log.With(log.DefaultLogger, "ts", log.DefaultTimestamp, "module", "test")
	return newRateLimiter(maxPer, 100*time.Millisecond, logger)
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := newTestRateLimiter(3) // 100ms 窗口，最多 3 次

	// 同一 IP 连续 4 次，第 4 次应被限流
	for i := 0; i < 3; i++ {
		if !rl.allow("1.2.3.4") {
			t.Errorf("req %d: expected allow", i+1)
		}
	}
	if rl.allow("1.2.3.4") {
		t.Errorf("req 4: expected block (over limit)")
	}
}

func TestRateLimiter_DifferentIPs(t *testing.T) {
	rl := newTestRateLimiter(2)

	// IP A 满后，IP B 仍可访问
	for i := 0; i < 2; i++ {
		rl.allow("1.2.3.4")
	}
	if rl.allow("1.2.3.4") {
		t.Errorf("IP A 3rd req should be blocked")
	}
	if !rl.allow("5.6.7.8") {
		t.Errorf("IP B first req should be allowed")
	}
}

func TestRateLimiter_Disabled(t *testing.T) {
	rl := newTestRateLimiter(0) // disabled

	// maxPer=0 时所有请求都允许
	for i := 0; i < 100; i++ {
		if !rl.allow("1.2.3.4") {
			t.Errorf("req %d: expected allow (disabled)", i+1)
		}
	}
}

func TestRateLimiter_WindowReset(t *testing.T) {
	rl := newTestRateLimiter(2)

	// 消耗窗口
	rl.allow("1.2.3.4")
	rl.allow("1.2.3.4")
	if rl.allow("1.2.3.4") {
		t.Fatalf("3rd req should be blocked initially")
	}

	// 等待 100ms（窗口大小）+ 一点 buffer
	time.Sleep(150 * time.Millisecond)

	// 新窗口：应允许
	if !rl.allow("1.2.3.4") {
		t.Errorf("after window reset, req should be allowed")
	}
}

func TestRateLimiter_Middleware(t *testing.T) {
	rl := newTestRateLimiter(2)
	mw := rl.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		remoteAddr string
		wantStatus int
	}{
		{"first req", "1.2.3.4:1234", http.StatusOK},
		{"second req", "1.2.3.4:1234", http.StatusOK},
		{"third req blocked", "1.2.3.4:1234", http.StatusTooManyRequests},
		{"different IP allowed", "5.6.7.8:5678", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			req.RemoteAddr = tt.remoteAddr
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (body=%s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestRateLimiter_RetryAfterHeader(t *testing.T) {
	rl := newTestRateLimiter(1)
	mw := rl.middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// 触发限流
	rl.allow("1.2.3.4")

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429", rec.Code)
	}

	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Errorf("Retry-After header should be set")
	}
	// 100ms 窗口 → 0 或 1（strconv 后）
	if v, err := strconv.Atoi(retryAfter); err != nil || v < 0 {
		t.Errorf("Retry-After = %q, want non-negative int", retryAfter)
	}
}

func TestRateLimiter_ParseRateLimitConfig(t *testing.T) {
	tests := []struct {
		input    int
		wantMax  int
		isActive bool
	}{
		{0, 0, false},
		{-1, 0, false},
		{60, 60, true},
		{300, 300, true},
	}
	for _, tt := range tests {
		cfg := parseRateLimitConfig(tt.input)
		if cfg.MaxPerMinute != tt.wantMax {
			t.Errorf("parseRateLimitConfig(%d) = %d, want %d", tt.input, cfg.MaxPerMinute, tt.wantMax)
		}
	}
}
