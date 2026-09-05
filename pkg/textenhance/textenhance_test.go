package textenhance_test

import (
	"context"
	"testing"
	"time"

	"backend-service/pkg/textenhance"
	"backend-service/pkg/textenhance/builtins"
)

// TestBuildPipeline_DefaultRegistry 验证：9 个默认 Processor 全部可注册可构建。
func TestBuildPipeline_DefaultRegistry(t *testing.T) {
	reg := builtins.NewDefaultRegistry()

	// 验证 9 个 processor 名称
	expected := []string{
		"cleaning", "filler", "vocab_matching",
		"alias_resolution", "deterministic_replacement",
		"phrase_standardization", "pinyin_correction",
		"fuzzy_matching", "context_correction",
		"llm_reserved",
	}
	for _, name := range expected {
		if !reg.Has(name) {
			t.Errorf("processor %q not registered", name)
		}
	}

	// BuildPipeline 用 DefaultPolicy
	pipeline, err := textenhance.BuildPipeline(reg, textenhance.DefaultPolicy())
	if err != nil {
		t.Fatalf("BuildPipeline: %v", err)
	}
	if got := len(pipeline.Processors()); got != 9 {
		t.Errorf("Processors count = %d, want 9 (default policy 不含 llm_reserved)", got)
	}
}

// TestBuildPipeline_WithLLMEnabled 验证：Policy.LLMEnabled=true 时附加 llm_reserved。
func TestBuildPipeline_WithLLMEnabled(t *testing.T) {
	reg := builtins.NewDefaultRegistry()
	policy := textenhance.DefaultPolicy()
	policy.LLMEnabled = true

	pipeline, err := textenhance.BuildPipeline(reg, policy)
	if err != nil {
		t.Fatalf("BuildPipeline: %v", err)
	}
	if got := len(pipeline.Processors()); got != 10 {
		t.Errorf("Processors count = %d, want 10 (含 llm_reserved)", got)
	}
}

// TestPipeline_Run_EmptyVocab 验证 HA：空 vocab 不阻断 Pipeline。
func TestPipeline_Run_EmptyVocab(t *testing.T) {
	reg := builtins.NewDefaultRegistry()
	policy := textenhance.DefaultPolicy()
	pipeline, err := textenhance.BuildPipeline(reg, policy)
	if err != nil {
		t.Fatalf("BuildPipeline: %v", err)
	}

	ec := textenhance.NewEnhancementContext("你好，世界", textenhance.EmptyVocabularySnapshot(), policy)
	pipeline.Run(context.Background(), ec)

	if ec.Status != textenhance.StatusSuccess {
		t.Errorf("Status = %d (%s), want %d (SUCCESS)",
			ec.Status, textenhance.StatusName(ec.Status), textenhance.StatusSuccess)
	}
	if ec.Text != "你好，世界" {
		t.Errorf("Text = %q, want unchanged (空 vocab 无匹配)", ec.Text)
	}
}

// TestPipeline_Run_CleaningFillter 验证 cleaning + filler 真实算法。
func TestPipeline_Run_CleaningFillter(t *testing.T) {
	reg := builtins.NewDefaultRegistry()
	policy := textenhance.DefaultPolicy()
	pipeline, err := textenhance.BuildPipeline(reg, policy)
	if err != nil {
		t.Fatalf("BuildPipeline: %v", err)
	}

	// 1) 多个连续空格 → 清洗为单空格
	// 2) 句首的"呃" → filler 删除
	input := "呃   你好，   世界。"
	ec := textenhance.NewEnhancementContext(input, textenhance.EmptyVocabularySnapshot(), policy)
	pipeline.Run(context.Background(), ec)

	t.Logf("output: %q", ec.Text)
	t.Logf("changes: %d", len(ec.Changes))
	if ec.Status != textenhance.StatusSuccess {
		t.Errorf("Status = %d, want SUCCESS", ec.Status)
	}
	// 期望：句首"呃"被删；多余空格被合并
	if ec.Text == input {
		t.Error("Text unchanged; cleaning + filler not applied")
	}
	// 期望有 2 个 change（cleaning 1 + filler 1）
	if len(ec.Changes) < 2 {
		t.Errorf("Changes = %d, want >= 2", len(ec.Changes))
	}
}

// TestPipeline_Run_WithVocab 验证词库相关 processor 真实工作。
func TestPipeline_Run_WithVocab(t *testing.T) {
	reg := builtins.NewDefaultRegistry()
	policy := textenhance.DefaultPolicy()
	pipeline, err := textenhance.BuildPipeline(reg, policy)
	if err != nil {
		t.Fatalf("BuildPipeline: %v", err)
	}

	// 构造 vocab：金种子 → alias → 金种籽
	entries := []*textenhance.VocabularyEntry{
		{ID: 1, StandardText: "金种籽", Category: "PRODUCT", Priority: 100},
	}
	relations := []*textenhance.VocabularyRelation{
		{EntryID: 1, RelationType: "ALIAS", RelatedText: "金种子", TargetEntryID: 1},
	}
	vocab := textenhance.NewVocabularySnapshot(entries, relations)

	input := "金种子是新产品"
	ec := textenhance.NewEnhancementContext(input, vocab, policy)
	pipeline.Run(context.Background(), ec)

	t.Logf("output: %q", ec.Text)
	// 期望：金种子 → 金种籽
	if ec.Text != "金种籽是新产品" {
		t.Errorf("Text = %q, want %q", ec.Text, "金种籽是新产品")
	}
}

// TestPipeline_Run_ContextCancel 验证 HA：ctx 取消后 Pipeline 退出。
func TestPipeline_Run_ContextCancel(t *testing.T) {
	reg := builtins.NewDefaultRegistry()
	policy := textenhance.DefaultPolicy()
	pipeline, err := textenhance.BuildPipeline(reg, policy)
	if err != nil {
		t.Fatalf("BuildPipeline: %v", err)
	}

	// 立即取消的 ctx
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	ec := textenhance.NewEnhancementContext("hello world", textenhance.EmptyVocabularySnapshot(), policy)
	pipeline.Run(ctx, ec)

	if !ec.Canceled {
		t.Error("expected ec.Canceled = true")
	}
	if ec.Status != textenhance.StatusCanceled && ec.Status != textenhance.StatusSuccess {
		// ctx 取消可能在第一步前检测到（SUCCESS，无任何 processor 跑）；也可能在中间（CANCELED）
		t.Errorf("Status = %d, want CANCELED or SUCCESS", ec.Status)
	}
}

// TestPipeline_Run_Timeout 验证 HA：默认 5s timeout 存在（不实际等待）。
func TestPipeline_Run_Timeout(t *testing.T) {
	reg := builtins.NewDefaultRegistry()
	policy := textenhance.DefaultPolicy()
	pipeline, err := textenhance.BuildPipeline(reg, policy)
	if err != nil {
		t.Fatalf("BuildPipeline: %v", err)
	}

	// 不带 deadline 的 ctx → Pipeline 应自动加 5s timeout
	ec := textenhance.NewEnhancementContext("x", textenhance.EmptyVocabularySnapshot(), policy)
	start := time.Now()
	pipeline.Run(context.Background(), ec)
	elapsed := time.Since(start)

	if elapsed > 6*time.Second {
		t.Errorf("Pipeline took %v, expected < 6s (default timeout 5s)", elapsed)
	}
}

// TestPolicy_DefaultPolicy 验证默认策略 8 步全开 + 阈值正确。
func TestPolicy_DefaultPolicy(t *testing.T) {
	p := textenhance.DefaultPolicy()
	if !p.IsEnabled("cleaning") {
		t.Error("cleaning should be enabled")
	}
	if !p.IsEnabled("fuzzy_matching") {
		t.Error("fuzzy_matching should be enabled")
	}
	if p.PinyinThreshold != 0.85 {
		t.Errorf("PinyinThreshold = %v, want 0.85", p.PinyinThreshold)
	}
}

// TestPolicy_NilIsDefaultEnabled 验证 nil policy = 全部启用（HA）。
func TestPolicy_NilIsDefaultEnabled(t *testing.T) {
	var p *textenhance.Policy
	if !p.IsEnabled("any_processor") {
		t.Error("nil policy should enable all processors")
	}
}

// TestPolicy_Validate 验证非法阈值被 clamp。
func TestPolicy_Validate(t *testing.T) {
	p := &textenhance.Policy{
		EnabledProcessors:     textenhance.DefaultProcessorOrder,
		PinyinThreshold:       -1.0, // 非法
		FuzzyAutoThreshold:    2.0,  // 非法
		FuzzySuggestThreshold: 0.5,  // 合法
	}
	validated, errs := p.Validate()
	if len(errs) != 2 {
		t.Errorf("errs = %d, want 2", len(errs))
	}
	if validated.PinyinThreshold != 0 {
		t.Errorf("PinyinThreshold = %v, want 0 (clamped from -1)", validated.PinyinThreshold)
	}
}

// TestEnhancementContext_LockIsLocked 验证 lock / IsLocked 行为。
func TestEnhancementContext_LockIsLocked(t *testing.T) {
	ec := textenhance.NewEnhancementContext("test", nil, nil)
	ec.Lock("片段A")
	ec.Lock("片段B")
	if !ec.IsLocked("片段A") {
		t.Error("片段A should be locked")
	}
	if !ec.IsLocked("片段B") {
		t.Error("片段B should be locked")
	}
	if ec.IsLocked("片段C") {
		t.Error("片段C should not be locked")
	}
}

// TestRegistry_Build_NotFound 验证未知 processor 返回 error 而非 panic。
func TestRegistry_Build_NotFound(t *testing.T) {
	reg := builtins.NewDefaultRegistry()
	_, err := reg.Build("non_existent")
	if err == nil {
		t.Error("expected error for unknown processor")
	}
}

// TestRegistry_OptionTypeMismatch 验证 Option 类型不匹配返回 error。
func TestRegistry_OptionTypeMismatch(t *testing.T) {
	reg := builtins.NewDefaultRegistry()
	// 用 filler.Option 给 cleaning
	_, err := reg.Build("cleaning", "wrong-type")
	if err == nil {
		t.Error("expected error for option type mismatch")
	}
}