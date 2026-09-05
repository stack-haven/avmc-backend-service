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
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stack-haven/lexnorm/lexicon"
)

// ----------------------------------------------------------------------------
// Test helpers
// ----------------------------------------------------------------------------

// mustBuildLexicon builds a tiny Lexicon with the given version for
// testing Store behavior.
func mustBuildLexicon(t *testing.T, version string) lexicon.Lexicon {
	t.Helper()
	lex, err := lexicon.NewBuilderWithVersion(version).
		Add(lexicon.Entry{ID: "e1", Text: "hello-" + version}).
		Build()
	if err != nil {
		t.Fatalf("Build Lexicon: %v", err)
	}
	return lex
}

// ----------------------------------------------------------------------------
// Construction & Current
// ----------------------------------------------------------------------------

func TestStore_NewStoreWithInitial(t *testing.T) {
	lex := mustBuildLexicon(t, "v1")
	s := lexicon.NewStore(lex)

	if got := s.Current(); got != lex {
		t.Errorf("Current() must return the initial Lexicon")
	}
	if !s.IsInitialized() {
		t.Error("IsInitialized() must be true")
	}
	if got := s.Version(); got != "v1" {
		t.Errorf("Version() = %q, want v1", got)
	}
}

func TestStore_NewStoreWithNilInitial(t *testing.T) {
	s := lexicon.NewStore(nil)
	if got := s.Current(); got != nil {
		t.Errorf("Current() must return nil for nil initial, got %v", got)
	}
	if s.IsInitialized() {
		t.Error("IsInitialized() must be false for nil initial")
	}
	if got := s.Version(); got != "" {
		t.Errorf("Version() = %q, want empty", got)
	}
}

// ----------------------------------------------------------------------------
// Swap
// ----------------------------------------------------------------------------

func TestStore_Swap(t *testing.T) {
	s := lexicon.NewStore(mustBuildLexicon(t, "v1"))

	v2 := mustBuildLexicon(t, "v2")
	if err := s.Swap(v2); err != nil {
		t.Fatalf("Swap: %v", err)
	}
	if got := s.Current(); got != v2 {
		t.Error("Swap did not update Current()")
	}
	if got := s.Version(); got != "v2" {
		t.Errorf("Version() = %q, want v2", got)
	}
}

func TestStore_Swap_Nil(t *testing.T) {
	v1 := mustBuildLexicon(t, "v1")
	s := lexicon.NewStore(v1)

	if err := s.Swap(nil); !errors.Is(err, lexicon.ErrNilLexicon) {
		t.Errorf("Swap(nil) must return ErrNilLexicon, got %v", err)
	}
	// Current must remain V1 (LKG).
	if got := s.Current(); got != v1 {
		t.Error("failed Swap must preserve V1 (Last Known Good)")
	}
}

// ----------------------------------------------------------------------------
// TryUpdate: success
// ----------------------------------------------------------------------------

func TestStore_TryUpdate_Success(t *testing.T) {
	s := lexicon.NewStore(mustBuildLexicon(t, "v1"))

	v2 := mustBuildLexicon(t, "v2")
	err := s.TryUpdate(func() (lexicon.Lexicon, error) {
		return v2, nil
	})
	if err != nil {
		t.Fatalf("TryUpdate: %v", err)
	}
	if got := s.Version(); got != "v2" {
		t.Errorf("Version() = %q, want v2", got)
	}
}

func TestStore_TryUpdate_FromNilInitial(t *testing.T) {
	s := lexicon.NewStore(nil)
	if s.IsInitialized() {
		t.Fatal("expected uninitialized Store")
	}

	v1 := mustBuildLexicon(t, "v1")
	err := s.TryUpdate(func() (lexicon.Lexicon, error) { return v1, nil })
	if err != nil {
		t.Fatal(err)
	}
	if !s.IsInitialized() {
		t.Error("Store must be initialized after successful TryUpdate")
	}
	if got := s.Version(); got != "v1" {
		t.Errorf("Version() = %q, want v1", got)
	}
}

// ----------------------------------------------------------------------------
// TryUpdate: failures preserve LKG
// ----------------------------------------------------------------------------

func TestStore_TryUpdate_Fail_LKG(t *testing.T) {
	v1 := mustBuildLexicon(t, "v1")
	s := lexicon.NewStore(v1)

	buildErr := errors.New("build failed")
	err := s.TryUpdate(func() (lexicon.Lexicon, error) {
		return nil, buildErr
	})
	if err == nil {
		t.Fatal("TryUpdate with build error must return error")
	}
	// Current must remain V1 (LKG).
	if got := s.Current(); got != v1 {
		t.Error("failed TryUpdate must preserve V1 (Last Known Good)")
	}
	if got := s.Version(); got != "v1" {
		t.Errorf("Version() = %q, want v1 (LKG)", got)
	}
}

func TestStore_TryUpdate_NilResult_ReturnsError(t *testing.T) {
	s := lexicon.NewStore(mustBuildLexicon(t, "v1"))

	err := s.TryUpdate(func() (lexicon.Lexicon, error) {
		return nil, nil // build "succeeded" but returned nil
	})
	if !errors.Is(err, lexicon.ErrNilLexicon) {
		t.Errorf("TryUpdate with nil result must return ErrNilLexicon, got %v", err)
	}
	// V1 preserved.
	if got := s.Version(); got != "v1" {
		t.Errorf("Version() = %q, want v1 (LKG)", got)
	}
}

func TestStore_TryUpdate_EmptyResult_ReturnsError(t *testing.T) {
	s := lexicon.NewStore(mustBuildLexicon(t, "v1"))

	emptyLex, err := lexicon.NewBuilder().Build()
	if err != nil {
		t.Fatal(err)
	}
	if emptyLex.Len() != 0 {
		t.Fatalf("empty Lexicon should have 0 entries, got %d", emptyLex.Len())
	}

	err = s.TryUpdate(func() (lexicon.Lexicon, error) {
		return emptyLex, nil
	})
	if !errors.Is(err, lexicon.ErrEmptyLexicon) {
		t.Errorf("TryUpdate with empty Lexicon must return ErrEmptyLexicon, got %v", err)
	}
	if got := s.Version(); got != "v1" {
		t.Errorf("Version() = %q, want v1 (LKG)", got)
	}
}

func TestStore_TryUpdate_Panic_Recovers(t *testing.T) {
	// If build panics, the build lock must be released and current LKG preserved.
	s := lexicon.NewStore(mustBuildLexicon(t, "v1"))

	err := s.TryUpdate(func() (lexicon.Lexicon, error) {
		panic("intentional panic")
	})
	if err == nil {
		t.Fatal("TryUpdate must propagate panic as error")
	}

	// Build lock must be released; subsequent TryUpdate should work.
	v2 := mustBuildLexicon(t, "v2")
	err = s.TryUpdate(func() (lexicon.Lexicon, error) { return v2, nil })
	if err != nil {
		t.Errorf("subsequent TryUpdate must succeed (build lock released): %v", err)
	}
	if got := s.Version(); got != "v2" {
		t.Errorf("Version() = %q, want v2", got)
	}
}

// ----------------------------------------------------------------------------
// TryUpdate: concurrency (build lock)
// ----------------------------------------------------------------------------

func TestStore_TryUpdate_ConcurrentBuildsSerialized(t *testing.T) {
	s := lexicon.NewStore(mustBuildLexicon(t, "v1"))

	const N = 50
	var wg sync.WaitGroup
	wg.Add(N)
	var successCount atomic.Int32
	var inProgressCount atomic.Int32
	var maxInProgress atomic.Int32

	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			lex := mustBuildLexicon(t, fmt.Sprintf("v%d", i))
			err := s.TryUpdate(func() (lexicon.Lexicon, error) {
				cur := inProgressCount.Add(1)
				for {
					max := maxInProgress.Load()
					if cur <= max || maxInProgress.CompareAndSwap(max, cur) {
						break
					}
				}
				time.Sleep(time.Millisecond) // simulate build time
				inProgressCount.Add(-1)
				return lex, nil
			})
			if err == nil {
				successCount.Add(1)
			} else if !errors.Is(err, lexicon.ErrBuildInProgress) {
				t.Errorf("unexpected error: %v", err)
			}
		}(i)
	}
	wg.Wait()

	if maxInProgress.Load() != 1 {
		t.Errorf("max concurrent builds = %d, want 1 (serialized)", maxInProgress.Load())
	}
	if successCount.Load() < 1 {
		t.Error("at least one TryUpdate must succeed")
	}
}

// ----------------------------------------------------------------------------
// Request Consistency (Architecture Invariant I8)
// ----------------------------------------------------------------------------

func TestStore_RequestConsistency_CapturedReferenceUnaffected(t *testing.T) {
	// Capture V1, then Swap to V2; captured reference must remain V1.
	v1 := mustBuildLexicon(t, "v1")
	s := lexicon.NewStore(v1)

	captured := s.Current() // simulate Normalize capturing at start
	if captured.Version() != "v1" {
		t.Fatalf("captured version = %q, want v1", captured.Version())
	}

	v2 := mustBuildLexicon(t, "v2")
	if err := s.Swap(v2); err != nil {
		t.Fatal(err)
	}

	// captured must remain V1 (immutability).
	if got := captured.Version(); got != "v1" {
		t.Errorf("captured version = %q, want v1 (Request Consistency)", got)
	}

	// Current must be V2.
	if got := s.Current().Version(); got != "v2" {
		t.Errorf("Current.Version() = %q, want v2", got)
	}
}

func TestStore_RequestConsistency_DuringTryUpdate(t *testing.T) {
	// V1 → TryUpdate starts building V2 → during build, callers see V1.
	v1 := mustBuildLexicon(t, "v1")
	s := lexicon.NewStore(v1)

	buildStarted := make(chan struct{})
	releaseBuild := make(chan struct{})
	v2 := mustBuildLexicon(t, "v2")

	go func() {
		_ = s.TryUpdate(func() (lexicon.Lexicon, error) {
			close(buildStarted)
			<-releaseBuild // wait for test to release us
			return v2, nil
		})
	}()

	<-buildStarted

	// While build is in progress, Current() must still return V1.
	if got := s.Current().Version(); got != "v1" {
		t.Errorf("during build, Current.Version() = %q, want v1 (LKG)", got)
	}
	if !s.IsBuildInProgress() {
		t.Error("IsBuildInProgress() must return true during build")
	}

	// Concurrent TryUpdate must fail with ErrBuildInProgress.
	err := s.TryUpdate(func() (lexicon.Lexicon, error) {
		return mustBuildLexicon(t, "v-conflict"), nil
	})
	if !errors.Is(err, lexicon.ErrBuildInProgress) {
		t.Errorf("concurrent TryUpdate must return ErrBuildInProgress, got %v", err)
	}

	close(releaseBuild)
	// Give the build time to complete.
	time.Sleep(50 * time.Millisecond)

	// Now Current() must be V2.
	if got := s.Current().Version(); got != "v2" {
		t.Errorf("after build, Current.Version() = %q, want v2", got)
	}
}

// ----------------------------------------------------------------------------
// Concurrent reads + writes
// ----------------------------------------------------------------------------

func TestStore_ConcurrentReadsDuringSwap(t *testing.T) {
	v1 := mustBuildLexicon(t, "v1")
	s := lexicon.NewStore(v1)

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// Readers: must see v1 or any v2/v3/... version (writers write "vN").
	const N = 50
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				lex := s.Current()
				if lex == nil {
					t.Error("Current returned nil")
					return
				}
				v := lex.Version()
				// Accept any "vN" pattern; reject garbage.
				if len(v) < 2 || v[0] != 'v' {
					t.Errorf("malformed version: %q", v)
				}
			}
		}()
	}

	// Writers: swap to several versions.
	const M = 5
	for i := 0; i < M; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v := mustBuildLexicon(t, fmt.Sprintf("v%d", i+2))
			if err := s.Swap(v); err != nil {
				t.Errorf("Swap: %v", err)
			}
			time.Sleep(time.Millisecond)
		}(i)
	}

	// Let writers do their work, then signal stop.
	time.Sleep(30 * time.Millisecond)
	close(stop)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("goroutines did not finish in 5s")
	}
}
