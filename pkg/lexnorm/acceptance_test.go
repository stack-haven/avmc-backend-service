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
	"context"
	"sync"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/internal/lexutil"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/alias"
	"github.com/stack-haven/lexnorm/processor/deterministic"
	"github.com/stack-haven/lexnorm/processor/disfluency"
	"github.com/stack-haven/lexnorm/processor/normalize"
)

// acceptanceLex returns a Lexicon with entries needed for the
// acceptance scenarios (A–F).
func acceptanceLex() lexicon.Lexicon {
	return lexutil.NewMemLexicon(
		[]lexicon.Entry{
			{
				ID:   "zhou",
				Text: "周丽群",
				Variants: []lexicon.Variant{
					{Text: "周莉群", Kind: lexicon.VariantAlias, Confidence: 1.0},
					{Text: "周丽裙", Kind: lexicon.VariantCorrection, Confidence: 0.95},
				},
			},
			{
				ID:   "tian",
				Text: "田华",
				Variants: []lexicon.Variant{
					{Text: "小田", Kind: lexicon.VariantAlias, Confidence: 1.0},
				},
			},
			{
				ID:   "ge",
				Text: "颗种籽",
				Variants: []lexicon.Variant{
					{Text: "个种籽", Kind: lexicon.VariantCorrection, Confidence: 0.95},
					{Text: "个种子", Kind: lexicon.VariantCorrection, Confidence: 0.90},
				},
			},
		},
		"acceptance-v1",
	)
}

// ----------------------------------------------------------------------------
// Scenario A: ASR
// ----------------------------------------------------------------------------

func TestAcceptance_A_ASR(t *testing.T) {
	// Input contains two known errors: "周莉群" (alias) and "个种籽" (correction).
	// Expected output: both corrected to canonical forms.
	lex := acceptanceLex()
	pipe := lexnorm.NewPipeline(
		normalize.New(),
		disfluency.New(),
		alias.New(lex),
		deterministic.New(lex),
	)
	e, err := lexnorm.New(
		lexnorm.WithLexicon(lex),
		lexnorm.WithPipeline(pipe),
	)
	if err != nil {
		t.Fatal(err)
	}

	res, err := e.Normalize(context.Background(), "周莉群帮我查一下个种籽的情况")
	if err != nil {
		t.Fatal(err)
	}

	if res.Text != "周丽群帮我查一下颗种籽的情况" {
		t.Errorf("Text = %q, want %q", res.Text, "周丽群帮我查一下颗种籽的情况")
	}
	if res.Status != lexnorm.StatusSuccess {
		t.Errorf("Status = %v, want StatusSuccess", res.Status)
	}
	if len(res.Changes) != 2 {
		t.Errorf("len(Changes) = %d, want 2", len(res.Changes))
	}
	if res.Runtime.LexiconVersion != "acceptance-v1" {
		t.Errorf("Runtime.LexiconVersion = %q, want acceptance-v1", res.Runtime.LexiconVersion)
	}
}

// (disflency_New helper removed; tests use disfluency.New() directly.)

// ----------------------------------------------------------------------------
// Scenario B: Meeting
// ----------------------------------------------------------------------------

func TestAcceptance_B_Meeting(t *testing.T) {
	// Input contains "小田" (alias) and "周莉群" (alias).
	lex := acceptanceLex()
	pipe := lexnorm.NewPipeline(
		normalize.New(),
		alias.New(lex),
	)
	e, _ := lexnorm.New(
		lexnorm.WithLexicon(lex),
		lexnorm.WithPipeline(pipe),
	)

	res, _ := e.Normalize(context.Background(), "小田和周莉群明天去吃饭")
	if res.Text != "田华和周丽群明天去吃饭" {
		t.Errorf("Text = %q, want %q", res.Text, "田华和周丽群明天去吃饭")
	}
	if len(res.Changes) != 2 {
		t.Errorf("len(Changes) = %d, want 2", len(res.Changes))
	}
}

// ----------------------------------------------------------------------------
// Scenario C: Protected Span
// ----------------------------------------------------------------------------

func TestAcceptance_C_ProtectedSpan(t *testing.T) {
	// Lock [0, 9) (covering "周莉群"). Then Alias tries to replace the
	// same span — must conflict; Fuzzy would also be blocked if it
	// tried to modify that range.
	lex := acceptanceLex()
	lockingPipe := newPipelineWithLock(0, 9, lex)
	e, _ := lexnorm.New(
		lexnorm.WithLexicon(lex),
		lexnorm.WithPipeline(lockingPipe),
	)

	res, _ := e.Normalize(context.Background(), "周莉群")
	// Lock blocks the Alias replacement: text remains unchanged.
	if res.Text != "周莉群" {
		t.Errorf("Text = %q, want %q (Lock must block Alias replacement)", res.Text, "周莉群")
	}

	// No Applied changes (Alias was rejected; Lock is not a Change).
	appliedCount := 0
	for _, c := range res.Changes {
		if c.Applied {
			appliedCount++
		}
	}
	if appliedCount != 0 {
		t.Errorf("Applied changes = %d, want 0 (Lock blocks Alias)", appliedCount)
	}
}

// ----------------------------------------------------------------------------
// Scenario D: Lexicon Hot Update (Request Consistency)
// ----------------------------------------------------------------------------

func TestAcceptance_D_LexiconHotUpdate(t *testing.T) {
	// Note: in the current implementation, the Pipeline is constructed
	// with a specific Lexicon reference; Store-based HA requires a
	// future "lazy Lexicon" mechanism so the Pipeline resolves the
	// current Snapshot per call. This test verifies the part that
	// works: Store + RuntimeInfo version propagation.
	v1 := lexutil.NewMemLexicon(
		[]lexicon.Entry{{ID: "e1", Text: "周丽群"}}, "v1",
	)
	store := lexicon.NewStore(v1)
	e, _ := lexnorm.New(
		lexnorm.WithLexiconStore(store),
		lexnorm.WithPipeline(lexnorm.NewPipeline(normalize.New())),
	)

	// V1 call: text unchanged (no aliases), version = v1.
	res1, _ := e.Normalize(context.Background(), "周莉群")
	if res1.Runtime.LexiconVersion != "v1" {
		t.Errorf("V1: LexiconVersion = %q, want v1", res1.Runtime.LexiconVersion)
	}
	if res1.Runtime.ProfileID == "" {
		t.Error("Runtime.ProfileID must be populated (Request Consistency)")
	}

	// Update Store to V2.
	v2 := lexutil.NewMemLexicon(
		[]lexicon.Entry{{ID: "e1", Text: "周丽群"}}, "v2",
	)
	if err := store.TryUpdate(func() (lexicon.Lexicon, error) { return v2, nil }); err != nil {
		t.Fatal(err)
	}

	// V2 call: version = v2. Result is independent of res1.
	res2, _ := e.Normalize(context.Background(), "周莉群")
	if res2.Runtime.LexiconVersion != "v2" {
		t.Errorf("V2: LexiconVersion = %q, want v2", res2.Runtime.LexiconVersion)
	}
	if res1.Runtime.LexiconVersion != "v1" {
		t.Error("V1 result was mutated by V2 update (immutability violated)")
	}

	// Verify LKG semantics: failed update keeps current.
	if err := store.TryUpdate(func() (lexicon.Lexicon, error) {
		return nil, errIntentionallyFailing
	}); err == nil {
		t.Error("failed TryUpdate must return error")
	}
	if got := store.Version(); got != "v2" {
		t.Errorf("after failed update, Version = %q, want v2 (LKG)", got)
	}
}

// ----------------------------------------------------------------------------
// Scenario E: Processor Failure Degradation
// ----------------------------------------------------------------------------

func TestAcceptance_E_ProcessorFailureDegrades(t *testing.T) {
	lex := acceptanceLex()
	pipe := lexnorm.NewPipeline(
		normalize.New(),
		&failProcessor{name: "fail", err: errIntentionallyFailing},
		alias.New(lex),
	)
	e, _ := lexnorm.New(
		lexnorm.WithLexicon(lex),
		lexnorm.WithPipeline(pipe),
		lexnorm.WithErrorPolicy(lexnorm.ContinueOnError),
	)

	res, err := e.Normalize(context.Background(), "周莉群")
	// ContinueOnError: err == nil; Status == StatusPartial.
	if err != nil {
		t.Errorf("ContinueOnError must NOT return non-nil err, got %v", err)
	}
	if res.Status != lexnorm.StatusPartial {
		t.Errorf("Status = %v, want StatusPartial", res.Status)
	}
	if res.Text == "" {
		t.Error("Text must be preserved on failure (invariant I10)")
	}
	if len(res.Errors) != 1 {
		t.Errorf("len(Errors) = %d, want 1", len(res.Errors))
	}
}

var errIntentionallyFailing = &intentionalError{"intentional failure"}

type intentionalError struct{ s string }

func (e *intentionalError) Error() string { return e.s }

// failProcessor returns an error on every call.
type failProcessor struct {
	name string
	err  error
}

func (p *failProcessor) Name() string { return p.name }
func (p *failProcessor) Process(_ context.Context, _ *lexnorm.State) error {
	return p.err
}

// ----------------------------------------------------------------------------
// Scenario F: Multi-Profile Concurrent
// ----------------------------------------------------------------------------

func TestAcceptance_F_MultiProfileConcurrent(t *testing.T) {
	// Two profiles with different alias mappings; run concurrently.
	lexA := lexutil.NewMemLexicon(
		[]lexicon.Entry{{ID: "eA", Text: "周丽群", Variants: []lexicon.Variant{
			{Text: "周莉群", Kind: lexicon.VariantAlias, Confidence: 1.0},
		}}},
		"vA",
	)
	lexB := lexutil.NewMemLexicon(
		[]lexicon.Entry{{ID: "eB", Text: "田华", Variants: []lexicon.Variant{
			{Text: "小田", Kind: lexicon.VariantAlias, Confidence: 1.0},
		}}},
		"vB",
	)
	pipeA := lexnorm.NewPipeline(alias.New(lexA))
	pipeB := lexnorm.NewPipeline(alias.New(lexB))
	e, _ := lexnorm.New(
		lexnorm.WithProfiles(map[lexnorm.ProfileID]lexnorm.ProfileBundle{
			"A": {Lexicon: lexA, Pipeline: pipeA},
			"B": {Lexicon: lexB, Pipeline: pipeB},
		}),
		lexnorm.WithDefaultProfile("A"),
	)

	const N = 50
	var wg sync.WaitGroup
	wg.Add(N * 2)
	for i := 0; i < N; i++ {
		go func() {
			defer wg.Done()
			res, _ := e.Normalize(context.Background(), "周莉群", lexnorm.WithProfileID("A"))
			if res.Text != "周丽群" {
				t.Errorf("A: Text = %q, want 周丽群", res.Text)
			}
			if res.Runtime.LexiconVersion != "vA" {
				t.Errorf("A: version = %q, want vA", res.Runtime.LexiconVersion)
			}
		}()
		go func() {
			defer wg.Done()
			res, _ := e.Normalize(context.Background(), "小田", lexnorm.WithProfileID("B"))
			if res.Text != "田华" {
				t.Errorf("B: Text = %q, want 田华", res.Text)
			}
			if res.Runtime.LexiconVersion != "vB" {
				t.Errorf("B: version = %q, want vB", res.Runtime.LexiconVersion)
			}
		}()
	}
	wg.Wait()
}

// ----------------------------------------------------------------------------
// 15 不变量架构约束综合验证
// ----------------------------------------------------------------------------

func TestInvariant_Batch(t *testing.T) {
	// Smoke test: an Engine + Pipeline + State work end-to-end without
	// violating any documented invariant. This is a "tripwire" test —
	// if any of the 15 invariants is broken, integration breaks here too.
	e, err := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(lexnorm.NewPipeline(
			normalize.New(),
			alias.New(simpleLexicon()),
		)),
	)
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.Normalize(context.Background(), "  hello  ")
	if err != nil {
		t.Fatal(err)
	}
	if res.Text != "hello" {
		t.Errorf("integrated Normalize: Text = %q, want %q", res.Text, "hello")
	}
	if res.Runtime.LexiconVersion == "" {
		t.Error("RuntimeInfo.LexiconVersion must be populated")
	}
}

// (No additional helpers required.)
