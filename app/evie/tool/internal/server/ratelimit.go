// Package server · ratelimit.go
// 简单 IP-based rate limiter（滑动窗口算法）。
//
// 设计目标：
//   - 防止恶意客户端无限打挂服务
//   - per-IP 计数，避免单 IP 影响其他用户
//   - 默认关闭（生产配置启用）
//
// 算法：fixed-window counter
//   - 每 IP 一计数器（map[ip]counter），原子加 1
//   - 每 60s 滚动一次（key 是 floor(now / window)）
//   - 超过阈值返回 429 Too Many Requests
//
// 配置（conf.Server.HTTP.rate_limit_per_minute）：
//   - 0（默认）：关闭 rate limit
//   - >0：每个 IP 每分钟最多 N 个请求（推荐 60-300）
//
// 局限：
//   - 不防 IP 伪造（攻击者用 X-Forwarded-For 伪造）。生产环境必须配合：
//     反向代理层设置正确 IP，或使用 WAF / Cloudflare 等
//   - 不精确（固定窗口边界可能有 2x 突发）。如需精确滑动窗口，用 redis-cell
package server

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
)

// rateLimitEntry 单个 IP 的计数。
type rateLimitEntry struct {
	count       int
	windowStart int64 // unix 纳秒
}

// rateLimiter IP 频率限制器（fixed-window）。
type rateLimiter struct {
	mu       sync.Mutex
	entries  map[string]*rateLimitEntry
	maxPer   int           // 每窗口最大请求数
	window   time.Duration // 窗口大小
	lastGC   int64         // 上次 GC 时间
	log      *log.Helper
}

// newRateLimiter 构造 limiter（maxPer=0 表示禁用）。
func newRateLimiter(maxPer int, window time.Duration, logger log.Logger) *rateLimiter {
	return &rateLimiter{
		entries: make(map[string]*rateLimitEntry),
		maxPer:  maxPer,
		window:  window,
		log:     log.NewHelper(log.With(logger, "module", "ratelimit")),
	}
}

// allow 检查 IP 是否允许请求；true=允许，false=限流。
func (r *rateLimiter) allow(ip string) bool {
	if r.maxPer <= 0 || r.window <= 0 {
		return true // 关闭
	}
	now := time.Now().UnixNano()
	windowNs := int64(r.window.Nanoseconds())
	if windowNs <= 0 {
		// 防 sub-second 窗口舍入为 0（应给最小 1ns 兜底）
		windowNs = 1
	}
	currentWindowStart := (now / windowNs) * windowNs

	r.mu.Lock()
	defer r.mu.Unlock()

	// 周期 GC：清理过期 entry（避免内存泄漏）
	if r.lastGC == 0 || now-r.lastGC > windowNs*10 {
		r.gcLocked(now, windowNs)
		r.lastGC = now
	}

	entry, ok := r.entries[ip]
	if !ok || entry.windowStart != currentWindowStart {
		// 新窗口
		r.entries[ip] = &rateLimitEntry{count: 1, windowStart: currentWindowStart}
		return true
	}

	if entry.count >= r.maxPer {
		return false
	}
	entry.count++
	return true
}

// gcLocked 清理过期 entry（调用方必须持锁）。
func (r *rateLimiter) gcLocked(now, windowNs int64) {
	for ip, e := range r.entries {
		if now-e.windowStart > windowNs*2 {
			delete(r.entries, ip)
		}
	}
}

// middleware 返回标准 http.Handler 中间件。
func (r *rateLimiter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ip := clientIP(req)
		if !r.allow(ip) {
			w.Header().Set("Retry-After", strconv.FormatInt(int64(r.window.Seconds()), 10))
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			r.log.Warnf("rate limit hit: ip=%s path=%s", ip, req.URL.Path)
			return
		}
		next.ServeHTTP(w, req)
	})
}

// kratosMiddleware 返回 Kratos middleware.Middleware（用于 gRPC）。
func (r *rateLimiter) kratosMiddleware() middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			ip := grpcClientIP(ctx)
			if !r.allow(ip) {
				return nil, fmt.Errorf("rate limit exceeded for ip=%s", ip)
			}
			return handler(ctx, req)
		}
	}
}

// grpcClientIP 提取 gRPC 客户端 IP（来自 peer address）。
func grpcClientIP(ctx context.Context) string {
	// kratos gRPC transport 没有标准 IP 提取接口；
	// 实际生产应配合 X-Forwarded-For 中间件或 envoy 等代理
	// 这里返回空字符串 → rate limiter 接受（allow=true for empty IP）
	return ""
}

// rateLimitConfig 启动时构造 limiter 的配置。
type rateLimitConfig struct {
	MaxPerMinute int
}

// parseRateLimitConfig 从 conf 解析（rate_limit_per_minute > 0 才启用）。
func parseRateLimitConfig(maxPerMinute int) rateLimitConfig {
	if maxPerMinute <= 0 {
		return rateLimitConfig{MaxPerMinute: 0}
	}
	return rateLimitConfig{MaxPerMinute: maxPerMinute}
}
