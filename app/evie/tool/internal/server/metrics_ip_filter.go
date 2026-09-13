// Package server · metrics_ip_filter.go
// /metrics 端点的 IP 白名单 middleware（生产环境推荐）。
//
// 设计动机：
//   - /metrics 暴露 Prometheus 文本格式的内部指标
//   - 默认无鉴权，任何 IP 可读取
//   - 生产环境应限制为内网 IP 或 Prometheus 采集 IP
//
// 配置（conf.Server.HTTP.metrics_allowed_ips）：
//   - 空（默认）：任意 IP 可访问（仅 dev/test）
//   - "127.0.0.1,10.0.0.5"：白名单模式（精确匹配）
//   - "10.0.0.0/8"：CIDR 范围匹配（生产推荐）
package server

import (
	"net"
	"net/http"
	"strings"
)

// metricsIPFilter 仅允许指定 IP 访问 /metrics 端点。
//
// allowedIPs 逗号分隔，每项可以是：
//   - 精确 IP：127.0.0.1、::1
//   - CIDR 范围：10.0.0.0/8、192.168.1.0/24
//
// 匹配逻辑：解析 IP 后，依次尝试精确匹配 + CIDR 匹配。
// 解析失败的白名单条目会忽略并 warn（不阻断其他 IP 匹配）。
type metricsIPFilter struct {
	allowedExact map[string]bool // 精确 IP 集合（v4/v6 都转 string）
	allowedCIDRs []*net.IPNet
	allowAll     bool // 空配置 = 不限制
}

// newMetricsIPFilter 构造 IP 过滤器。
func newMetricsIPFilter(allowedIPs string) *metricsIPFilter {
	f := &metricsIPFilter{
		allowedExact: map[string]bool{},
	}
	if allowedIPs == "" {
		f.allowAll = true
		return f
	}
	for _, raw := range strings.Split(allowedIPs, ",") {
		ip := strings.TrimSpace(raw)
		if ip == "" {
			continue
		}
		// 先尝试 CIDR 解析
		if _, cidr, err := net.ParseCIDR(ip); err == nil {
			f.allowedCIDRs = append(f.allowedCIDRs, cidr)
			continue
		}
		// 否则尝试精确 IP
		if parsed := net.ParseIP(ip); parsed != nil {
			f.allowedExact[parsed.String()] = true
			continue
		}
		// 解析失败：忽略
	}
	return f
}

// allow 检查 IP 是否在白名单中。
func (f *metricsIPFilter) allow(ipStr string) bool {
	if f.allowAll {
		return true
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false // 无法解析的 IP 直接拒绝
	}
	if f.allowedExact[ip.String()] {
		return true
	}
	for _, cidr := range f.allowedCIDRs {
		if cidr.Contains(ip) {
			return true
		}
	}
	return false
}

// middleware 返回标准 http.Handler 中间件。
func (f *metricsIPFilter) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 解析客户端 IP（支持 X-Forwarded-For）
		ip := clientIP(r)
		if !f.allow(ip) {
			http.Error(w, "metrics access denied", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// clientIP 解析客户端真实 IP（优先 X-Forwarded-For，其次 RemoteAddr）。
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// X-Forwarded-For 格式：client, proxy1, proxy2
		// 取第一个（原始客户端）
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	// RemoteAddr 格式：IP:port
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// metricsIPFilterKey gin/kratos 中间件标识（保留扩展用）。
const metricsIPFilterKey = "metrics_ip_filter"
