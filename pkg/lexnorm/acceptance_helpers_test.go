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

	"github.com/stack-haven/lexnorm"
	"github.com/stack-haven/lexnorm/lexicon"
	"github.com/stack-haven/lexnorm/processor/alias"
)

// lockingProcessor locks a fixed span at the start of its Process call.
// Used in Scenario C to verify Protected Span prevents later modifications.
type lockingProcessor struct {
	name string
	span lexnorm.Span
}

func (p *lockingProcessor) Name() string { return p.name }
func (p *lockingProcessor) Process(_ context.Context, s *lexnorm.State) error {
	return s.Lock(p.span)
}

// newPipelineWithLock returns a Pipeline that Locks the given span first
// and then runs the provided inner Pipeline.
//
// Used by acceptance test C to verify the Lock conflict path.
func newPipelineWithLock(start, end int, lex lexicon.Lexicon) lexnorm.Pipeline {
	lock := &lockingProcessor{name: "lock", span: lexnorm.Span{Start: start, End: end}}
	aliasP := alias.New(lex)
	return lexnorm.NewPipeline(lock, aliasP)
}
