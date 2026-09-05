package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	httpsrc "backend-service/app/evie/tool/pkg/source/http"
)

// helper: build a test server with handler and return its URL.
func mockServer(t *testing.T, h http.HandlerFunc) *httptest.Server {
	t.Helper()
	return httptest.NewServer(h)
}

func TestHTTPSource_FetchUsers(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"list": []map[string]any{
				{"id": "u1", "name": "Alice"},
				{"id": "u2", "name": "Bob"},
			}},
		})
	})
	defer srv.Close()

	src, err := httpsrc.New(httpsrc.Config{
		BaseURL:  srv.URL,
		UserPath: "/users",
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	entities, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(entities) != 2 {
		t.Fatalf("entities = %d, want 2", len(entities))
	}
	if entities[0].SourceID != "u1" || entities[0].EntityType != "user" {
		t.Errorf("entity mismatch: %+v", entities[0])
	}
}

func TestHTTPSource_FetchDepts_TopLevelArray(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "d1", "name": "Engineering"},
		})
	})
	defer srv.Close()

	src, _ := httpsrc.New(httpsrc.Config{
		BaseURL:  srv.URL,
		DeptPath: "/depts",
	})
	entities, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(entities) != 1 || entities[0].SourceID != "d1" {
		t.Errorf("depts mismatch: %+v", entities)
	}
}

func TestHTTPSource_QueryParamsAndHeaders(t *testing.T) {
	var capturedQuery, capturedAuth, capturedZone string
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedQuery = r.URL.RawQuery
		capturedAuth = r.Header.Get("Authorization")
		capturedZone = r.Header.Get("zone")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "u1"}}})
	})
	defer srv.Close()

	src, _ := httpsrc.New(httpsrc.Config{
		BaseURL:     srv.URL,
		UserPath:    "/users",
		DeptPath:    "/depts",
		QueryParams: map[string]string{"selectAll": "true"},
		StaticToken: "test-tok",
		Headers:     map[string]string{"zone": "Asia/Shanghai"},
	})
	_, _ = src.Fetch(context.Background())

	if capturedQuery != "selectAll=true" {
		t.Errorf("query = %q, want selectAll=true", capturedQuery)
	}
	if capturedAuth != "Bearer test-tok" {
		t.Errorf("auth = %q, want Bearer test-tok", capturedAuth)
	}
	if capturedZone != "Asia/Shanghai" {
		t.Errorf("zone = %q, want Asia/Shanghai", capturedZone)
	}
}

func TestHTTPSource_TokenProviderFromCtx(t *testing.T) {
	var capturedAuth string
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "u1"}}})
	})
	defer srv.Close()

	src, _ := httpsrc.New(httpsrc.Config{
		BaseURL:       srv.URL,
		UserPath:      "/users",
		TokenProvider: httpsrc.TokenFunc(func(_ context.Context) (string, error) { return "ctx-tok", nil }),
	})
	ctx := context.WithValue(context.Background(), ctxKey{}, true)
	_, _ = src.Fetch(ctx)
	if capturedAuth != "Bearer ctx-tok" {
		t.Errorf("auth = %q, want Bearer ctx-tok", capturedAuth)
	}
}

func TestHTTPSource_TokenProviderError(t *testing.T) {
	sentinel := errors.New("no token")
	src, _ := httpsrc.New(httpsrc.Config{
		BaseURL:       "http://example.test",
		UserPath:      "/users",
		TokenProvider: httpsrc.TokenFunc(func(_ context.Context) (string, error) { return "", sentinel }),
	})
	_, err := src.Fetch(context.Background())
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want sentinel", err)
	}
}

func TestHTTPSource_TenantIDProviderFromCtx(t *testing.T) {
	var capturedTenant string
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		capturedTenant = r.Header.Get("tenant-id")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{{"id": "u1"}}})
	})
	defer srv.Close()

	src, _ := httpsrc.New(httpsrc.Config{
		BaseURL:        srv.URL,
		UserPath:       "/users",
		TenantHeader:   "tenant-id",
		TenantIDProvider: httpsrc.TenantIDFunc(func(_ context.Context) (string, error) { return "42", nil }),
	})
	_, _ = src.Fetch(context.Background())
	if capturedTenant != "42" {
		t.Errorf("tenant-id = %q, want 42", capturedTenant)
	}
}

func TestHTTPSource_CodeErrorMap(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 401, "msg": "unauthorized",
			"data": map[string]any{"list": []map[string]any{{"id": "u1"}}},
		})
	})
	defer srv.Close()

	sentinel := errors.New("custom 401")
	src, _ := httpsrc.New(httpsrc.Config{
		BaseURL:  srv.URL,
		UserPath: "/users",
		Envelope: httpsrc.Envelope{
			UsersPath: "data.list",
			CodePath:  "code",
			CodeOK:    0,
		},
		CodeErrorMap: map[int]func(code int, msg string) error{
			401: func(_ int, _ string) error { return sentinel },
		},
	})
	_, err := src.Fetch(context.Background())
	if !errors.Is(err, sentinel) {
		t.Errorf("err = %v, want sentinel", err)
	}
}

func TestHTTPSource_PartialFailure(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"data": map[string]any{"list": []map[string]any{{"id": "u1"}}},
		})
	})
	mux.HandleFunc("/depts", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src, _ := httpsrc.New(httpsrc.Config{
		BaseURL:  srv.URL,
		UserPath: "/users",
		DeptPath: "/depts",
	})
	entities, err := src.Fetch(context.Background())
	if err == nil {
		t.Fatalf("expected error from dept endpoint")
	}
	if len(entities) != 1 {
		t.Errorf("partial: expected 1 user, got %d", len(entities))
	}
}

func TestHTTPSource_NewValidation(t *testing.T) {
	_, err := httpsrc.New(httpsrc.Config{BaseURL: ""})
	if err == nil {
		t.Errorf("expected error for missing BaseURL")
	}
	_, err = httpsrc.New(httpsrc.Config{BaseURL: "http://x"})
	if err == nil {
		t.Errorf("expected error for missing both paths")
	}
}

func TestHTTPSource_BadJSON(t *testing.T) {
	srv := mockServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "not json")
	})
	defer srv.Close()
	src, _ := httpsrc.New(httpsrc.Config{BaseURL: srv.URL, UserPath: "/users"})
	_, err := src.Fetch(context.Background())
	if err == nil {
		t.Errorf("expected decode error")
	}
	if !contains(err.Error(), "decode") {
		t.Errorf("err = %v, want decode-related", err)
	}
}

func contains(s, sub string) bool {
	return fmt.Sprintf("%s", s) != "" && (len(sub) == 0 || (len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)))
}
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// ctxKey is unexported to avoid collisions.
type ctxKey struct{}
