package processors

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

// recordingObserver 记录所有事件（用于测试）。
type recordingObserver struct {
	pipelineStarts   int32
	pipelineCompletes int32
	pipelineErrors   int32
	processorStarts  map[string]int
	processorCompletes map[string]int
	processorErrors  map[string]int
	completedDurations map[string]time.Duration
	mu                chan struct{} // 用 channel 当 mutex（避免 atomic 复杂度）
}

func newRecordingObserver() *recordingObserver {
	return &recordingObserver{
		processorStarts:     make(map[string]int),
		processorCompletes: make(map[string]int),
		processorErrors:     make(map[string]int),
		completedDurations:  make(map[string]time.Duration),
		mu:                  make(chan struct{}, 1),
	}
}

func (r *recordingObserver) lock()   { r.mu <- struct{}{} }
func (r *recordingObserver) unlock() { <-r.mu }

func (r *recordingObserver) OnPipelineStart(ctx context.Context, names []string) {
	r.lock()
	defer r.unlock()
	atomic.AddInt32(&r.pipelineStarts, 1)
}

func (r *recordingObserver) OnPipelineComplete(ctx context.Context, snap EnhancementSnapshot) {
	r.lock()
	defer r.unlock()
	atomic.AddInt32(&r.pipelineCompletes, 1)
}

func (r *recordingObserver) OnPipelineError(ctx context.Context, err error) {
	r.lock()
	defer r.unlock()
	atomic.AddInt32(&r.pipelineErrors, 1)
}

func (r *recordingObserver) OnProcessorStart(ctx context.Context, name string) {
	r.lock()
	defer r.unlock()
	r.processorStarts[name]++
}

func (r *recordingObserver) OnProcessorComplete(ctx context.Context, name string, dur time.Duration, changes []Change) {
	r.lock()
	defer r.unlock()
	r.processorCompletes[name]++
	r.completedDurations[name] = dur
}

func (r *recordingObserver) OnProcessorError(ctx context.Context, name string, err error) {
	r.lock()
	defer r.unlock()
	r.processorErrors[name]++
}

// ============================================================================
// safeNotify 测试
// ============================================================================

func TestSafeNotify_NormalCall(t *testing.T) {
	called := false
	safeNotify(func() { called = true }, "test", "")
	if !called {
		t.Error("safeNotify should call function")
	}
}

func TestSafeNotify_PanicRecovered(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Error("safeNotify should NOT propagate panic")
		}
	}()
	safeNotify(func() { panic("boom") }, "test", "")
	// 没崩 = pass
}

// ============================================================================
// ObservingProcessor 测试
// ============================================================================

// fakeProcessor 始终按指定方式工作。
type fakeProcessor struct {
	name    string
	panics  bool
	textOut string // Process 后 text 改成的值
	changes []Change
}

func (f *fakeProcessor) Name() string { return f.name }
func (f *fakeProcessor) Process(ctx context.Context, ec *EnhancementContext) {
	if f.panics {
		panic("fake panic")
	}
	if f.textOut != "" {
		ec.Text = f.textOut
	}
	for _, ch := range f.changes {
		ec.Changes = append(ec.Changes, ch)
	}
}

func TestObservingProcessor_NormalExecution(t *testing.T) {
	rec := newRecordingObserver()
	inner := &fakeProcessor{name: "test_proc", textOut: "new"}
	obs := NewObservingProcessor(inner, []Observer{rec})

	ec := NewEnhancementContext("old", nil, nil)
	obs.Process(context.Background(), ec)

	if rec.processorStarts["test_proc"] != 1 {
		t.Errorf("processorStarts = %d, want 1", rec.processorStarts["test_proc"])
	}
	if rec.processorCompletes["test_proc"] != 1 {
		t.Errorf("processorCompletes = %d, want 1", rec.processorCompletes["test_proc"])
	}
	if ec.GetText() != "new" {
		t.Errorf("Text = %q, want new", ec.GetText())
	}
}

func TestObservingProcessor_PanicNotSwallowed(t *testing.T) {
	rec := newRecordingObserver()
	inner := &fakeProcessor{name: "panicker", panics: true}
	obs := NewObservingProcessor(inner, []Observer{rec})

	ec := NewEnhancementContext("test", nil, nil)
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic to propagate")
		}
		if rec.processorErrors["panicker"] != 1 {
			t.Errorf("processorErrors = %d, want 1", rec.processorErrors["panicker"])
		}
	}()
	obs.Process(context.Background(), ec)
}

func TestObservingProcessor_ObserverPanicSwallowed(t *testing.T) {
	// observer 自身 panic 不应阻断主流程
	panicObserver := ObserverFunc(func() {
		// noop
	})
	// 包装成会 panic 的 observer
	_ = panicObserver

	inner := &fakeProcessor{name: "ok_proc"}
	badObserver := &badObserverImpl{}

	obs := NewObservingProcessor(inner, []Observer{badObserver})

	ec := NewEnhancementContext("test", nil, nil)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("observer panic should NOT propagate: %v", r)
		}
	}()
	obs.Process(context.Background(), ec)
	// 即使 observer panic，inner 仍应正常执行
	if ec.GetText() != "test" {
		t.Errorf("Text = %q, want test (unchanged)", ec.GetText())
	}
}

type badObserverImpl struct{}

func (b *badObserverImpl) OnPipelineStart(ctx context.Context, names []string)         {}
func (b *badObserverImpl) OnPipelineComplete(ctx context.Context, snap EnhancementSnapshot) {}
func (b *badObserverImpl) OnPipelineError(ctx context.Context, err error)                 {}
func (b *badObserverImpl) OnProcessorStart(ctx context.Context, name string)             { panic("bad observer panic") }
func (b *badObserverImpl) OnProcessorComplete(ctx context.Context, name string, dur time.Duration, changes []Change) {
}
func (b *badObserverImpl) OnProcessorError(ctx context.Context, name string, err error)    {}

// ObserverFunc 适配器（让 func() 实现 Observer）。
type ObserverFunc func()

func (ObserverFunc) OnPipelineStart(context.Context, []string)                              {}
func (ObserverFunc) OnPipelineComplete(context.Context, EnhancementSnapshot)                  {}
func (ObserverFunc) OnPipelineError(context.Context, error)                                   {}
func (ObserverFunc) OnProcessorStart(context.Context, string)                                  {}
func (ObserverFunc) OnProcessorComplete(context.Context, string, time.Duration, []Change)      {}
func (ObserverFunc) OnProcessorError(context.Context, string, error)                             {}

// ============================================================================
// SnapshotOf 测试
// ============================================================================

func TestSnapshotOf_NilEC(t *testing.T) {
	snap := SnapshotOf(nil)
	if snap.OriginalText != "" || snap.Status != 0 {
		t.Errorf("nil snap: %+v", snap)
	}
}

func TestSnapshotOf_CopiesData(t *testing.T) {
	ec := NewEnhancementContext("hello", nil, nil)
	ec.SetText("world")
	ec.Lock("片段X")
	ec.Changes = append(ec.Changes, Change{From: "a", To: "b"})

	snap := SnapshotOf(ec)
	if snap.OriginalText != "hello" {
		t.Errorf("OriginalText = %q, want hello", snap.OriginalText)
	}
	if snap.EnhancedText != "world" {
		t.Errorf("EnhancedText = %q, want world", snap.EnhancedText)
	}
	if len(snap.Changes) != 1 {
		t.Errorf("Changes len = %d, want 1", len(snap.Changes))
	}
}

// 防止 errors 包误删
var _ = errors.New