// Package server · pprof.go
//
// pprof 调试端点挂载。仅当 EVIE_TOOL_PPROF=1 时启用。
//
// 端点（net/http/pprof 默认注册到 http.DefaultServeMux）：
//
//	/debug/pprof/             - 索引页
//	/debug/pprof/heap         - 堆 profile
//	/debug/pprof/goroutine    - goroutine profile
//	/debug/pprof/profile      - CPU profile（秒数由 ?seconds= 控制）
//	/debug/pprof/trace        - 执行 trace
//	/debug/pprof/symbol       - 符号解析
//	/debug/pprof/cmdline      - 命令行
//
// # Kratos 路由细节
//
// Kratos HTTP server 的 NotFoundHandler 默认是 http.DefaultServeMux，
// 而 net/http/pprof 在 init() 已注册到 DefaultServeMux。这意味着即使
// 我们不显式挂载，/debug/pprof/* 在 Kratos 下也是可达的。
//
// 因此本函数的语义是：
//   - env=1: 显式注册 /debug/pprof/ 等"锚点"，其余由 DefaultServeMux 提供
//   - env!=1: 注册一个前缀处理器，对所有 /debug/pprof/* 返回 404，
//     屏蔽 DefaultServeMux 的 fallback
//
// 安全：
//   - 默认关闭（env 开关），避免在生产环境误暴露 runtime 状态
//   - 不做 IP 白名单（应在部署侧 NetworkPolicy / Sidecar 控制）
//   - 调用方式：curl -sS http://host/debug/pprof/heap > heap.pprof
package server

import (
	"net/http"
	"net/http/pprof"
	"os"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

// EnvPProf 控制 pprof 挂载的环境变量名。
const EnvPProf = "EVIE_TOOL_PPROF"

// MountPProf 根据 EVIE_TOOL_PPROF 环境变量决定挂载 / 屏蔽 /debug/pprof/*。
//
// 返回 true 表示 pprof 已启用（env=1）。
func MountPProf(srv *khttp.Server) bool {
	if os.Getenv(EnvPProf) == "1" {
		srv.HandleFunc("/debug/pprof/", pprof.Index)
		srv.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
		srv.HandleFunc("/debug/pprof/profile", pprof.Profile)
		srv.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		srv.HandleFunc("/debug/pprof/trace", pprof.Trace)
		// heap/goroutine/allocs/block/mutex/threadcreate 由 DefaultServeMux 提供
		return true
	}
	// env off：屏蔽 /debug/pprof/* 的 fallback
	srv.HandlePrefix("/debug/pprof/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	return false
}
