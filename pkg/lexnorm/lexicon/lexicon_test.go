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

package lexicon_test

import (
	"testing"

	"github.com/stack-haven/lexnorm/lexicon"
)

func TestEntryID_String(t *testing.T) {
	if got := (lexicon.EntryID("e-001")).String(); got != "e-001" {
		t.Errorf("EntryID.String() = %q, want %q", got, "e-001")
	}
}

func TestEntryID_IsZero(t *testing.T) {
	if !(lexicon.EntryID("")).IsZero() {
		t.Error("empty EntryID must be zero")
	}
	if (lexicon.EntryID("x")).IsZero() {
		t.Error("non-empty EntryID must NOT be zero")
	}
}

func TestVariantKind_String(t *testing.T) {
	tests := []struct {
		k    lexicon.VariantKind
		want string
	}{
		{lexicon.VariantAlias, "alias"},
		{lexicon.VariantCorrection, "correction"},
		{lexicon.VariantHomophone, "homophone"},
		{lexicon.VariantApproximate, "approximate"},
		{lexicon.VariantKind(99), "unknown"},
	}
	for _, tc := range tests {
		if got := tc.k.String(); got != tc.want {
			t.Errorf("VariantKind(%d).String() = %q, want %q", tc.k, got, tc.want)
		}
	}
}

func TestVariant_IsValid(t *testing.T) {
	if (lexicon.Variant{}).IsValid() {
		t.Error("Variant with empty Text must NOT be valid")
	}
	if !(lexicon.Variant{Text: "abc"}).IsValid() {
		t.Error("Variant with Text must be valid")
	}
}

func TestEntry_IsValid(t *testing.T) {
	if (lexicon.Entry{}).IsValid() {
		t.Error("Entry with empty ID must NOT be valid")
	}
	if !(lexicon.Entry{ID: "e-1"}).IsValid() {
		t.Error("Entry with ID must be valid")
	}
}

func TestEntry_Fields(t *testing.T) {
	e := lexicon.Entry{
		ID:   "e-001",
		Text: "canonical",
		Variants: []lexicon.Variant{
			{Text: "alt1", Kind: lexicon.VariantAlias, Confidence: 1.0},
			{Text: "alt2", Kind: lexicon.VariantCorrection, Confidence: 0.9},
		},
		Meta: map[string]any{"category": "name"},
	}
	if e.ID != "e-001" {
		t.Error("ID mismatch")
	}
	if e.Text != "canonical" {
		t.Error("Text mismatch")
	}
	if len(e.Variants) != 2 {
		t.Errorf("Variants len = %d, want 2", len(e.Variants))
	}
	if e.Meta["category"] != "name" {
		t.Error("Meta mismatch")
	}
}

func TestRelation_Fields(t *testing.T) {
	r := lexicon.Relation{
		From:   "e-001",
		To:     "e-002",
		Kind:   lexicon.VariantAlias,
		Weight: 0.8,
	}
	if r.From != "e-001" || r.To != "e-002" {
		t.Error("From/To mismatch")
	}
	if r.Kind != lexicon.VariantAlias {
		t.Error("Kind mismatch")
	}
	if r.Weight != 0.8 {
		t.Error("Weight mismatch")
	}
}
