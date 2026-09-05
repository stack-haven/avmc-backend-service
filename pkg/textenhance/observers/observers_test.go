package observers_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"backend-service/pkg/textenhance/observers"
	"backend-service/pkg/textenhance/processors"
)

// ============================================================================
// Logger 适配器测试
// ============================================================================

type bufferLogger struct {
	mu   sync.Mutex
	logs []string
}

func (l *bufferLogger) WithContext(_ context.Context) observers.Logger { return l }
func (l *bufferLogger) Debugf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, "D:"+fmt.Sprintf(format, args...))
}
func (l *bufferLogger) Infof(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, "I:"+fmt.Sprintf(format, args...))
}
func (l *bufferLogger) Warnf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, "W:"+fmt.Sprintf(format, args...))
}
func (l *bufferLogger) Errorf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = append(l.logs, "E:"+fmt.Sprintf(format, args...))
}

func (l *bufferLogger) HasPrefix(prefix string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, s := range l.logs {
		if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// ============================================================================
// LoggingObserver 测试
// ============================================================================

func TestLoggingObserver_AllEvents(t *testing.T) {
	bl := &bufferLogger{}
	obs := observers.NewLoggingObserver(bl)

	ctx := context.Background()
	obs.OnPipelineStart(ctx, []string{"a", "b"})
	obs.OnPipelineComplete(ctx, processors.EnhancementSnapshot{
		OriginalText: "old", EnhancedText: "new", Status: processors.StatusSuccess,
	})
	obs.OnPipelineError(ctx, errors.New("boom"))
	obs.OnProcessorStart(ctx, "cleaning")
	obs.OnProcessorComplete(ctx, "cleaning", 5*time.Millisecond, nil)
	obs.OnProcessorError(ctx, "cleaning", errors.New("oops"))

	// 调试输出
	t.Logf("all logs:")
	for _, s := range bl.logs {
		t.Logf("  [%s]", s)
	}

	if !bl.HasPrefix("D:textenhance: pipeline start") {
		t.Error("missing pipeline start log")
	}
	if !bl.HasPrefix("D:textenhance: pipeline complete") {
		t.Error("missing pipeline complete log")
	}
	if !bl.HasPrefix("W:textenhance: pipeline error") {
		t.Error("missing pipeline error warn log")
	}
	if !bl.HasPrefix("D:textenhance:   step start: cleaning") {
		t.Error("missing step start log")
	}
	if !bl.HasPrefix("D:textenhance:   step done:  cleaning") {
		t.Error("missing step complete log")
	}
	if !bl.HasPrefix("W:textenhance:   step error: cleaning") {
		t.Error("missing step error warn log")
	}
}

func TestLoggingObserver_DiscardFallback(t *testing.T) {
	obs := observers.NewLoggingObserver(nil) // nil logger
	// 不应该 panic
	obs.OnPipelineStart(context.Background(), []string{"x"})
	obs.OnProcessorError(context.Background(), "y", errors.New("z"))
}

// ============================================================================
// CountingObserver 测试
// ============================================================================

func TestCountingObserver_Stats(t *testing.T) {
	obs := observers.NewCountingObserver()
	ctx := context.Background()

	obs.OnPipelineStart(ctx, []string{"a"})
	obs.OnProcessorStart(ctx, "a")
	obs.OnProcessorComplete(ctx, "a", 10*time.Millisecond, nil)
	obs.OnProcessorStart(ctx, "a")
	obs.OnProcessorComplete(ctx, "a", 20*time.Millisecond, nil)
	obs.OnPipelineComplete(ctx, processors.EnhancementSnapshot{
		Status: processors.StatusSuccess, Duration: 30 * time.Millisecond,
	})

	stats := obs.Stats()
	aStats, ok := stats["a"]
	if !ok {
		t.Fatal("expected stats for 'a'")
	}
	if aStats.Invocations != 2 {
		t.Errorf("Invocations = %d, want 2", aStats.Invocations)
	}
	if aStats.Successes != 2 {
		t.Errorf("Successes = %d, want 2", aStats.Successes)
	}
	if aStats.TotalTime != 30*time.Millisecond {
		t.Errorf("TotalTime = %v, want 30ms", aStats.TotalTime)
	}
	if aStats.MaxTime != 20*time.Millisecond {
		t.Errorf("MaxTime = %v, want 20ms", aStats.MaxTime)
	}

	ps := obs.PipelineStatsSnapshot()
	if ps.Invocations != 1 {
		t.Errorf("Pipeline invocations = %d, want 1", ps.Invocations)
	}
	if ps.Successes != 1 {
		t.Errorf("Pipeline successes = %d, want 1", ps.Successes)
	}
}

func TestCountingObserver_AvgTime(t *testing.T) {
	obs := observers.NewCountingObserver()
	ctx := context.Background()

	obs.OnProcessorStart(ctx, "x")
	obs.OnProcessorComplete(ctx, "x", 10*time.Millisecond, nil)
	obs.OnProcessorStart(ctx, "x")
	obs.OnProcessorComplete(ctx, "x", 30*time.Millisecond, nil)

	stats := obs.Stats()
	xStats := stats["x"]
	if avg := xStats.AvgTime(); avg != 20*time.Millisecond {
		t.Errorf("AvgTime = %v, want 20ms", avg)
	}
}

func TestCountingObserver_Concurrent(t *testing.T) {
	obs := observers.NewCountingObserver()
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			obs.OnProcessorStart(ctx, "concurrent")
			obs.OnProcessorComplete(ctx, "concurrent", time.Microsecond, nil)
		}()
	}
	wg.Wait()
	stats := obs.Stats()
	if stats["concurrent"].Invocations != 100 {
		t.Errorf("concurrent invocations = %d, want 100", stats["concurrent"].Invocations)
	}
}

func TestCountingObserver_ProcessorError(t *testing.T) {
	obs := observers.NewCountingObserver()
	ctx := context.Background()
	obs.OnProcessorStart(ctx, "err_proc")
	obs.OnProcessorError(ctx, "err_proc", errors.New("boom"))
	stats := obs.Stats()
	if stats["err_proc"].Errors != 1 {
		t.Errorf("Errors = %d, want 1", stats["err_proc"].Errors)
	}
}

// ============================================================================
// DiscardLogger 测试
// ============================================================================

func TestDiscardLogger_NoPanic(t *testing.T) {
	var l observers.Logger = observers.DiscardLogger{}
	ctx := context.Background()
	l.WithContext(ctx).Debugf("test")
	l.WithContext(ctx).Errorf("test")
}

// ============================================================================
// StdLogger 测试
// ============================================================================

func TestStdLogger_DelegatesToFuncs(t *testing.T) {
	called := false
	sl := observers.StdLogger{
		Debug: func(format string, args ...any) { called = true },
	}
	sl.Debugf("test %s", "x")
	if !called {
		t.Error("StdLogger.Debug not called")
	}
}