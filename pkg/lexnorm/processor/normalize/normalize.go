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

// Package normalize implements the Normalize Processor.
//
// # Position in Standard Pipeline
//
// Normalize runs first in the Standard Pipeline. It is the foundation
// that downstream Processors (Disfluency, Alias, Deterministic) depend on
// for consistent whitespace and encoding.
//
// # Behavior
//
//   - Trims leading and trailing whitespace.
//   - Collapses runs of whitespace (space, tab, newline) into single
//     spaces.
//   - Strips control characters (except \n and \t which become space).
//   - Optionally converts full-width ASCII (U+FF01..U+FF5E, U+3000) to
//     half-width (default: enabled).
//
// # Certainty
//
// Normalize is fully deterministic and high-certainty: rules are
// unambiguous and do not depend on Lexicon content.
package normalize

import (
	"context"
	"encoding/json"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/stack-haven/lexnorm"
)

const (
	// Name is the Processor name exposed via Processor.Name().
	Name = "normalize"

	// Version is the semantic version of this Processor.
	Version = "v1"
)

// Processor implements the Normalize step.
//
// It holds no Lexicon reference (Normalize is Lexicon-independent).
type Processor struct {
	// fullWidthToHalf controls whether full-width ASCII characters are
	// converted to half-width. Default true.
	fullWidthToHalf bool
}

// New returns a Processor with default settings.
func New() *Processor {
	return &Processor{fullWidthToHalf: true}
}

// WithFullWidthToHalf enables or disables full-width-to-half-width conversion.
// Default is enabled.
func (p *Processor) WithFullWidthToHalf(enable bool) *Processor {
	p.fullWidthToHalf = enable
	return p
}

// Name implements lexnorm.Processor.
func (p *Processor) Name() string { return Name }

// Version implements lexnorm.Versioner.
func (p *Processor) Version() string { return Version }

// Certainty implements lexnorm.CertaintyReporter.
func (p *Processor) Certainty() lexnorm.Certainty { return lexnorm.CertaintyHigh }

// Process implements lexnorm.Processor.
//
// Performs per-position replacements (whitespace collapse, control
// char removal, fullwidth → halfwidth) so that Original byte offsets
// of meaningful content remain stable for downstream Processors.
func (p *Processor) Process(_ context.Context, s *lexnorm.State) error {
	if s.Text() == "" {
		return nil
	}

	meta := lexnorm.ChangeMeta{
		Source:     Name,
		Confidence: 1.0,
		Reason:     "whitespace and full-width normalization",
	}

	type edit struct {
		start, end int
		to         string
	}
	var edits []edit

	text := s.Text()
	lastWasSpace := true // suppress leading whitespace
	for i := 0; i < len(text); {
		r, size := utf8.DecodeRuneInString(text[i:])

		if p.fullWidthToHalf {
			r = fullWidthToHalfRune(r)
		}

		if unicode.IsSpace(r) {
			if !lastWasSpace {
				edits = append(edits, edit{start: i, end: i + size, to: " "})
				lastWasSpace = true
			} else {
				edits = append(edits, edit{start: i, end: i + size, to: ""})
			}
			i += size
			continue
		}

		if unicode.IsControl(r) {
			edits = append(edits, edit{start: i, end: i + size, to: ""})
			i += size
			continue
		}

		// Halfwidth conversion only differs from original for full-width
		// runes (3 bytes → 1 or 3 bytes → 3 bytes).
		if p.fullWidthToHalf {
			origSize := size
			newSize := utf8.RuneLen(r)
			if newSize != origSize {
				buf := make([]byte, newSize)
				utf8.EncodeRune(buf, r)
				edits = append(edits, edit{start: i, end: i + origSize, to: string(buf)})
			}
		}

		lastWasSpace = false
		i += size
	}

	// If input ended on whitespace, change the trailing " " collapse
	// marker to "" so trailing whitespace is fully suppressed.
	if lastWasSpace {
		for k := len(edits) - 1; k >= 0; k-- {
			if edits[k].to == " " {
				edits[k].to = ""
				break
			}
		}
	}

	// Apply edits from right to left so earlier offsets stay valid.
	for j := len(edits) - 1; j >= 0; j-- {
		e := edits[j]
		if e.to == "" && e.start == e.end {
			continue // no-op
		}
		if err := s.Replace(lexnorm.Span{Start: e.start, End: e.end}, e.to, meta); err != nil {
			return err
		}
	}

	return nil
}

// normalizeText applies the configured transformations.
//
// The transformation is pure (no side effects, deterministic).
func normalizeText(s string, fullWidthToHalf bool) string {
	b := strings.Builder{}
	b.Grow(len(s))

	// lastWasSpace starts true so leading whitespace is suppressed.
	lastWasSpace := true

	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		i += size

		if fullWidthToHalf {
			r = fullWidthToHalfRune(r)
		}

		if unicode.IsSpace(r) {
			if !lastWasSpace {
				b.WriteByte(' ')
				lastWasSpace = true
			}
			continue
		}

		if unicode.IsControl(r) {
			// Skip control characters entirely.
			continue
		}

		b.WriteRune(r)
		lastWasSpace = false
	}

	result := b.String()
	if lastWasSpace {
		result = strings.TrimRight(result, " ")
	}
	return result
}

// fullWidthToHalfRune converts full-width ASCII characters to half-width.
//
//   - U+FF01..U+FF5E (！..～) → 0x21..0x7E
//   - U+3000 (full-width space) → U+0020 (regular space)
//
// Other runes are passed through unchanged.
func fullWidthToHalfRune(r rune) rune {
	if r >= 0xFF01 && r <= 0xFF5E {
		return r - 0xFEE0
	}
	if r == 0x3000 {
		return ' '
	}
	return r
}

// Descriptor is the Registry Descriptor for this Processor.
//
// Use it with lexnorm.NewRegistry().Register(normalize.Descriptor) to
// enable dynamic / configuration-driven construction.
var Descriptor = lexnorm.Descriptor{
	Name:      Name,
	Certainty: lexnorm.CertaintyHigh,
	New: func(_ json.RawMessage) (lexnorm.Processor, error) {
		return New(), nil
	},
	Default: func() any { return nil },
}
