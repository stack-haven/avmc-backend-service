package utils

import (
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
)

func TestDuration(t *testing.T) {
	fallback := 2 * time.Second
	if got := Duration(nil, fallback); got != fallback {
		t.Fatalf("nil duration = %v, want %v", got, fallback)
	}
	if got := Duration(durationpb.New(0), fallback); got != fallback {
		t.Fatalf("zero duration = %v, want %v", got, fallback)
	}
	if got := Duration(durationpb.New(750*time.Millisecond), fallback); got != 750*time.Millisecond {
		t.Fatalf("explicit duration = %v", got)
	}
}
