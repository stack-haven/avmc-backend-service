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
	"errors"
	"fmt"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/internal/lexutil"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/alias"
	"github.com/stack-haven/lexnorm/processor/normalize"
	"github.com/stack-haven/lexnorm/processor/presets"
)

// ExampleNewPipeline demonstrates basic Pipeline construction.
func ExampleNewPipeline() {
	pipe := lexnorm.NewPipeline(
		normalize.New(),
		&echoProcessor{name: "echo"},
	)
	fmt.Println(pipe.Name())
	// Output: pipeline
}

// ExampleNew demonstrates basic Engine construction.
func ExampleNew() {
	lex := lexutil.NewMemLexicon(
		[]lexicon.Entry{
			{ID: "e1", Text: "周丽群", Variants: []lexicon.Variant{
				{Text: "周莉群", Kind: lexicon.VariantAlias, Confidence: 1.0},
			}},
		}, "v1",
	)
	engine, err := lexnorm.New(
		lexnorm.WithLexicon(lex),
		lexnorm.WithPipeline(lexnorm.NewPipeline(
			normalize.New(),
			alias.New(lex),
		)),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(engine != nil)
	// Output: true
}

// ExampleEngine_Normalize demonstrates a complete Normalize call.
func ExampleEngine_Normalize() {
	lex := lexutil.NewMemLexicon(
		[]lexicon.Entry{
			{ID: "e1", Text: "周丽群", Variants: []lexicon.Variant{
				{Text: "周莉群", Kind: lexicon.VariantAlias, Confidence: 1.0},
			}},
		}, "v1",
	)
	engine, _ := lexnorm.New(
		lexnorm.WithLexicon(lex),
		// Two Normalize calls book-end the Pipeline: collapse input
		// whitespace AND collapse output whitespace from Replace.
		lexnorm.WithPipeline(lexnorm.NewPipeline(
			normalize.New(),
			alias.New(lex),
			normalize.New(),
		)),
	)
	res, _ := engine.Normalize(context.Background(), "  周莉群  ")
	fmt.Println(res.Text)
	// Output: 周丽群
}

// Example_presets_Standard demonstrates the Standard preset factory.
func Example_presets_Standard() {
	lex := lexutil.NewMemLexicon(
		[]lexicon.Entry{{ID: "e1", Text: "周丽群"}}, "v1",
	)
	preset := presets.Standard(lex, nil)
	engine, _ := lexnorm.New(lexnorm.WithPreset(*preset))
	res, _ := engine.Normalize(context.Background(), "周莉群")
	fmt.Println(res.Status == lexnorm.StatusSuccess)
	// Output: true
}

// ExampleNewRegistry demonstrates dynamic Processor construction via Registry.
func ExampleNewRegistry() {
	r := lexnorm.NewRegistry()
	r.Register(normalize.Descriptor)

	p, err := r.Build("normalize", nil)
	if err != nil {
		fmt.Println("error:", err)
		return
	}
	fmt.Println(p.Name())
	// Output: normalize
}

// ExampleRecover demonstrates the Recover Middleware.
func ExampleRecover() {
	engine, _ := lexnorm.New(
		lexnorm.WithLexicon(simpleLexicon()),
		lexnorm.WithPipeline(lexnorm.NewPipeline(&examplePanic{})),
		lexnorm.WithMiddleware(lexnorm.Recover()),
	)
	res, _ := engine.Normalize(context.Background(), "x")
	fmt.Println(res.Status == lexnorm.StatusPartial)
	// Output: true
}

// ExampleWithLexiconStore demonstrates HA mode with hot Lexicon updates.
func ExampleWithLexiconStore() {
	lex := lexutil.NewMemLexicon(
		[]lexicon.Entry{{ID: "e1", Text: "周丽群"}}, "v1",
	)
	store := lexicon.NewStore(lex)
	engine, _ := lexnorm.New(
		lexnorm.WithLexiconStore(store),
		lexnorm.WithPipeline(lexnorm.NewPipeline(normalize.New())),
	)
	res, _ := engine.Normalize(context.Background(), "周丽群")
	fmt.Println(res.Runtime.LexiconVersion)
	// Output: v1
}

// ExampleWrapProcessorError demonstrates error detection via errors.Is.
func ExampleWrapProcessorError() {
	err := lexnorm.WrapProcessorError("alias", "match", lexnorm.ErrRuntime)
	fmt.Println(errors.Is(err, lexnorm.ErrRuntime))
	// Output: true
}

// ----------------------------------------------------------------------------
// Test helpers (this file only)
// ----------------------------------------------------------------------------

type echoProcessor struct{ name string }

func (e *echoProcessor) Name() string                                      { return e.name }
func (e *echoProcessor) Process(_ context.Context, _ *lexnorm.State) error { return nil }

type examplePanic struct{}

func (p *examplePanic) Name() string { return "panic" }
func (p *examplePanic) Process(_ context.Context, _ *lexnorm.State) error {
	panic("intentional")
}
