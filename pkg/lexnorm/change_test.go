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
	"testing"

	"github.com/stack-haven/lexnorm"
)

func TestChangeKind_String(t *testing.T) {
	tests := []struct {
		k    lexnorm.ChangeKind
		want string
	}{
		{lexnorm.ChangeReplace, "replace"},
		{lexnorm.ChangeRemove, "remove"},
		{lexnorm.ChangeSuggest, "suggest"},
		{lexnorm.ChangeKind(99), "ChangeKind(99)"},
	}
	for _, tc := range tests {
		if got := tc.k.String(); got != tc.want {
			t.Errorf("ChangeKind(%d).String() = %q, want %q", tc.k, got, tc.want)
		}
	}
}

func TestChange_IsZero(t *testing.T) {
	if !(lexnorm.Change{}).IsZero() {
		t.Error("zero Change must be zero")
	}
	c := lexnorm.Change{
		Span:    lexnorm.Span{Start: 0, End: 2},
		From:    "ab",
		To:      "cd",
		Applied: true,
	}
	if c.IsZero() {
		t.Error("populated Change must NOT be zero")
	}
}

func TestChange_HasRule(t *testing.T) {
	if (lexnorm.Change{}).HasRule() {
		t.Error("Change with no RuleID must NOT HasRule")
	}
	if !(lexnorm.Change{RuleID: "rule-001"}).HasRule() {
		t.Error("Change with RuleID must HasRule")
	}
}

func TestChange_HasLexiconEntry(t *testing.T) {
	if (lexnorm.Change{}).HasLexiconEntry() {
		t.Error("Change with no EntryID must NOT HasLexiconEntry")
	}
	if !(lexnorm.Change{EntryID: "entry-123"}).HasLexiconEntry() {
		t.Error("Change with EntryID must HasLexiconEntry")
	}
}

func TestChangeMeta_IsZero(t *testing.T) {
	if !(lexnorm.ChangeMeta{}).IsZero() {
		t.Error("zero ChangeMeta must be zero")
	}
	m := lexnorm.ChangeMeta{Source: "auto-mined", Confidence: 0.95}
	if m.IsZero() {
		t.Error("populated ChangeMeta must NOT be zero")
	}
}

func TestChangeMeta_Fields(t *testing.T) {
	m := lexnorm.ChangeMeta{
		Source:     "human-curated",
		Confidence: 0.85,
		RuleID:     "rule-007",
		EntryID:    "entry-42",
		Reason:     "common typo",
	}
	if m.Source != "human-curated" {
		t.Error("Source field must be settable")
	}
	if m.Confidence != 0.85 {
		t.Error("Confidence field must be settable")
	}
	if m.RuleID != "rule-007" {
		t.Error("RuleID field must be settable")
	}
	if m.EntryID != "entry-42" {
		t.Error("EntryID field must be settable")
	}
	if m.Reason != "common typo" {
		t.Error("Reason field must be settable")
	}
}

func TestChange_Fields(t *testing.T) {
	c := lexnorm.Change{
		Span:             lexnorm.Span{Start: 3, End: 6},
		From:             "abc",
		To:               "xyz",
		Action:           lexnorm.ActionReplace,
		Kind:             lexnorm.ChangeReplace,
		Source:           "auto-mined",
		RuleID:           "rule-007",
		EntryID:          "entry-42",
		Processor:        "alias",
		ProcessorVersion: "v2.1.0",
		Confidence:       0.95,
		Applied:          true,
		Reason:           "exact alias match",
	}
	if c.Span != (lexnorm.Span{Start: 3, End: 6}) {
		t.Error("Span field mismatch")
	}
	if c.From != "abc" || c.To != "xyz" {
		t.Error("From/To mismatch")
	}
	if c.Action != lexnorm.ActionReplace {
		t.Error("Action mismatch")
	}
	if c.Kind != lexnorm.ChangeReplace {
		t.Error("Kind mismatch")
	}
	if c.Source != "auto-mined" {
		t.Error("Source mismatch (must be free-form string)")
	}
	if c.Processor != "alias" {
		t.Error("Processor mismatch")
	}
	if c.ProcessorVersion != "v2.1.0" {
		t.Error("ProcessorVersion mismatch (D3 / 1.1+ field)")
	}
	if c.Confidence != 0.95 {
		t.Error("Confidence mismatch")
	}
	if !c.Applied {
		t.Error("Applied must be true")
	}
}
