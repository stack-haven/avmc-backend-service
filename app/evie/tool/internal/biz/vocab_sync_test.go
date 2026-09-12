package biz

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	v1conf "backend-service/app/evie/tool/internal/conf"
)

// TestTenantRegistry_LoadWithExpires 验证 sync_token_expires_at 字段解析。
func TestTenantRegistry_LoadWithExpires(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tenants.json")
	content := `[
		{"id": "1", "sync_token": "tok-1", "sync_token_expires_at": "2030-12-31T23:59:59+08:00"},
		{"id": "2", "sync_token": "tok-2"},
		{"id": "3"}
	]`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewTenantRegistry(&v1conf.TenantRegistry{Path: path})

	// id=1: 有 token + 过期时间
	if r.GetSyncToken("1") != "tok-1" {
		t.Errorf("token 1 = %q, want tok-1", r.GetSyncToken("1"))
	}
	exp := r.SyncTokenExpiresAt("1")
	if exp.IsZero() {
		t.Error("id=1 expires_at should not be zero")
	}
	want := time.Date(2030, 12, 31, 23, 59, 59, 0, time.FixedZone("CST", 8*3600))
	if !exp.Equal(want) {
		t.Errorf("expires_at = %v, want %v", exp, want)
	}

	// id=2: 有 token 但无过期时间（兼容旧配置）
	if r.GetSyncToken("2") != "tok-2" {
		t.Errorf("token 2 = %q", r.GetSyncToken("2"))
	}
	if !r.SyncTokenExpiresAt("2").IsZero() {
		t.Error("id=2 expires_at should be zero (no expiry configured)")
	}

	// id=3: 无 token
	if r.GetSyncToken("3") != "" {
		t.Errorf("token 3 should be empty")
	}
}

// TestTenantRegistry_ExpiringAndExpired 验证即将过期 / 已过期 分类。
func TestTenantRegistry_ExpiringAndExpired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tenants.json")
	now := time.Now()
	soon := now.Add(30 * time.Minute).Format(time.RFC3339)  // 30 分钟后过期
	expired := now.Add(-1 * time.Hour).Format(time.RFC3339) // 1 小时前过期
	farFuture := now.Add(24 * time.Hour).Format(time.RFC3339)
	content := `[
		{"id": "soon",    "sync_token": "t1", "sync_token_expires_at": "` + soon + `"},
		{"id": "expired", "sync_token": "t2", "sync_token_expires_at": "` + expired + `"},
		{"id": "future",  "sync_token": "t3", "sync_token_expires_at": "` + farFuture + `"},
		{"id": "no-exp",  "sync_token": "t4"}
	]`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewTenantRegistry(&v1conf.TenantRegistry{Path: path})

	// 1h 内即将过期
	exp1h := r.ExpiringTenants(time.Hour)
	if !contains(exp1h, "soon") || !contains(exp1h, "expired") {
		t.Errorf("ExpiringTenants(1h) = %v, want contains soon+expired", exp1h)
	}
	if contains(exp1h, "future") {
		t.Errorf("future should not be in ExpiringTenants(1h): %v", exp1h)
	}

	// 已过期
	exp := r.ExpiredTenants()
	if !contains(exp, "expired") {
		t.Errorf("ExpiredTenants = %v, want contains expired", exp)
	}
	if contains(exp, "soon") {
		t.Errorf("soon should not be in ExpiredTenants: %v", exp)
	}

	// snapshot
	snaps := r.SnapshotAll()
	for _, s := range snaps {
		switch s.ID {
		case "soon":
			if !s.IsExpiringSoon || s.IsExpired {
				t.Errorf("soon snapshot wrong: %+v", s)
			}
		case "expired":
			if !s.IsExpired {
				t.Errorf("expired snapshot wrong: %+v", s)
			}
		case "future":
			if s.IsExpiringSoon || s.IsExpired {
				t.Errorf("future snapshot wrong: %+v", s)
			}
		case "no-exp":
			if s.IsExpiringSoon || s.IsExpired {
				t.Errorf("no-exp snapshot wrong: %+v", s)
			}
		}
	}
}

// TestTenantRegistry_InvalidExpires 验证格式错误的 expires_at 不会让加载失败。
func TestTenantRegistry_InvalidExpires(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tenants.json")
	content := `[
		{"id": "1", "sync_token": "t", "sync_token_expires_at": "not-a-date"}
	]`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	r := NewTenantRegistry(&v1conf.TenantRegistry{Path: path})

	if r.GetSyncToken("1") != "t" {
		t.Error("token should still load despite invalid expires_at")
	}
	if !r.SyncTokenExpiresAt("1").IsZero() {
		t.Error("invalid expires_at should result in zero time")
	}
}

// TestTenantRegistry_SetSyncTokenWithExpiry 验证 refresher 用法。
func TestTenantRegistry_SetSyncTokenWithExpiry(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tenants.json")
	if err := os.WriteFile(path, []byte("[]"), 0644); err != nil {
		t.Fatal(err)
	}
	r := NewTenantRegistry(&v1conf.TenantRegistry{Path: path})
	exp := time.Now().Add(2 * time.Hour)

	r.SetSyncTokenWithExpiry("t1", "new-tok", exp)

	if r.GetSyncToken("t1") != "new-tok" {
		t.Errorf("token = %q", r.GetSyncToken("t1"))
	}
	if !r.SyncTokenExpiresAt("t1").Equal(exp) {
		t.Errorf("expires = %v, want %v", r.SyncTokenExpiresAt("t1"), exp)
	}
	if r.LastRefreshAt("t1").IsZero() {
		t.Error("LastRefreshAt should be set on SetSyncTokenWithExpiry")
	}
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
