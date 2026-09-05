package credential_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"backend-service/pkg/credential"
	credredis "backend-service/pkg/credential/redis"
)

// TestMapFromMapper 覆盖 FieldMapper 各种字段映射场景。
func TestMapFromMapper(t *testing.T) {
	tests := []struct {
		name    string
		root    map[string]any
		mapper  credential.FieldMapper
		check   func(t *testing.T, id credential.CallerIdentity)
	}{
		{
			name: "top-level fields",
			root: map[string]any{
				"tenantId":   "T1",
				"userId":     "U1",
				"userType":   float64(2),
				"expiresTime": float64(1898491296083),
			},
			mapper: credential.FieldMapper{
				TenantID: "tenantId", UserID: "userId", UserType: "userType", ExpiresAt: "expiresTime",
			},
			check: func(t *testing.T, id credential.CallerIdentity) {
				if id.TenantID != "T1" || id.UserID != "U1" {
					t.Errorf("Tenant/User mismatch: %+v", id)
				}
				if id.UserType != 2 {
					t.Errorf("UserType = %d, want 2", id.UserType)
				}
				// expiresTime = 1898491296083 ms = 2030-02-28 ish
				if id.ExpiresAt.UnixMilli() != 1898491296083 {
					t.Errorf("ExpiresAt = %d, want 1898491296083", id.ExpiresAt.UnixMilli())
				}
			},
		},
		{
			name: "nested dotted path",
			root: map[string]any{
				"tenantId": "T1",
				"userInfo": map[string]any{"nickname": "Alice", "deptId": "D9"},
			},
			mapper: credential.FieldMapper{
				TenantID: "tenantId", UserName: "userInfo.nickname", DeptID: "userInfo.deptId",
			},
			check: func(t *testing.T, id credential.CallerIdentity) {
				if id.UserName != "Alice" || id.DeptID != "D9" {
					t.Errorf("nested fields mismatch: %+v", id)
				}
			},
		},
		{
			name: "missing fields stay zero",
			root: map[string]any{"tenantId": "T1"},
			mapper: credential.FieldMapper{
				TenantID: "tenantId", UserID: "userId", UserName: "userInfo.nickname",
			},
			check: func(t *testing.T, id credential.CallerIdentity) {
				if id.UserID != "" || id.UserName != "" {
					t.Errorf("expected zero values, got %+v", id)
				}
			},
		},
		{
			name: "empty mapper returns zero identity with raw preserved",
			root: map[string]any{"foo": "bar"},
			mapper: credential.FieldMapper{},
			check: func(t *testing.T, id credential.CallerIdentity) {
				if id.TenantID != "" || id.UserID != "" {
					t.Errorf("expected empty identity, got %+v", id)
				}
				if id.Raw == nil {
					t.Errorf("Raw should be preserved even with empty mapper")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			id := credential.MapFromMapper(tt.root, tt.mapper)
			tt.check(t, id)
		})
	}
}

func TestParseExpiresAt(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want int64 // expected unix-ms; 0 = zero time
		ok   bool
	}{
		{"nil", nil, 0, false},
		{"epoch ms float64", float64(1898491296083), 1898491296083, true},
		{"epoch s float64", float64(1898491296), 1898491296000, true},
		{"epoch ms int64", int64(1898491296083), 1898491296083, true},
		{"epoch s int64", int64(1898491296), 1898491296000, true},
		{"json.Number ms", json.Number("1898491296083"), 1898491296083, true},
		{"RFC3339", "2030-02-28T10:00:00Z", 0, true}, // exact ms not asserted
		{"empty string", "", 0, false},
		{"garbage string", "not-a-time", 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := credential.ParseExpiresAt(tt.in)
			if ok != tt.ok {
				t.Errorf("ok = %v, want %v", ok, tt.ok)
				return
			}
			if tt.ok && tt.want != 0 && got.UnixMilli() != tt.want {
				t.Errorf("unix-ms = %d, want %d", got.UnixMilli(), tt.want)
			}
		})
	}
}

func TestExtractString(t *testing.T) {
	tests := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"hello", "hello"},
		{float64(42), "42"},
		{float64(3.14), "3.14"},
		{int64(42), "42"},
		{json.Number("100"), "100"},
		{true, "true"},
	}
	for _, tt := range tests {
		got := credential.ExtractString(tt.in)
		if got != tt.want {
			t.Errorf("ExtractString(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestLookupPath(t *testing.T) {
	root := map[string]any{
		"a": map[string]any{"b": map[string]any{"c": "deep"}},
		"top": "value",
	}
	tests := []struct {
		path string
		want any
	}{
		{"a.b.c", "deep"},
		{"top", "value"},
		{"a.b", map[string]any{"c": "deep"}},
		{"missing", nil},
		{"a.missing", nil},
		{"", nil},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := credential.LookupPath(root, tt.path)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("LookupPath(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

// TestRedisProvider_EndToEnd 用 miniredis 覆盖真实 Redis 路径。
func TestRedisProvider_EndToEnd(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	prov, err := credredis.New(credredis.Config{
		Client:    rdb,
		KeyPrefix: "test:",
		Fields: credential.FieldMapper{
			TenantID: "tenantId", UserID: "userId", UserName: "userInfo.nickname",
			UserType: "userType", ExpiresAt: "expiresTime",
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	payload := map[string]any{
		"tenantId": "T1", "userId": "U1",
		"userType":     float64(2),
		"expiresTime":  float64(1898491296083),
		"accessToken":  "echo-back",
		"userInfo":     map[string]any{"nickname": "Alice"},
	}
	raw, _ := json.Marshal(payload)
	mr.Set("test:good-token", string(raw))

	t.Run("found", func(t *testing.T) {
		id, err := prov.Authenticate(context.Background(), "good-token")
		if err != nil {
			t.Fatalf("Authenticate: %v", err)
		}
		if id.TenantID != "T1" || id.UserID != "U1" || id.UserName != "Alice" {
			t.Errorf("identity mismatch: %+v", id)
		}
		// AccessToken echoes the bearer itself
		if id.AccessToken != "good-token" {
			t.Errorf("AccessToken = %q, want %q", id.AccessToken, "good-token")
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := prov.Authenticate(context.Background(), "missing")
		if !errors.Is(err, credential.ErrTokenNotFound) {
			t.Errorf("err = %v, want ErrTokenNotFound", err)
		}
	})

	t.Run("empty token", func(t *testing.T) {
		_, err := prov.Authenticate(context.Background(), "")
		if !errors.Is(err, credential.ErrTokenNotFound) {
			t.Errorf("err = %v, want ErrTokenNotFound", err)
		}
	})

	t.Run("malformed json", func(t *testing.T) {
		mr.Set("test:bad-json", "{not json")
		_, err := prov.Authenticate(context.Background(), "bad-json")
		if !errors.Is(err, credential.ErrTokenInvalid) {
			t.Errorf("err = %v, want ErrTokenInvalid", err)
		}
	})
}
