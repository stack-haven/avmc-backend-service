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

package lexnorm

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"
)

// Handler is the unit of work wrapped by Middleware.
//
// A Handler invokes the Pipeline (or any other work) on a State and
// returns an error. Middleware compose around Handlers.
type Handler func(ctx context.Context, s *State) error

// Middleware wraps a Handler to add cross-cutting concerns (tracing,
// timing, panic recovery, etc.).
//
// Middleware are composed outermost-first: when multiple Middleware are
// registered, the first one is the outermost layer and the last is the
// innermost (closest to the Pipeline execution).
//
// # Composition Order
//
// Given three middlewares A, B, C registered in that order:
//
//	final := A(B(C(handler)))
//
// So A's "before" code runs first, then B's, then C's, then the actual
// handler; on the way back, C's "after" code runs first.
type Middleware func(next Handler) Handler

// chainMiddleware composes a list of Middleware around a Handler.
//
// The first middleware in the slice becomes the outermost layer.
func chainMiddleware(handler Handler, mws ...Middleware) Handler {
	// Apply in reverse so first-registered is outermost.
	for i := len(mws) - 1; i >= 0; i-- {
		handler = mws[i](handler)
	}
	return handler
}

// Recover is a Middleware that catches panics in the wrapped Handler
// and converts them to errors.
//
// Recover should typically be the OUTERMOST Middleware so it catches
// panics from all other Middleware as well.
func Recover() Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, s *State) (err error) {
			defer func() {
				if r := recover(); r != nil {
					stack := debug.Stack()
					err = fmt.Errorf("lexnorm: panic recovered: %v\n%s: %w", r, stack, ErrRuntime)
				}
			}()
			return next(ctx, s)
		}
	}
}

// Timeout returns a Middleware that applies a per-call timeout.
//
// If the wrapped Handler does not complete within d, the context is
// cancelled and the call returns context.DeadlineExceeded. The
// underlying Processors SHOULD honor ctx.Done() for cooperative
// cancellation.
//
// Typical use:
//
//	engine, _ := lexnorm.New(
//	    lexnorm.WithPipeline(p),
//	    lexnorm.WithMiddleware(lexnorm.Timeout(5 * time.Second)),
//	)
func Timeout(d time.Duration) Middleware {
	return func(next Handler) Handler {
		return func(ctx context.Context, s *State) error {
			ctx, cancel := context.WithTimeout(ctx, d)
			defer cancel()
			return next(ctx, s)
		}
	}
}
