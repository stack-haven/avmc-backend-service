package service_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kratos/kratos/v2/log"

	"backend-service/pkg/textenhance"
	"backend-service/pkg/textenhance/builtins"

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

	conf := &v1conf.Enhancement{
		Pipeline: []string{"cleaning", "filler", "vocab_matching", "alias_resolution", "deterministic_replacement", "phrase_standardization", "pinyin_correction", "fuzzy_matching", "context_correction"},
	}
	vb, _ := biz.NewVocabularyBuilder(&v1conf.SystemDict{Path: dictPath})
	policy := biz.NewPolicyFromConf(conf)
	reg := builtins.NewDefaultRegistry()
	pipeline, _ := textenhance.BuildPipeline(reg, policy)
	uc := biz.NewEnhancementUsecase(pipeline, vb, policy)

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
	if resp.Status == 0 {
		t.Errorf("Status = 0, expected > 0 (textenhance StatusSuccess=%d)", textenhance.StatusSuccess)
	}
	t.Logf("Status = %d (textenhance StatusSuccess=%d)", resp.Status, textenhance.StatusSuccess)
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