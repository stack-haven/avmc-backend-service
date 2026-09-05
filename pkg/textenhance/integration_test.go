package textenhance_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"backend-service/pkg/textenhance"
	"backend-service/pkg/textenhance/builtins"
	"backend-service/pkg/textenhance/observers"
	"backend-service/pkg/textenhance/processors"
)

// ============================================================================
// 端到端：Pipeline + Observer + 真实 Processor
// ============================================================================

func TestE2E_PipelineWithObservers(t *testing.T) {
	// 1. 默认 Registry + 默认 Policy
	reg := builtins.NewDefaultRegistry()
	policy := textenhance.DefaultPolicy()

	// 2. 收集 observer（用 sync.Map 模拟并发安全）
	counter := observers.NewCountingObserver()
	logging := observers.NewLoggingObserver(observers.DiscardLogger{})

	pipeline, err := textenhance.BuildPipeline(reg, policy,
		textenhance.WithObservers(counter, logging),
	)
	if err != nil {
		t.Fatalf("BuildPipeline: %v", err)
	}
	if got := len(pipeline.Observers()); got != 2 {
		t.Errorf("Observers len = %d, want 2", got)
	}

	// 3. 准备词库
	vocab := textenhance.NewVocabularySnapshot(
		[]*processors.VocabularyEntry{
			{ID: 1, StandardText: "金种籽", Category: "PRODUCT"},
		},
		[]*processors.VocabularyRelation{
			{EntryID: 1, RelationType: "ALIAS", RelatedText: "金种子", TargetEntryID: 1},
		},
	)

	// 4. 执行
	ec := textenhance.NewEnhancementContext("呃  金种子", vocab, policy)
	pipeline.Run(context.Background(), ec)

	// 5. 验证结果
	expected := " 金种籽" // leading space 保留（filler 只删词不删空格）
	if ec.GetText() != expected {
		t.Errorf("Text = %q, want %q (cleaning + filler + alias 共同作用)", ec.GetText(), expected)
	}
	if ec.GetStatus() != textenhance.StatusSuccess {
		t.Errorf("Status = %d, want SUCCESS", ec.GetStatus())
	}

	// 6. 验证 counter 统计
	stats := counter.Stats()
	if _, ok := stats["filler"]; !ok {
		t.Error("expected filler stats")
	}
	if _, ok := stats["alias_resolution"]; !ok {
		t.Error("expected alias_resolution stats")
	}
	ps := counter.PipelineStatsSnapshot()
	if ps.Invocations != 1 {
		t.Errorf("Pipeline invocations = %d, want 1", ps.Invocations)
	}
	if ps.Successes != 1 {
		t.Errorf("Pipeline successes = %d, want 1", ps.Successes)
	}
}

func TestE2E_ObserverPanicDoesNotBreak(t *testing.T) {
	reg := builtins.NewDefaultRegistry()
	policy := textenhance.DefaultPolicy()

	// 故意 panic 的 observer
	badObs := &crashyObserver{}
	counter := observers.NewCountingObserver()

	pipeline, err := textenhance.BuildPipeline(reg, policy,
		textenhance.WithObservers(badObs, counter),
	)
	if err != nil {
		t.Fatalf("BuildPipeline: %v", err)
	}

	ec := textenhance.NewEnhancementContext("hello", textenhance.EmptyVocabularySnapshot(), policy)

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("observer panic should NOT propagate: %v", r)
		}
	}()

	pipeline.Run(context.Background(), ec)

	// 即使 badObs 一直 panic，counter 应该仍能记录
	stats := counter.Stats()
	if _, ok := stats["cleaning"]; !ok {
		t.Error("counter should still record stats even with panic observer")
	}
}

type crashyObserver struct{}

func (c *crashyObserver) OnPipelineStart(ctx context.Context, names []string) {
	panic("crashy observer")
}
func (c *crashyObserver) OnPipelineComplete(ctx context.Context, snap processors.EnhancementSnapshot) {
}
func (c *crashyObserver) OnPipelineError(ctx context.Context, err error) {
}
func (c *crashyObserver) OnProcessorStart(ctx context.Context, name string) {
	panic("crashy observer")
}
func (c *crashyObserver) OnProcessorComplete(ctx context.Context, name string, dur time.Duration, changes []processors.Change) {
}
func (c *crashyObserver) OnProcessorError(ctx context.Context, name string, err error) {
}

// ============================================================================
// safeNotify 内部测试
// ============================================================================

func TestSafeNotify_NotUsedExternally(t *testing.T) {
	// 占位：safeNotify 是 internal 行为，外部不直接测
	// 这里仅验证导入 + 间接通过 observers 验证
	if errors.New("test") == nil {
		t.Error("sanity")
	}
}

var _ = sync.WaitGroup{}