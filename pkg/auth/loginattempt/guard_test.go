package loginattempt

import (
	"strings"
	"testing"
	"time"
)

func TestRedisGuardKeysDoNotExposeIdentity(t *testing.T) {
	t.Parallel()

	guard := NewRedisGuard(nil, Options{})
	keyA := guard.attemptKey("username", " Admin@Example.com ", 7)
	keyB := guard.attemptKey("username", "admin@example.com", 7)

	if keyA != keyB {
		t.Fatalf("normalized identities produced different keys: %q != %q", keyA, keyB)
	}
	if strings.Contains(keyA, "admin@example.com") {
		t.Fatalf("key exposes login identity: %q", keyA)
	}
	if !strings.HasPrefix(keyA, defaultPrefix+":attempt:username:7:") {
		t.Fatalf("unexpected key format: %q", keyA)
	}
}

func TestOptionsFromEnv(t *testing.T) {
	t.Setenv("TEST_LOGIN_MAX_ATTEMPTS", "7")
	t.Setenv("TEST_LOGIN_WINDOW", "20m")
	t.Setenv("TEST_LOGIN_LOCKOUT", "30m")

	opts, err := OptionsFromEnv("TEST_LOGIN")
	if err != nil {
		t.Fatalf("OptionsFromEnv() error = %v", err)
	}
	if opts.MaxAttempts != 7 || opts.Window != 20*time.Minute || opts.Lockout != 30*time.Minute {
		t.Fatalf("OptionsFromEnv() = %#v", opts)
	}
}

func TestOptionsFromEnvRejectsUnsafeValues(t *testing.T) {
	t.Setenv("TEST_LOGIN_MAX_ATTEMPTS", "100")
	if _, err := OptionsFromEnv("TEST_LOGIN"); err == nil {
		t.Fatal("OptionsFromEnv() error = nil")
	}
}

func TestNewRedisGuardAppliesSafeDefaults(t *testing.T) {
	t.Parallel()

	guard := NewRedisGuard(nil, Options{})
	if guard.maxAttempts != defaultMaxAttempts || guard.window != defaultWindow || guard.lockout != defaultLockout {
		t.Fatalf("unexpected defaults: %#v", guard)
	}
}
