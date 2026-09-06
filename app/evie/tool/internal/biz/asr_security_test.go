package biz

import (
	"path/filepath"
	"testing"
)

func TestSanitizeSessionID(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"", "", true},
		{"abc123", "abc123", true},
		{"a-b_c", "a-b_c", true},
		{"../../etc/passwd", "", false},
		{"id with space", "", false},
		{"id/with/slash", "", false},
		{"中文session", "", false},
	}
	for _, c := range cases {
		got, err := SanitizeSessionID(c.in)
		if (err == nil) != c.ok {
			t.Errorf("SanitizeSessionID(%q) ok=%v, want %v (err=%v)", c.in, err == nil, c.ok, err)
		}
		if got != c.want {
			t.Errorf("SanitizeSessionID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSanitizeTenantID(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"", "", false},
		{"158", "158", true},
		{"t-001", "t-001", true},
		{"../bad", "", false},
		{"id space", "", false},
		{"abs/path", "", false},
	}
	for _, c := range cases {
		got, err := SanitizeTenantID(c.in)
		if (err == nil) != c.ok {
			t.Errorf("SanitizeTenantID(%q) ok=%v, want %v (err=%v)", c.in, err == nil, c.ok, err)
		}
		if got != c.want {
			t.Errorf("SanitizeTenantID(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestIsPathWithin(t *testing.T) {
	tmp := t.TempDir()
	if !IsPathWithin(tmp, filepath.Join(tmp, "a.wav")) {
		t.Error("expected file inside base to be allowed")
	}
	if IsPathWithin(tmp, filepath.Join(tmp, "..", "evil")) {
		t.Error("expected ../evil to be blocked")
	}
	if IsPathWithin(tmp, "/etc/passwd") {
		t.Error("expected absolute outside path to be blocked")
	}
}
