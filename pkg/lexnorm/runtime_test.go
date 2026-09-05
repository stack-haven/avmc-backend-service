// Copyright 2024 The Ark Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lexnorm_test

import (
	"strings"
	"testing"

	"github.com/stack-haven/lexnorm"
)

func TestRuntimeInfo_IsZero(t *testing.T) {
	if !(lexnorm.RuntimeInfo{}).IsZero() {
		t.Error("zero RuntimeInfo must be zero")
	}
	ri := lexnorm.RuntimeInfo{
		ProfileID:      "default",
		LexiconVersion: "v1",
	}
	if ri.IsZero() {
		t.Error("populated RuntimeInfo must NOT be zero")
	}
}

func TestRuntimeInfo_ProcessorVersion(t *testing.T) {
	ri := lexnorm.RuntimeInfo{
		ProcessorVersions: map[string]string{
			"alias":  "v1",
			"fuzzy":  "v2",
			"noproc": "", // explicit empty
		},
	}

	tests := []struct {
		name string
		want string
	}{
		{"alias", "v1"},
		{"fuzzy", "v2"},
		{"noproc", ""},
		{"missing", ""},
	}
	for _, tc := range tests {
		if got := ri.ProcessorVersion(tc.name); got != tc.want {
			t.Errorf("ProcessorVersion(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestRuntimeInfo_ProcessorVersion_NilMap(t *testing.T) {
	// nil map should be handled without panic.
	var ri lexnorm.RuntimeInfo
	if got := ri.ProcessorVersion("anything"); got != "" {
		t.Errorf("nil map ProcessorVersion = %q, want empty", got)
	}
}

func TestRuntimeInfo_String_Zero(t *testing.T) {
	if got := (lexnorm.RuntimeInfo{}).String(); got != "RuntimeInfo{}" {
		t.Errorf("zero RuntimeInfo.String() = %q, want %q", got, "RuntimeInfo{}")
	}
}

func TestRuntimeInfo_String_Deterministic(t *testing.T) {
	// String output must be deterministic regardless of map iteration order.
	ri1 := lexnorm.RuntimeInfo{
		ProfileID:         "default",
		ProfileVersion:    "v1",
		LexiconVersion:    "lex-v1",
		PipelineVersion:   "pipe-v1",
		ProcessorVersions: map[string]string{"a": "1", "b": "2", "c": "3"},
	}
	ri2 := lexnorm.RuntimeInfo{
		ProfileID:         "default",
		ProfileVersion:    "v1",
		LexiconVersion:    "lex-v1",
		PipelineVersion:   "pipe-v1",
		ProcessorVersions: map[string]string{"c": "3", "a": "1", "b": "2"},
	}
	if ri1.String() != ri2.String() {
		t.Errorf("RuntimeInfo.String() must be deterministic across map orders:\n  ri1=%q\n  ri2=%q",
			ri1.String(), ri2.String())
	}

	// Verify alphabetical ordering of processor list.
	got := ri1.String()
	if !strings.Contains(got, "a@1, b@2, c@3") {
		t.Errorf("expected alphabetical order, got %q", got)
	}
}

func TestRuntimeInfo_String_Format(t *testing.T) {
	ri := lexnorm.RuntimeInfo{
		ProfileID:         "asr",
		ProfileVersion:    "v2",
		LexiconVersion:    "lex-v3",
		PipelineVersion:   "pipe-v4",
		ProcessorVersions: map[string]string{"alias": "v1"},
	}
	got := ri.String()
	want := "Profile{asr@v2} Lexicon@lex-v3 Pipeline@pipe-v4 [alias@v1]"
	if got != want {
		t.Errorf("RuntimeInfo.String() = %q, want %q", got, want)
	}
}

func TestRuntimeInfo_String_NoProcessorVersions(t *testing.T) {
	ri := lexnorm.RuntimeInfo{
		ProfileID:      "default",
		ProfileVersion: "v1",
	}
	got := ri.String()
	if strings.Contains(got, "[") {
		t.Errorf("no processors must produce no [] list, got %q", got)
	}
}
