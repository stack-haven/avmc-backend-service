package errs

import (
	"errors"
	"fmt"
	"testing"
)

func TestNewAndIs(t *testing.T) {
	cause := errors.New("root cause")
	e := New(42, "something failed", cause)

	if !Is(e) {
		t.Fatal("expected Is to recognize unified error")
	}
	if !errors.Is(e, cause) {
		t.Fatal("expected errors.Is to unwrap to cause")
	}
	if msg := e.Error(); msg == "" {
		t.Fatal("empty error message")
	}
}

func TestGetCode(t *testing.T) {
	e := New(7, "test", nil)
	code, ok := GetCode(e)
	if !ok {
		t.Fatal("expected GetCode to succeed")
	}
	if code != 7 {
		t.Fatalf("code = %d, want 7", code)
	}

	// 包装后仍能提取
	wrapped := fmt.Errorf("wrapped: %w", e)
	code, ok = GetCode(wrapped)
	if !ok || code != 7 {
		t.Fatalf("wrapped code = %d (ok=%v), want 7", code, ok)
	}

	// 普通错误
	if _, ok := GetCode(errors.New("plain")); ok {
		t.Fatal("expected GetCode to fail for plain error")
	}
}
