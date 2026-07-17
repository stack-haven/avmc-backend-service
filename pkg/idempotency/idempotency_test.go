package idempotency

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestManagerExecuteReplaysCompletedResult(t *testing.T) {
	store := NewMemoryStore()
	manager := NewManager(store)
	ctx := context.Background()
	req := Request{Scope: "tenant:1", Key: "file:create:1", Fingerprint: "sha256:a", TTL: time.Hour}

	first, err := manager.Execute(ctx, req, func(context.Context) ([]byte, error) {
		return []byte(`{"id":1}`), nil
	})
	if err != nil {
		t.Fatalf("Execute() first error = %v", err)
	}
	if first.Replay {
		t.Fatal("first result must not be replay")
	}

	calls := 0
	second, err := manager.Execute(ctx, req, func(context.Context) ([]byte, error) {
		calls++
		return []byte(`{"id":2}`), nil
	})
	if err != nil {
		t.Fatalf("Execute() second error = %v", err)
	}
	if !second.Replay {
		t.Fatal("second result must be replay")
	}
	if calls != 0 {
		t.Fatalf("replay must not invoke handler, calls = %d", calls)
	}
	if got := string(second.Record.Value); got != `{"id":1}` {
		t.Fatalf("replay value = %s, want first value", got)
	}
}

func TestManagerExecuteRejectsFingerprintConflict(t *testing.T) {
	store := NewMemoryStore()
	manager := NewManager(store)
	ctx := context.Background()

	_, err := manager.Execute(ctx, Request{Scope: "tenant:1", Key: "same-key", Fingerprint: "a"}, func(context.Context) ([]byte, error) {
		return []byte("ok"), nil
	})
	if err != nil {
		t.Fatalf("Execute() first error = %v", err)
	}

	_, err = manager.Execute(ctx, Request{Scope: "tenant:1", Key: "same-key", Fingerprint: "b"}, func(context.Context) ([]byte, error) {
		return []byte("unexpected"), nil
	})
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("Execute() conflict error = %v, want ErrConflict", err)
	}
}

func TestManagerExecuteDoesNotStoreFailedOperation(t *testing.T) {
	store := NewMemoryStore()
	manager := NewManager(store)
	ctx := context.Background()
	req := Request{Scope: "tenant:1", Key: "unstable", Fingerprint: "a"}
	failed := errors.New("failed")

	_, err := manager.Execute(ctx, req, func(context.Context) ([]byte, error) {
		return nil, failed
	})
	if !errors.Is(err, failed) {
		t.Fatalf("Execute() error = %v, want handler error", err)
	}

	calls := 0
	result, err := manager.Execute(ctx, req, func(context.Context) ([]byte, error) {
		calls++
		return []byte("ok"), nil
	})
	if err != nil {
		t.Fatalf("Execute() retry error = %v", err)
	}
	if result.Replay {
		t.Fatal("successful retry after failure must not be replay")
	}
	if calls != 1 {
		t.Fatalf("retry handler calls = %d, want 1", calls)
	}
}
