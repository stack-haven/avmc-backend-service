package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

const (
	LivenessPath  = "/health/live"
	ReadinessPath = "/health/ready"
)

type Checker interface {
	Ready(context.Context) error
}

type CheckFunc func(context.Context) error

func (f CheckFunc) Ready(ctx context.Context) error {
	return f(ctx)
}

func RegisterHTTP(server *khttp.Server, checker Checker, timeout time.Duration) {
	handler := HTTPHandler(checker, timeout)
	server.Handle(LivenessPath, handler)
	server.Handle(ReadinessPath, handler)
}

func HTTPHandler(checker Checker, timeout time.Duration) http.Handler {
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			writeStatus(w, http.StatusMethodNotAllowed, "method_not_allowed")
			return
		}
		switch r.URL.Path {
		case LivenessPath:
			writeStatus(w, http.StatusOK, "ok")
		case ReadinessPath:
			if checker == nil {
				writeStatus(w, http.StatusServiceUnavailable, "unavailable")
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			if err := checker.Ready(ctx); err != nil {
				writeStatus(w, http.StatusServiceUnavailable, "unavailable")
				return
			}
			writeStatus(w, http.StatusOK, "ok")
		default:
			http.NotFound(w, r)
		}
	})
}

func writeStatus(w http.ResponseWriter, status int, value string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": value})
}
