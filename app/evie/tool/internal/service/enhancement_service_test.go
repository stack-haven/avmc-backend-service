package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kratos/kratos/v2/log"

	v1 "backend-service/api/evie/tool/v1"
	v1conf "backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/data"
	"backend-service/app/evie/tool/internal/service"
)

// ctxWithAuth 构造含 AuthInfo 的 ctx（绕过 Redis 真实调用）。
func ctxWithAuth(tenantID string) context.Context {
	auth := &data.AuthInfo{
		TenantID:    tenantID,
		UserID:      "2031552504886435841",
		AccessToken: "mock-token",
	}
	return biz.WithAuth(context.Background(), auth)
}

func writeServiceTestDict(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "system.json")
	content := `{
  "version": "test",
  "entries": [
    {"standard_text": "金种籽", "category": "PRODUCT", "aliases": ["金种子"]}
  ]
}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write dict: %v", err)
	}
	return path
}

func newTestService(t *testing.T) *service.EnhancementService {
	t.Helper()
	dictPath := writeServiceTestDict(t)

	vb, _ := biz.NewVocabularyBuilder(&v1conf.SystemDict{Path: dictPath})
	engine, err := biz.NewLexnormEngine(&v1conf.Enhancement{}, vb, log.DefaultLogger)
	if err != nil {
		t.Fatalf("NewLexnormEngine: %v", err)
	}
	uc := biz.NewEnhancementUsecase(engine)

	// 使用 kratos default logger（避免 nil panic）
	return service.NewEnhancementService(uc, log.DefaultLogger)
}

func TestEnhancementService_EnhanceText_Success(t *testing.T) {
	svc := newTestService(t)

	resp, err := svc.EnhanceText(ctxWithAuth("158"), &v1.EnhanceTextRequest{
		Text: "金种子是新产品",
	})
	if err != nil {
		t.Fatalf("EnhanceText: %v", err)
	}
	if resp.EnhancedText != "金种籽是新产品" {
		t.Errorf("EnhancedText = %q, want %q", resp.EnhancedText, "金种籽是新产品")
	}
	if resp.Status > 2 {
		t.Errorf("Status = %d, expected <= 2 (success/partial/canceled)", resp.Status)
	}
	t.Logf("Status = %d", resp.Status)
}

func TestEnhancementService_EnhanceText_NoAuth(t *testing.T) {
	svc := newTestService(t)

	// 不放 AuthInfo → 应返回 error
	_, err := svc.EnhanceText(context.Background(), &v1.EnhanceTextRequest{
		Text: "test",
	})
	if err == nil {
		t.Fatal("expected error for missing auth")
	}
}

func TestEnhancementService_EnhanceText_EmptyText(t *testing.T) {
	svc := newTestService(t)

	_, err := svc.EnhanceText(ctxWithAuth("158"), &v1.EnhanceTextRequest{
		Text: "",
	})
	if err == nil {
		t.Fatal("expected error for empty text")
	}
}
