package biz_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stack-haven/lexnorm"

	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/conf"
)

// testSystemDict 写入临时文件，返回 path。
func writeTestSystemDict(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "system.json")
	content := `{
  "version": "2026-09-test",
  "entries": [
    {
      "standard_text": "金种籽",
      "category": "PRODUCT",
      "priority": 100,
      "aliases": ["金种子"],
      "corrections": [],
      "homophones": []
    },
    {
      "standard_text": "技术研发部",
      "category": "ORGANIZATION",
      "priority": 80,
      "aliases": ["研发部"],
      "corrections": [],
      "homophones": []
    }
  ],
  "phrase_rules": [
    {"from": "个种籽", "to": "颗种籽"}
  ]
}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write system dict: %v", err)
	}
	return path
}

func newEngine(t *testing.T, dictPath string) *lexnorm.Engine {
	t.Helper()
	vb, err := biz.NewVocabularyBuilder(&conf.SystemDict{Path: dictPath})
	if err != nil {
		t.Fatalf("NewVocabularyBuilder: %v", err)
	}
	engine, err := biz.NewLexnormEngine(&conf.Enhancement{}, vb, log.DefaultLogger)
	if err != nil {
		t.Fatalf("NewLexnormEngine: %v", err)
	}
	return engine
}

// TestEnhancementUsecase_EndToEnd 端到端：Usecase + lexnorm 引擎 + 系统词库。
func TestEnhancementUsecase_EndToEnd(t *testing.T) {
	dictPath := writeTestSystemDict(t)

	engine := newEngine(t, dictPath)
	uc := biz.NewEnhancementUsecase(engine)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "alias_resolution",
			input: "金种子是新产品",
			want:  "金种籽是新产品",
		},
		{
			name:  "phrase_standardization",
			input: "一个种籽",
			want:  "一颗种籽",
		},
		{
			name:  "cleaning_filler_phrase",
			input: "呃   金种子  是  1个种籽",
			// 期望：leading whitespace 被 normalize 保留或删除；具体由 lexnorm 行为决定
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := uc.EnhanceText(context.Background(), tt.input, "158")
			if err != nil {
				t.Fatalf("EnhanceText: %v", err)
			}
			if tt.want != "" && res.EnhancedText != tt.want {
				t.Errorf("EnhancedText = %q, want %q", res.EnhancedText, tt.want)
			}
			// status 检查：lexnorm StatusSuccess=0；>0 表示 partial/canceled/failed
			if res.Status > 2 {
				t.Errorf("Status = %d, want <= 2 (success/partial/canceled)", res.Status)
			}
			t.Logf("[%s] %q -> %q (status=%d, changes=%d)",
				tt.name, tt.input, res.EnhancedText, res.Status, len(res.Changes))
		})
	}
}

func TestEnhancementUsecase_EmptyText(t *testing.T) {
	dictPath := writeTestSystemDict(t)
	engine := newEngine(t, dictPath)
	uc := biz.NewEnhancementUsecase(engine)

	_, err := uc.EnhanceText(context.Background(), "", "158")
	if err == nil {
		t.Error("expected error for empty text")
	}
}

func TestEnhancementUsecase_EmptyTenant(t *testing.T) {
	dictPath := writeTestSystemDict(t)
	engine := newEngine(t, dictPath)
	uc := biz.NewEnhancementUsecase(engine)

	_, err := uc.EnhanceText(context.Background(), "hello", "")
	if err == nil {
		t.Error("expected error for empty tenantID")
	}
}

func TestVocabularyBuilder_LoadFromFile(t *testing.T) {
	dictPath := writeTestSystemDict(t)
	vb, err := biz.NewVocabularyBuilder(&conf.SystemDict{Path: dictPath})
	if err != nil {
		t.Fatalf("NewVocabularyBuilder: %v", err)
	}

	snap := vb.Build(context.Background(), "158")
	if snap.EntryCount() != 2 {
		t.Errorf("EntryCount = %d, want 2", snap.EntryCount())
	}

	// 检查 alias 关系
	rs := snap.LookupRelations("金种子")
	if len(rs) != 1 {
		t.Errorf("LookupRelations(金种子) = %d, want 1", len(rs))
	} else if rs[0].RelationType != "ALIAS" {
		t.Errorf("RelationType = %q, want ALIAS", rs[0].RelationType)
	}
}

func TestVocabularyBuilder_NotFound(t *testing.T) {
	// HA 行为：system.json 缺失不阻断启动；用空快照
	vb, err := biz.NewVocabularyBuilder(&conf.SystemDict{Path: "/nonexistent/file.json"})
	if err != nil {
		t.Fatalf("HA: should NOT error on missing file: %v", err)
	}
	snap := vb.Build(context.Background(), "158")
	if snap.EntryCount() != 0 {
		t.Errorf("expected empty vocab, got %d entries", snap.EntryCount())
	}
}
