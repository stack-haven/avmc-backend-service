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

package llm_test

import (
	"context"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/llm"
)

// TestPlaceholder verifies the M11 placeholder package compiles and loads,
// and that its Processor identity / capability metadata are sound.
//
// M11 implementation is deferred; this test exists to document the
// package and prevent accidental deletion of the placeholder.
func TestPlaceholder(t *testing.T) {
	t.Log("M11 LLM Processor is a placeholder; implementation deferred")
}

func TestProcessor_Identity(t *testing.T) {
	p := llm.New()
	if p.Name() != "llm" {
		t.Errorf("Name = %q, want llm", p.Name())
	}
	if p.Version() != "v0" {
		t.Errorf("Version = %q, want v0", p.Version())
	}
	if p.Certainty() != lexnorm.CertaintyLow {
		t.Errorf("Certainty = %v, want low", p.Certainty())
	}
}

func TestProcessor_ProcessIsNoOp(t *testing.T) {
	lex, err := lexicon.NewBuilder().Build()
	if err != nil {
		t.Fatal(err)
	}
	st, err := lexnorm.NewState(context.Background(), "原文", lex, lexnorm.DefaultConfig())
	if err != nil {
		t.Fatal(err)
	}
	if err := llm.New().Process(context.Background(), st); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if st.Text() != "原文" || len(st.Changes()) != 0 {
		t.Fatalf("placeholder must be a no-op, text = %q", st.Text())
	}
}

func TestProcessor_Descriptor(t *testing.T) {
	d := llm.New().Descriptor()
	if d.Category != lexnorm.CategorySemantic {
		t.Errorf("Category = %q, want semantic", d.Category)
	}
	if d.DefaultOrder != 8 {
		t.Errorf("DefaultOrder = %d, want 8", d.DefaultOrder)
	}
	if d.Deterministic || d.Determinism != lexnorm.DeterministicGenerative {
		t.Errorf("determinism flags wrong: %+v", d)
	}
	if d.MutatesText || d.SupportsSuggest {
		t.Errorf("placeholder must not declare mutation: %+v", d)
	}
}
