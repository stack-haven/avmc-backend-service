package health

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type detailsChecker struct{}

func (detailsChecker) Ready(context.Context) error { return nil }
func (detailsChecker) Details(context.Context) map[string]any {
	return map[string]any{"cache": map[string]any{"hits": 2}}
}

func TestHTTPHandler(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		path    string
		checker Checker
		want    int
	}{
		{name: "live", method: http.MethodGet, path: LivenessPath, want: http.StatusOK},
		{name: "head", method: http.MethodHead, path: LivenessPath, want: http.StatusOK},
		{name: "ready", method: http.MethodGet, path: ReadinessPath, checker: CheckFunc(func(context.Context) error { return nil }), want: http.StatusOK},
		{name: "not ready", method: http.MethodGet, path: ReadinessPath, checker: CheckFunc(func(context.Context) error { return errors.New("down") }), want: http.StatusServiceUnavailable},
		{name: "missing checker", method: http.MethodGet, path: ReadinessPath, want: http.StatusServiceUnavailable},
		{name: "wrong method", method: http.MethodPost, path: LivenessPath, want: http.StatusMethodNotAllowed},
		{name: "other path", method: http.MethodGet, path: "/api", want: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := HTTPHandler(tt.checker, time.Second)
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, httptest.NewRequest(tt.method, tt.path, nil))

			if recorder.Code != tt.want {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.want)
			}
			if recorder.Header().Get("Cache-Control") != "no-store" && tt.want != http.StatusNotFound {
				t.Fatal("health responses must not be cached")
			}
		})
	}
}

func TestHTTPHandlerReadinessTimeout(t *testing.T) {
	checker := CheckFunc(func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	})
	handler := HTTPHandler(checker, time.Millisecond)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, ReadinessPath, nil))

	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d", recorder.Code)
	}
}

func TestHTTPHandlerReadinessDetails(t *testing.T) {
	handler := HTTPHandler(detailsChecker{}, time.Second)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, ReadinessPath, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("status body = %+v", body)
	}
	details, ok := body["details"].(map[string]any)
	if !ok {
		t.Fatalf("details missing: %+v", body)
	}
	if _, ok := details["cache"].(map[string]any); !ok {
		t.Fatalf("cache details missing: %+v", details)
	}
}
