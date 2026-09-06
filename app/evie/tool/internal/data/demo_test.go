package data

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"backend-service/app/evie/tool/internal/conf"
)

func TestStaticTokenCache_GetAndKey(t *testing.T) {
	cfg := &conf.StaticCredential{
		DefaultTenant: "demo",
		Users: []*conf.StaticUser{
			{Token: "tok-a", TenantId: "tenantA", UserId: "u-a", UserType: 1},
			{Token: "tok-b", TenantId: "tenantB", UserId: "u-b"},
		},
	}
	cache, err := NewStaticTokenCache(cfg)
	if err != nil {
		t.Fatalf("NewStaticTokenCache: %v", err)
	}
	info, err := cache.Get(context.Background(), "tok-a")
	if err != nil {
		t.Fatalf("Get tok-a: %v", err)
	}
	if info.TenantID != "tenantA" || info.UserID != "u-a" || info.UserType != 1 {
		t.Errorf("info mismatch: %+v", info)
	}
	// 不存在的 token。
	if _, err := cache.Get(context.Background(), "nope"); err == nil {
		t.Error("expected error for unknown token")
	}
	// Key 不为空。
	if cache.Key("tok-a") == "" {
		t.Error("Key should not be empty")
	}
}

func TestFileVocabularySource_Fetch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vocab.json")
	content := `{
	  "version":"v1",
	  "users":[{"id":"u1","name":"Alice","mobile":"13800000001"}],
	  "departments":[{"id":"d1","name":"工程部"}]
	}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	src, err := NewFileVocabularySource(&conf.FileVocabulary{Path: path})
	if err != nil {
		t.Fatalf("NewFileVocabularySource: %v", err)
	}
	entities, err := src.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(entities) != 2 {
		t.Fatalf("entities = %d, want 2", len(entities))
	}
	if src.Name() != "file" {
		t.Errorf("Name = %q, want file", src.Name())
	}
}

func TestNewTokenCacheFromConf_NotStatic(t *testing.T) {
	if _, err := NewTokenCacheFromConf(&conf.Credential{Provider: "redis"}); err == nil {
		t.Error("expected error for non-static provider")
	}
	if _, err := NewTokenCacheFromConf(nil); err == nil {
		t.Error("expected error for nil cred")
	}
}

func TestNewVocabSourceFromConf_NotFile(t *testing.T) {
	if _, err := NewVocabularySourceFromConf(&conf.Vocabulary{Source: "qua"}); err == nil {
		t.Error("expected error for non-file source")
	}
	if _, err := NewVocabularySourceFromConf(nil); err == nil {
		t.Error("expected error for nil vocab")
	}
}
