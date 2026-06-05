package listing

import (
	"testing"

	pbCore "backend-service/api/core/service/v1"

	"go.einride.tech/aip/filtering"
)

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

func TestOptionsClampUnsafeValues(t *testing.T) {
	t.Parallel()

	opts := Options{}
	LimitOption(1000)(&opts)
	OffsetOption(-10)(&opts)
	if opts.Limit != MaxPageSize || opts.Offset != 0 {
		t.Fatalf("options = %#v", opts)
	}
}

func TestParseParamsNormalizesAndParses(t *testing.T) {
	req := &pbCore.ListUsersRequest{
		PageSize:  1000,
		PageToken: "",
		Filter:    ptr(`name = "mock_admin"`),
		OrderBy:   ptr("name desc"),
	}
	params, err := ParseParams(req, filtering.DeclareIdent("name", filtering.TypeString))
	if err != nil {
		t.Fatalf("ParseParams() error = %v", err)
	}
	if params.PageSize != MaxPageSize {
		t.Fatalf("PageSize = %d, want %d", params.PageSize, MaxPageSize)
	}
	if params.Filter.CheckedExpr == nil {
		t.Fatal("filter was not parsed")
	}
	if len(params.OrderBy.Fields) != 1 || params.OrderBy.Fields[0].Path != "name" {
		t.Fatalf("OrderBy = %#v", params.OrderBy)
	}
}

func TestParseParamsRejectsUnknownFilterField(t *testing.T) {
	req := &pbCore.ListUsersRequest{Filter: ptr(`email = "mock@example.com"`)}
	if _, err := ParseParams(req, filtering.DeclareIdent("name", filtering.TypeString)); err == nil {
		t.Fatal("ParseParams() error = nil")
	}
}

func ptr[T any](v T) *T {
	return &v
}
