package biz_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"backend-service/pkg/textenhance"
	"backend-service/pkg/textenhance/builtins"
	"backend-service/pkg/textenhance/observers"

	v1conf "backend-service/app/evie/tool/internal/conf"
	"backend-service/app/evie/tool/internal/biz"
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

// TestEnhancementUsecase_EndToEnd 端到端：Usecase + Pipeline + 系统词库。
func TestEnhancementUsecase_EndToEnd(t *testing.T) {
	dictPath := writeTestSystemDict(t)

	// 1. conf
	conf := &v1conf.Enhancement{
		Pipeline:               []string{"cleaning", "filler", "vocab_matching", "alias_resolution", "deterministic_replacement", "phrase_standardization", "pinyin_correction", "fuzzy_matching", "context_correction"},
		PinyinThreshold:        0.85,
		FuzzyAutoThreshold:     0.80,
		FuzzySuggestThreshold:  0.60,
		LlmEnabled:            false,
	}
	sysConf := &v1conf.SystemDict{Path: dictPath}

	// 2. VocabularyBuilder（从 system.json 加载）
	vb, err := biz.NewVocabularyBuilder(sysConf)
	if err != nil {
		t.Fatalf("NewVocabularyBuilder: %v", err)
	}

	// 3. Pipeline（带 observer）
	reg := builtins.NewDefaultRegistry()
	policy := biz.NewPolicyFromConf(conf)
	pipeline, err := textenhance.BuildPipeline(reg, policy,
		textenhance.WithObservers(observers.NewCountingObserver()),
	)
	if err != nil {
		t.Fatalf("BuildPipeline: %v", err)
	}

	// 4. Usecase
	uc := biz.NewEnhancementUsecase(pipeline, vb, policy)

	// 5. 跑增强
	tests := []struct {
		name  string
		input string
		want  string // 期望 enhanced_text（allow 多个等价）
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
			want:  " 金种籽 是 1颗种籽", // leading 空格保留；连续空格被 cleaning 合并
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := uc.EnhanceText(context.Background(), tt.input, "158")
			if err != nil {
				t.Fatalf("EnhanceText: %v", err)
			}
			if res.EnhancedText != tt.want {
				t.Errorf("EnhancedText = %q, want %q", res.EnhancedText, tt.want)
			}
			if res.Status != textenhance.StatusSuccess {
				t.Errorf("Status = %d, want SUCCESS", res.Status)
			}
		})
	}
}

func TestEnhancementUsecase_EmptyText(t *testing.T) {
	dictPath := writeTestSystemDict(t)
	sysConf := &v1conf.SystemDict{Path: dictPath}
	vb, _ := biz.NewVocabularyBuilder(sysConf)
	policy := biz.NewPolicyFromConf(&v1conf.Enhancement{})

	reg := builtins.NewDefaultRegistry()
	pipeline, _ := textenhance.BuildPipeline(reg, policy)

	uc := biz.NewEnhancementUsecase(pipeline, vb, policy)

	_, err := uc.EnhanceText(context.Background(), "", "158")
	if err == nil {
		t.Error("expected error for empty text")
	}
}

func TestEnhancementUsecase_EmptyTenant(t *testing.T) {
	dictPath := writeTestSystemDict(t)
	sysConf := &v1conf.SystemDict{Path: dictPath}
	vb, _ := biz.NewVocabularyBuilder(sysConf)
	policy := biz.NewPolicyFromConf(&v1conf.Enhancement{})

	reg := builtins.NewDefaultRegistry()
	pipeline, _ := textenhance.BuildPipeline(reg, policy)

	uc := biz.NewEnhancementUsecase(pipeline, vb, policy)

	_, err := uc.EnhanceText(context.Background(), "hello", "")
	if err == nil {
		t.Error("expected error for empty tenantID")
	}
}

func TestNewPolicyFromConf_NilConf(t *testing.T) {
	p := biz.NewPolicyFromConf(nil)
	if p == nil {
		t.Fatal("expected non-nil default policy")
	}
	if !p.IsEnabled("cleaning") {
		t.Error("default policy should enable all")
	}
}

func TestNewPolicyFromConf_InvalidThreshold(t *testing.T) {
	c := &v1conf.Enhancement{
		Pipeline:              []string{"cleaning"},
		PinyinThreshold:       2.0, // 越界
		FuzzyAutoThreshold:    0.5, // 合法
		FuzzySuggestThreshold: 0.4, // 合法
	}
	p := biz.NewPolicyFromConf(c)
	if p.PinyinThreshold > 1.0 {
		t.Errorf("PinyinThreshold = %v, should be clamped to <= 1", p.PinyinThreshold)
	}
}

func TestVocabularyBuilder_LoadFromFile(t *testing.T) {
	dictPath := writeTestSystemDict(t)
	vb, err := biz.NewVocabularyBuilder(&v1conf.SystemDict{Path: dictPath})
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
	vb, err := biz.NewVocabularyBuilder(&v1conf.SystemDict{Path: "/nonexistent/file.json"})
	if err != nil {
		t.Fatalf("HA: should NOT error on missing file: %v", err)
	}
	snap := vb.Build(context.Background(), "158")
	if snap.EntryCount() != 0 {
		t.Errorf("expected empty vocab, got %d entries", snap.EntryCount())
	}
}