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

// Golden corpus tests: freeze the observable behavior of the built-in
// Presets over a fixed corpus, so any behavior change in later phases is
// an explicit, reviewable diff.
//
// Regenerate goldens after an INTENDED behavior change:
//
//	go test -run TestGoldenCorpus -update ./...
//
// Every golden diff must map 1:1 to a CHANGELOG entry.

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/presets"
)

var updateGolden = flag.Bool("update", false, "rewrite golden files")

// goldenLexicon covers every VariantKind plus the canonical-substring
// hazard shapes that later phases address.
func goldenLexicon() lexicon.Lexicon {
	lex, err := lexicon.NewBuilder().Add(
		lexicon.Entry{ID: "e-alias", Text: "田华",
			Variants: []lexicon.Variant{
				{Text: "田工", Kind: lexicon.VariantAlias, Confidence: 0.9},
			}},
		lexicon.Entry{ID: "e-corr", Text: "金种籽",
			Variants: []lexicon.Variant{
				{Text: "金种仔", Kind: lexicon.VariantCorrection, Confidence: 1.0},
			}},
		lexicon.Entry{ID: "e-homo", Text: "袁孟莲",
			Variants: []lexicon.Variant{
				{Text: "袁梦莲", Kind: lexicon.VariantHomophone, Confidence: 0.9},
			}},
		lexicon.Entry{ID: "e-fuzzy", Text: "周丽群",
			Variants: []lexicon.Variant{
				{Text: "周莉群", Kind: lexicon.VariantApproximate, Confidence: 0.95},
			}},
	).Build()
	if err != nil {
		panic(err)
	}
	return lex
}

// goldenCorpus covers the input classes required by the Processor spec
// (§8 独立运行 / §12 测试要求): normal, empty, no-candidate, multi-match,
// protected span, fillers, OOV, boundary characters.
var goldenCorpus = []struct {
	name string
	text string
}{
	{"normal", "小田今天帮我查一下金种仔的情况"},
	{"empty", ""},
	{"no_candidate", "今天天气不错"},
	{"multi_match", "田工和周莉群都来了，金种仔也在"},
	{"fillers", "你好 嗯 世界"},
	{"filler_between_words", "请让 呃 田工 嗯 来开会"},
	{"oov", "完全不在词库里的句子"},
	{"boundary_chars", "田工🎉\t周莉群\n"},
	{"repeated", "啊啊啊啊这个测试"},
}

func goldenEngine(t *testing.T, name string) *lexnorm.Engine {
	t.Helper()
	lex := goldenLexicon()
	var preset *lexnorm.Preset
	switch name {
	case "standard":
		preset = presets.Standard(lex, nil)
	case "high-accuracy":
		preset = presets.HighAccuracy(lex, nil)
	case "fast":
		preset = presets.Fast(lex)
	default:
		t.Fatalf("unknown preset %q", name)
	}
	e, err := lexnorm.New(lexnorm.WithPreset(*preset))
	if err != nil {
		t.Fatalf("New(%s): %v", name, err)
	}
	return e
}

// renderResult is the golden serialization: deterministic, compact, and
// human-reviewable.
func renderResult(res lexnorm.Result) string {
	var b strings.Builder
	fmt.Fprintf(&b, "text: %q\n", res.Text)
	fmt.Fprintf(&b, "status: %s\n", res.Status.String())
	for _, c := range res.Changes {
		fmt.Fprintf(&b, "change: %q->%q span=%d:%d src=%s conf=%.2f applied=%v\n",
			c.From, c.To, c.Span.Start, c.Span.End, c.Source, c.Confidence, c.Applied)
	}
	for _, s := range res.Suggestions {
		fmt.Fprintf(&b, "suggest: %q->%q span=%d:%d src=%s conf=%.2f\n",
			s.From, s.To, s.Span.Start, s.Span.End, s.Source, s.Confidence)
	}
	return b.String()
}

func TestGoldenCorpus(t *testing.T) {
	for _, presetName := range []string{"standard", "high-accuracy", "fast"} {
		e := goldenEngine(t, presetName)
		for _, tc := range goldenCorpus {
			tc := tc
			goldenName := fmt.Sprintf("%s-%s", presetName, tc.name)
			t.Run(goldenName, func(t *testing.T) {
				res, err := e.Normalize(context.Background(), tc.text)
				if err != nil {
					t.Fatalf("Normalize: %v", err)
				}
				got := renderResult(res)

				goldenPath := filepath.Join("testdata", "golden", goldenName+".golden")
				if *updateGolden {
					if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(goldenPath, []byte(got), 0o644); err != nil {
						t.Fatal(err)
					}
					return
				}
				want, err := os.ReadFile(goldenPath)
				if err != nil {
					t.Fatalf("golden file missing (run with -update): %v", err)
				}
				if got != string(want) {
					t.Errorf("golden mismatch for %s\n--- want ---\n%s\n--- got ---\n%s",
						goldenName, want, got)
				}
			})
		}
	}
}
