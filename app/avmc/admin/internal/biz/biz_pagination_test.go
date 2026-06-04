package biz

import "testing"

func TestNormalizePageSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input int32
		want  int
	}{
		{input: -1, want: DefaultPageSize},
		{input: 0, want: DefaultPageSize},
		{input: 50, want: 50},
		{input: 1000, want: MaxPageSize},
	}
	for _, tt := range tests {
		if got := NormalizePageSize(tt.input); got != tt.want {
			t.Fatalf("NormalizePageSize(%d) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

func TestListOptionsClampUnsafeValues(t *testing.T) {
	t.Parallel()

	opts := ListOptions{}
	ListLimit(1000)(&opts)
	ListOffset(-10)(&opts)
	if opts.Limit != MaxPageSize || opts.Offset != 0 {
		t.Fatalf("options = %#v", opts)
	}
}
