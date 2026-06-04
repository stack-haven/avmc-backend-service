package convert

import (
	"testing"
	"time"
)

func TestTimeValueToString(t *testing.T) {
	t.Parallel()

	if got := TimeValueToString(nil, time.DateOnly); got != nil {
		t.Fatalf("TimeValueToString(nil) = %q, want nil", *got)
	}

	zero := time.Time{}
	if got := TimeValueToString(&zero, time.DateOnly); got != nil {
		t.Fatalf("TimeValueToString(zero) = %q, want nil", *got)
	}

	value := time.Date(2026, time.June, 4, 12, 30, 0, 0, time.UTC)
	got := TimeValueToString(&value, time.DateTime)
	if got == nil || *got != "2026-06-04 12:30:00" {
		t.Fatalf("TimeValueToString(value) = %v, want 2026-06-04 12:30:00", got)
	}
}

func TestStringValueToTime(t *testing.T) {
	t.Parallel()

	if got := StringValueToTime(nil, time.DateOnly); got != nil {
		t.Fatalf("StringValueToTime(nil) = %v, want nil", got)
	}

	invalid := "not-a-date"
	if got := StringValueToTime(&invalid, time.DateOnly); got != nil {
		t.Fatalf("StringValueToTime(invalid) = %v, want nil", got)
	}

	value := "2026-06-04"
	got := StringValueToTime(&value, time.DateOnly)
	want := time.Date(2026, time.June, 4, 0, 0, 0, 0, time.UTC)
	if got == nil || !got.Equal(want) {
		t.Fatalf("StringValueToTime(value) = %v, want %v", got, want)
	}
}
