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

package lexicon

import (
	"errors"
	"fmt"
	"sync/atomic"
)

// Store provides atomic, immutable Lexicon snapshots with Last Known Good
// semantics (1.1 §40.2).
//
// # Architecture (HA)
//
// Store holds an atomic.Pointer[Lexicon] that is read on every Current()
// call. Updates are atomic: a successful TryUpdate swaps the pointer;
// a failed build leaves the previous (Last Known Good) Lexicon intact.
// Concurrent readers see either the old or new Lexicon, never a partial
// state.
//
// # Request Consistency (Architecture Invariant I8)
//
// A Normalize call captures Lexicon via Store.Current() at the start and
// uses the captured pointer for the duration of the call. Concurrent
// TryUpdate calls do NOT affect in-flight requests.
//
// # Concurrency
//
// Store is safe for concurrent use. Current() may be called from any
// number of goroutines; TryUpdate serializes via a build lock.
type Store struct {
	current  atomic.Pointer[Lexicon]
	building atomic.Bool
}

// ErrBuildInProgress is returned by TryUpdate when another TryUpdate is
// already in progress on the same Store.
var ErrBuildInProgress = errors.New("lexicon: build already in progress")

// ErrNilLexicon is returned by Swap / TryUpdate when the candidate
// Lexicon is nil.
var ErrNilLexicon = errors.New("lexicon: nil Lexicon")

// ErrEmptyLexicon is returned by TryUpdate when the new Lexicon has no
// Entries. Use ErrNilLexicon for nil; ErrEmptyLexicon for zero-content.
var ErrEmptyLexicon = errors.New("lexicon: empty Lexicon rejected")

// NewStore creates a Store with an initial Lexicon.
//
// initial may be nil; in that case Current() returns nil until Swap or
// TryUpdate succeeds.
func NewStore(initial Lexicon) *Store {
	s := &Store{}
	if initial != nil {
		lex := initial
		s.current.Store(&lex)
	}
	return s
}

// Current returns the current Lexicon snapshot.
//
// Returns nil if the Store has no Lexicon (no initial, no successful
// Swap or TryUpdate yet). The returned Lexicon is safe to use
// concurrently and may be retained for the duration of a Normalize call.
func (s *Store) Current() Lexicon {
	if p := s.current.Load(); p != nil {
		return *p
	}
	return nil
}

// IsInitialized reports whether Current() returns a non-nil Lexicon.
func (s *Store) IsInitialized() bool {
	return s.current.Load() != nil
}

// Version returns the Version() of the current Lexicon (or "" if
// uninitialized).
func (s *Store) Version() string {
	if lex := s.Current(); lex != nil {
		return lex.Version()
	}
	return ""
}

// Swap atomically replaces the current Lexicon with new.
//
// Returns ErrNilLexicon if new is nil. On error, current is unchanged.
//
// Swap is synchronous: callers see the new Lexicon immediately after
// Swap returns. In-flight Normalize calls retain their captured Snapshot.
func (s *Store) Swap(new Lexicon) error {
	if new == nil {
		return ErrNilLexicon
	}
	s.current.Store(&new)
	return nil
}

// TryUpdate atomically builds a new Lexicon and swaps if successful.
//
// # Last Known Good (LKG) Semantics
//
//   - Phase 1: build() is invoked. If it returns an error, TryUpdate
//     returns the error and the current Lexicon is UNCHANGED.
//   - Phase 2: validate (non-nil, non-empty).
//   - Phase 3: atomic swap. Subsequent Current() calls return new.
//
// # Concurrency
//
// TryUpdate is serialized via an atomic build lock. Concurrent callers
// see ErrBuildInProgress. The lock is released even if build() panics
// (via defer + recover).
//
// # In-flight Requests
//
// TryUpdate does NOT abort or affect in-flight Normalize calls. They
// continue using the Lexicon they captured at the start (Request
// Consistency, architecture invariant I8).
func (s *Store) TryUpdate(build func() (Lexicon, error)) (err error) {
	if !s.building.CompareAndSwap(false, true) {
		return ErrBuildInProgress
	}
	defer s.building.Store(false)

	// Defensive panic recovery so the build lock is always released.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("lexicon: build panicked: %v", r)
		}
	}()

	// Phase 1: Build.
	newLex, err := build()
	if err != nil {
		return fmt.Errorf("lexicon: build: %w", err)
	}

	// Phase 2: Validate.
	if newLex == nil {
		return ErrNilLexicon
	}
	if newLex.Len() == 0 {
		return ErrEmptyLexicon
	}

	// Phase 3: Atomic swap.
	s.current.Store(&newLex)
	return nil
}

// IsBuildInProgress reports whether a TryUpdate is currently running.
func (s *Store) IsBuildInProgress() bool {
	return s.building.Load()
}
