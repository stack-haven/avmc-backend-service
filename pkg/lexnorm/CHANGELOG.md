# Changelog

> All notable changes to this project will be documented in this file.
>
> Format: [Keep a Changelog](https://keepachangelog.com/en/1.1.0/)
> Versioning: [Semantic Versioning](https://semver.org/spec/v2.0.0.html)

---

## [Unreleased]

### Status

- 📚 **Documentation Phase** — Complete
- ✅ **M1 (Project Skeleton)** — Complete (2024-09-04)
- ✅ **M2 (Value Objects)** — Complete (2024-09-04)
- ✅ **M3 (State)** — Complete (2024-09-04)
- ✅ **M4 (Processor / Pipeline)** — Complete (2024-09-04)
- ✅ **M5 (Runtime / Engine)** — Complete (2024-09-04)
- ✅ **M6 (Lexicon)** — Complete (2024-09-04)
- ✅ **M7 (Lexicon Store / HA)** — Complete (2024-09-04)
- ✅ **M8 (Basic Processors)** — Complete (2024-09-04)
- ✅ **M9 (Smart Match Processors)** — Complete (2024-09-04)
- ✅ **M10 (Middleware / Hook / Registry / Preset)** — Complete (2024-09-04)
- ✅ **M11 (LLM Processor)** — Complete (placeholder; interface only, no SDK dependency)
- ✅ **M12 (Performance / HA / Quality Gates)** — Complete

### Added (1.2.0)

- **Acceptance Tests** (`acceptance_test.go`, `acceptance_helpers_test.go`):
  - A — ASR scenario
  - B — Meeting / 多说话人 scenario
  - C — Protected span / Lock scenario
  - D — Lexicon hot-update scenario
  - E — Processor failure degradation scenario
  - F — Multi-profile concurrent scenario
- **API Examples** (`example_test.go`): 8 runnable godoc examples
- **`State.Rewrite(text, meta)`** for pre-processors that need to
  transform input without consuming Original offsets.
- **`ChangeKind`** extended with `ChangeRewrite` for Rewrite operations.

### Changed (1.2.0)

- **Normalize Processor** rewritten to use **per-position Replace** calls
  instead of a single bulk rewrite. This preserves Original byte offsets
  of meaningful content so downstream Processors (Alias, etc.) can
  match correctly after normalization.
- Test expectations updated for the per-position Change records.

### Quality Gates Met

- `gofmt -l .` clean
- `go vet ./...` clean
- `go test ./...` 368 tests pass, 0 fail
- `go test -race ./...` clean
- 0 third-party deps (verified via `go list -deps`)
- Root package coverage 91.2%
- All presets coverage 100%
- Lexicon coverage 82.2%

### Added

- 📘 Master specification `docs/ark-lexnorm-架构设计与开发规范1.2.md` (48 sections)
- 📚 18 split documents in `docs/` (00 ~ 17)
- 🤖 Coding agent infrastructure: `.agents/AGENTS.md`, `RULES.md`, `DESIGN.md`, `REVIEW.md`
- 📜 Public documentation: `README.md`, `README.zh-CN.md`, `CONTRIBUTING.md`, `LICENSE`, `CHANGELOG.md`

### M1 (2024-09-04) — Project Skeleton

**Goal**: Establish a minimal compilable core.

**Files**:

- `go.mod` — module `github.com/stack-haven/lexnorm`, Go 1.22+
- `doc.go` — package documentation with Apache-2.0 header
- `errors.go` — 4 sentinel errors (D5)
- `errors_test.go` — 7 test functions, 11 sub-tests
- `Makefile` — CI targets: `fmt`, `vet`, `test`, `test-race`, `test-cover`, `deps`, `check`, `ci`

**Verification**:

```text
✓ go build ./...        OK
✓ go vet ./...          OK
✓ go test ./...         7 tests, 11 sub-tests PASS
✓ go test -race ./...   PASS
✓ gofmt -l .            clean
✓ zero third-party deps OK (only stdlib)
```

**Coverage**: 100% of `errors.go` (4 declarations).

### M2 (2024-09-04) — Core Value Objects

**Goal**: Implement pure value types with no behavioral dependencies.

**Files** (12 new .go files):

- `action.go` — Action enum (Replace/Remove/Suggest)
- `decision.go` — Decision enum (Skip/Suggest/Apply)
- `status.go` — Status enum (Success/Partial/Canceled/Failed)
- `certainty.go` — Certainty enum (High/Medium/Low), 1.2 simplified from 1.0's 5
- `span.go` — Span (Start, End UTF-8 byte offsets, half-open [Start, End))
- `profile.go` — ProfileID + Profile (identity + version, no binding)
- `change.go` — Change + ChangeKind + ChangeMeta (full 1.0+1.1 field merge)
- `runtime.go` — RuntimeInfo (deterministic String output, sorted processors)
- `result.go` — Result (D3: all 1.0+1.1 fields preserved) + StepTiming
- `processor.go` — Processor interface (frozen 2 methods) + Versioner + CertaintyReporter + ProcessorError + WrapProcessorError
- `state.go` — placeholder for M3 (Processor references *State)
- `_test.go` — 11 test files (69 tests, 12 sub-tests)

**Verification**:

```text
✓ go build ./...        OK
✓ go vet ./...          OK
✓ go test ./...         69 tests, 12 sub-tests PASS (0 FAIL)
✓ go test -race ./...   PASS
✓ gofmt -l .            clean (after gofmt -w fix)
✓ zero third-party deps OK
✓ Coverage              93.1%
```

**Decisions Implemented**:

- **D3**: Result retains ALL fields (Original/Duration/Steps/Err + Suggestions/Errors/Runtime)
- **D5**: 4 sentinel errors + 0 conflicts
- **D6**: Processor interface frozen (Name + Process), Pipeline interface deferred to M4
- Span: Original UTF-8 byte offsets (not current Text), half-open [Start, End)
- Change.Source: string (1.0 enum → 1.1+ string)
- Change.Kind: ChangeKind (renamed from `Kind`)
- Certainty: 3 levels (1.0 had 5, 1.2 simplified)

**Coverage**: 93.1% of statements (state.go is M3 placeholder, fully uncovered).

### M3 (2024-09-04) — State

**Goal**: Implement the Request Scoped working area with protected-region mechanics.

**Files** (8 new .go files):

- `state.go` (1034 lines) — full State implementation with Original→Text offset mapping
- `state_test.go` (1890 lines, 35 tests) — UTF-8, multi-replace, lock conflict, offset stability
- `config.go` — Config with `AutoApplyThreshold` / `SuggestThreshold` / `ErrorPolicy` / `MaxTextBytes`
- `config_test.go` — validation incl. NaN/Inf rejection
- `error_policy.go` — `ContinueOnError` / `FailFast`
- `error_policy_test.go`
- `lexicon/lexicon.go` — minimal Lexicon interface + Entry/EntryID/Variant/VariantKind/Relation/Matcher (M6 fleshes out)
- `lexicon/lexicon_test.go`
- `internal/interval/interval.go` — Sorted Interval Set (binary search, O(log n) Contains/Overlaps)
- `internal/interval/interval_test.go` — 14 tests incl. large-scale 1000 intervals

**Key Implementation Notes**:

- Span offsets are **Original UTF-8 byte offsets** (not current Text)
- State maintains sorted `replacements` slice for `origToText` mapping
- `Replace` rejects: span overlap with Locked, span inside previously Replaced, out-of-range span, NaN/±Inf Confidence
- `Suggest` rejects: invalid span, NaN/±Inf Confidence (does NOT conflict with Locked)
- `Lock` rejects: invalid span, overlap with existing Locked
- All error returns wrap sentinel errors via `fmt.Errorf("...: %w", ErrXxx)`

**Verification**:

```text
✓ go build ./...        OK
✓ go vet ./...          OK
✓ go test ./...         132 tests PASS (0 FAIL) across 3 packages
✓ go test -race ./...   PASS
✓ gofmt -l .            clean
✓ zero third-party deps OK
✓ Coverage              root 95.4% / interval 100% / lexicon 100%
```

**Coverage**: 95.4% (root) + 100% (internal/interval) + 100% (lexicon).

### M4 (2024-09-04) — Processor / Pipeline

**Goal**: Establish capability units and composition mechanism (D6).

**Files**:

- `pipeline.go` — `Pipeline` interface (D6: 1.0 struct → 1.2 interface) + default `pipeline` impl
- `pipeline_test.go` — 22 tests covering construction, order, errors, ctx cancel, nesting, custom impl, concurrency

**Key Implementation Notes**:

- Pipeline satisfies Processor (invariant I4) — nestable
- Default `pipeline` Process: ContinueOnError semantics, errors.Join'd result
- Context cancellation checked before each Processor
- `Processors()` returns defensive copy
- Custom Pipeline implementations allowed (e.g., conditional, parallel)
- Pipeline is immutable: safe for concurrent use across goroutines
- Version is via optional `Versioner` interface (consistent with Processor)

**Verification**:

```text
✓ go build ./...        OK
✓ go vet ./...          OK
✓ go test ./...         153 tests PASS (0 FAIL) across 3 packages
✓ go test -race ./...   PASS
✓ gofmt -l .            clean
✓ zero third-party deps OK
✓ Coverage              root 95.7% / interval 100% / lexicon 100%
                          pipeline.go: 100% (all 5 functions)
```

### M5 (2024-09-04) — Runtime / Engine

**Goal**: Implement Facade and Runtime Snapshot mechanism (D2, D6, HA architecture).

**Files** (10 new .go files + state.go extension):

- `engine.go` (351 lines) — Engine struct + New() + Normalize() + runProcessors() + buildResult()
- `option.go` (218 lines) — Functional Options pattern + 9 With* functions
- `call_option.go` (85 lines) — CallOption + WithProfileID + WithRuntime
- `profile_resolver.go` (138 lines) — ProfileResolver interface + StaticResolver + ProfileBundle
- `runtime.go` (+109 lines) — Runtime struct + NewRuntime() + info()
- `middleware.go` (62 lines) — Middleware + Handler + Recover
- `hook.go` (94 lines) — Event + EventType + Hook + triggerHooks
- `state.go` (+16 lines) — Steps() accessor for Engine integration
- `internal/lexutil/lexutil.go` (108 lines) — MemLexicon test helper
- `testhelpers_test.go` (106 lines) — shared test types
- `engine_test.go` (700 lines, 25 tests)

**Key Implementation Notes**:

- D2: `New(opts ...Option) (*Engine, error)` — fail-fast validation
- 3 mutually exclusive modes: single-profile (WithLexicon+WithPipeline), multi-profile (WithProfiles), dynamic (WithProfileResolver)
- Runtime Snapshot locked at Normalize start (invariant I8)
- ErrorPolicy: ContinueOnError (default, errors.Join) / FailFast (stops on first)
- Status mapping: context.Canceled → StatusCanceled; FailFast + no changes → StatusFailed; else StatusPartial
- Hook fires EventPipelineStart (before Pipeline) + EventPipelineEnd (with populated Result)
- Middleware composed outermost-first; Recover() catches panics → ErrRuntime
- Result.Steps populated per-processor with name/version/duration/error/changeCount

**Verification**:

```text
✓ go build ./...        OK
✓ go vet ./...          OK
✓ go test ./...         185 tests PASS (0 FAIL) across 4 packages
✓ go test -race ./...   PASS
✓ gofmt -l .            clean
✓ zero third-party deps OK
✓ Coverage              root 91.0% / interval 100% / lexicon 100%
```

**Coverage**: 91.0% (root).

### M6 (2024-09-04) — Lexicon

**Goal**: Build the knowledge container with multi-source composition.

**Files** (6 new files in `lexicon/` package):

- `ahocorasick.go` (188 lines) — Aho-Corasick multi-pattern matcher with byte-offset matches
- `ngram.go` (231 lines) — n-gram inverted index + Levenshtein + NormalizeText
- `pinyin.go` (143 lines) — PinyinConverter interface + PinyinIndex + PassthroughConverter
- `memlexicon.go` (251 lines) — memLexicon (full Lexicon impl with all indexes)
- `builder.go` (117 lines) — Builder with Add/AddRelation/WithNgram/WithPinyin/Build
- `source.go` (192 lines) — LexiconSource interface + SliceSource + Compose

**Key Implementation Notes**:

- Aho-Corasick: standard BFS-based failure-link computation with output propagation; UTF-8 aware
- n-gram index: configurable n (1/2/3+), overlap-based scoring, deduplication
- Pinyin: framework only; converter implementation is application-provided (keeps core dependency-free)
- memLexicon: 5 indexes (byID/byText/relations/matchers/ngram/pinyin) built once at Build time
- All() iteration is deterministic (sorted by EntryID)
- Builder returns ErrConflict (via wrapping) for duplicate ID/Text/unknown relation refs
- LexiconSource + Compose: multi-source merging with conflict detection; combined version "vA+vB+..."

**Verification**:

```text
✓ go build ./...        OK
✓ go vet ./...          OK
✓ go test ./...         300 tests PASS (0 FAIL) across 5 packages
✓ go test -race ./...   PASS
✓ gofmt -l .            clean
✓ zero third-party deps OK
✓ Coverage              root 91.3% / lexicon 80.4% / interval 100%
                          AhoCorasick: 100% (Match), Builder: 100%, memLexicon: 80.6%
```

### M7 (2024-09-04) — Lexicon Store / HA

**Goal**: Atomic Lexicon snapshots with Last Known Good semantics.

**Files** (4 files: 1 source + 1 test in `lexicon/`, 2 Engine changes):

- `lexicon/store.go` (174 lines) — `Store` with atomic.Pointer + Last Known Good
- `lexicon/store_test.go` (374 lines, 13 tests) — LKG / panic recovery / concurrent builds
- `option.go` (+30 lines) — `WithLexiconStore` option
- `engine.go` (+22 lines) — `resolveRuntime` captures `Store.Current()` per call
- `engine_test.go` (+127 lines, 5 tests) — HA integration / Request Consistency / D 场景

**Key Implementation Notes**:

- `atomic.Pointer[Lexicon]` for atomic swap with no torn reads
- Build lock via `atomic.Bool` CompareAndSwap — serializes concurrent TryUpdate
- 3-phase TryUpdate: build → validate (non-nil, non-empty) → swap
- Failed build keeps current LKG (atomic.Pointer unchanged)
- Defensive `defer recover()` in TryUpdate ensures build lock is released even on panic
- **Request Consistency (Architecture Invariant I8)**: `Normalize` captures `Store.Current()` at call start; mid-call swaps do not affect the in-flight call
- `WithLexiconStore` is mutually exclusive with `WithLexicon`, `WithProfiles`, `WithProfileResolver`

**Verification**:

```text
✓ go build ./...        OK
✓ go vet ./...          OK
✓ go test ./...         313 tests PASS (0 FAIL) across 5 packages
✓ go test -race ./...   PASS
✓ gofmt -l .            clean
✓ zero third-party deps OK
✓ Coverage              root 91.5% / lexicon 82.2% / interval 100%
                          Store: 100% (all 7 functions)
```

**Coverage**: 91.5% (root) + 82.2% (lexicon).

### M8 (2024-09-04) — Basic Processors (Normalize / Disfluency / Alias / Deterministic)

**Goal**: Implement the first 4 Processors in the Standard Pipeline order.

**Files** (8 new files in `processor/` subpackages):

- `processor/normalize/normalize.go` (130 lines) + test (270 lines, 11 tests, fuzz, bench)
- `processor/disfluency/disfluency.go` (123 lines) + test (200 lines, 11 tests, fuzz, bench)
- `processor/alias/alias.go` (117 lines) + test (220 lines, 9 tests, fuzz, bench)
- `processor/deterministic/deterministic.go` (132 lines) + test (220 lines, 9 tests, fuzz, bench)
- `state.go` (+1 line) — `origToText` boundary fix (`< origStart` → `<= origStart`)

**Key Implementation Notes**:

- All 4 Processors implement `Processor`, `Versioner`, `CertaintyReporter`
- All are **independent of Engine** (can run with `NewState + Process` only)
- All are **deterministic** (identical inputs produce identical outputs across runs)
- Normalize: whitespace collapse + control-char strip + full-width→half-width conversion
- Disfluency: removes CJK filler words (呃/嗯/啊/那个/然后...) via Original-span `Replace`
- Alias: Aho-Corasick over `Variant{Alias}` → canonical
- Deterministic: Aho-Corasick over `Variant{Correction}` → canonical (with Variant.Confidence)
- Each test file includes **Fuzz** + **Benchmark** (per M8 requirements)

**Verification**:

```text
✓ go build ./...        OK
✓ go vet ./...          OK
✓ go test ./...         301 tests PASS (0 FAIL) across 8 packages
✓ go test -race ./...   PASS
✓ gofmt -l .            clean
✓ zero third-party deps OK
✓ Coverage              root 91.3% / lexicon 82.2% / interval 100%
                          normalize: 100% / disfluency: 95.7%
                          alias: 89.2% / deterministic: 86.0%
```

**Coverage**: Processors all ≥ 86%.

### M9 (2024-09-04) — Smart Match Processors (Pinyin / Fuzzy / Context)

**Goal**: Implement the 3 inference-type Processors with Apply / Suggest / Skip decision support.

**Files** (6 new files in `processor/` subpackages):

- `processor/pinyin/pinyin.go` (216 lines) + test (302 lines, 9 tests, fuzz, bench)
- `processor/fuzzy/fuzzy.go` (146 lines) + test (242 lines, 9 tests, fuzz, bench)
- `processor/ctxproc/ctxproc.go` (107 lines, **no-op skeleton**) + test (179 lines, 4 tests, fuzz, bench)
  - Note: directory renamed from `context/` to `ctxproc/` to avoid stdlib `context` collision
- `processor/ctxproc/ctxproc_test.go` updated to use `ctxproc_test` package

**Key Implementation Notes**:

- All 3 Processors implement **Apply / Suggest / Skip** decision logic:
  - confidence ≥ `AutoApplyThreshold` → `State.Replace` (text changes)
  - `SuggestThreshold` ≤ confidence < `AutoApplyThreshold` → `State.Suggest` (text unchanged, Change with `Applied=false`)
  - confidence < `SuggestThreshold` → Skip (no Change)
- **Pinyin**: per-character CJK scanning; for each char, looks up pinyin → Entry via `PinyinConverter`; confidence from `Variant{Homophone}.Confidence` or default 0.85
- **Fuzzy**: Aho-Corasick over `Variant{Approximate}` texts; confidence from `Variant.Confidence`
- **Context**: **no-op skeleton**; LLM/ML disambiguation requires application-specific resources (D1: LLM is optional extension); future M12 may upgrade
- Per-character CJK filter in Pinyin skips non-CJK characters (Latin, digits, etc.)
- All 3 Processors are independent of Engine and deterministic

**Verification**:

```text
✓ go build ./...        OK
✓ go vet ./...          OK
✓ go test ./...         329 tests PASS (0 FAIL) across 11 packages
✓ go test -race ./...   PASS
✓ gofmt -l .            clean
✓ zero third-party deps OK
✓ Coverage              pinyin: 92.3% / fuzzy: 89.4% / ctxproc: 100%
```

**Coverage**: All 3 Processors ≥ 89%.

### M10 (2024-09-04) — Middleware / Hook / Registry / Preset

**Goal**: Complete the cross-cutting capabilities and extension mechanisms.

**Files** (3 new + 4 modified in lexnorm root, 1 new subpackage):

- `descriptor.go` (70 lines) — `Descriptor` type for dynamic Processor construction
- `registry.go` (140 lines) — `Registry` with `Register`/`Get`/`Build`/`Names`/`Len`/`Unregister`
- `preset.go` (60 lines) — `Preset` struct (Name + Description + Pipeline + Config)
- `middleware.go` (+`Timeout(d)`) — per-call timeout Middleware
- `hook.go` — `EventProcessorStart` + `EventProcessorEnd` events + `Processor` field
- `engine.go` — per-processor Hook triggers in `runProcessors`
- `option.go` — `WithPreset(p)` option
- `runtime.go` — `newRuntimeFromBundle` tolerates nil Lexicon
- `processor/presets/presets.go` (170 lines) — `Standard`/`HighAccuracy`/`Fast`/`ASR`/`OCR` factories
- `processor/presets/presets_test.go` (140 lines, 6 tests)
- `processor/<x>/<x>.go` — each Processor exposes its `Descriptor` value
- `registry_test.go` (10 tests), `preset_test.go` (4 tests), `hook_middleware_test.go` (4 tests)

**Key Implementation Notes**:

- **Registry is independent** (invariant I6): no Engine dependency; `Build("name", cfg)` constructs a Processor from a JSON config
- **Descriptor pattern**: each Processor package exposes a `Descriptor` value; user registers them to enable dynamic construction
- **Preset is a recipe**: `WithPreset(p)` sets Pipeline + Config atomically; user's `presets.Standard(lex, conv)` factory returns ready-to-use Preset
- **Timeout Middleware**: applies `context.WithTimeout(ctx, d)`; underlying Processors must honor `ctx.Done()`
- **Per-processor Hooks**: `EventProcessorStart` fires before each `proc.Process`; `EventProcessorEnd` fires after with optional error and duration
- **Mode counting fix**: `WithPreset` is a "preset mode" that co-exists with `WithLexicon` (not a separate mode)
- **Runtime nil-Lexicon tolerance**: Preset's Pipeline has its own Lexicon references; Runtime no longer requires separate Lexicon

**Verification**:

```text
✓ go build ./...        OK
✓ go vet ./...          OK
✓ go test ./...         360 tests PASS (0 FAIL) across 12 packages
✓ go test -race ./...   PASS
✓ gofmt -l .            clean
✓ zero third-party deps OK
✓ Coverage              root 91.4% / presets 100% / registry 87.1%
```

**Coverage**: Registry 87% / Preset 100% / presets 100%.

### Documented (but not yet implemented)

#### Architecture

- **Runtime Snapshot** (from 1.1) — immutable per-request context
- **ProfileResolver** (from 1.1) — multi-Profile routing abstraction
- **LexiconSource + Compose** (from 1.1) — multi-source lexicon merging
- **Pipeline as interface** (1.2) — customizable pipeline
- **Last Known Good Lexicon** (from 1.1) — atomic hot-swap with fallback
- **15 architectural invariants**
- **20 Coding Agent execution rules**

#### Decisions (D1-D7)

- **D1** — LLM is optional extension, NOT in Standard Preset
- **D2** — `New(...) (*Engine, error)` retains error return (fail-fast)
- **D3** — Result retains all fields: Original / Duration / Steps / Err
- **D4** — Match conflict rule restored: Longest → Priority → Lex
- **D5** — 4 sentinel errors: ErrInvalidConfig / ErrInvalidSpan / ErrConflict / ErrRuntime
- **D6** — Pipeline is interface (allows custom pipelines)
- **D7** — EventType is uint8 enum

#### Standard Processors (order)

```
Normalize → Disfluency → Alias → Deterministic → Pinyin → Fuzzy → Context
```

(LLM is optional extension, not in this chain)

#### HA Architecture

- Engine HA (stateless, multi-instance)
- Lexicon HA (atomic snapshot + last-known-good)
- Request Consistency (V1 snapshot during in-flight requests)
- Processor HA (LLM optional, failure-tolerant)

#### Acceptance Scenarios (M12 hard requirement)

- Scenario A: ASR multi-Change
- Scenario B: Meeting multi-Change
- Scenario C: Protected Span enforcement
- Scenario D: Lexicon hot-swap consistency
- Scenario E: Processor failure degradation
- Scenario F: Multi-Profile concurrent isolation

### Performance Targets (M12)

| Scenario | Target |
|---|---:|
| 45 chars / 2000 entries | < 200 µs / < 32 KB / < 200 allocs |
| 1000 chars / 10000 entries | < 3 ms |

---

## [1.2.0-doc] — 2024-09-04

### Specification

- 🔀 **Merged 1.0 and 1.1** into unified specification `docs/ark-lexnorm-架构设计与开发规范1.2.md`
- 📋 **7 decisions** (D1-D7) explicitly logged
- 🔀 **8 conflicts** (C1-C8) resolved between 1.0 and 1.1
- 📚 **18 split documents** for working baseline

### Conflicts Resolved

| # | 1.0 | 1.1 | **1.2 Resolution** |
|:--:|---|---|---|
| C1 | LLM not in Standard Preset | LLM in recommended order 8 | **Match 1.0: NOT in Standard Preset** (D1) |
| C2 | `lexicon.Lexicon` | `Lexicon` in core | **1.2: `Lexicon` in core** |
| C3 | Result has Original/Duration/Steps/Err | Result has Suggestions/Errors/Runtime | **1.2: All fields coexist** (D3) |
| C4 | Performance baseline with memory/alloc | Only < 3ms | **1.2: Both baselines kept** |
| C5 | Change.Source enum | Change.Source string | **1.2: string** |
| C6 | Change.Kind `Kind` | ChangeKind | **1.2: ChangeKind** |
| C7 | NewEngine returns error | NewEngine no error | **1.2: keeps error return** (D2) |
| C8 | Span "Original byte offset" | Span "UTF-8 byte offset" | **1.2: Original UTF-8 byte offset** |

### Restored from 1.0

- Match conflict rule (Longest → Priority → Lex) → 1.2 D4
- Performance baseline detail (memory / alloc targets)
- Architectural invariants I3/I5/I6/I7/I13/I14/I16 (implicitly merged)

### Restored from 1.1

- Runtime Snapshot abstraction
- ProfileResolver abstraction
- LexiconSource + Compose
- Pipeline as interface
- HA architecture (4 levels)
- Change new fields (RuleID/EntryID/Processor/ProcessorVersion)

---

## [1.1.0-doc] — Historical Archive

> See `docs/ark-lexnorm-架构设计与开发规范1.1.md` (kept as historical archive)

Key innovations:
- Runtime Snapshot / ProfileResolver
- LexiconSource / Compose
- Pipeline interface (1.2 adopts)
- HA architecture (Engine / Lexicon / Request / Processor)
- Result fields: Suggestions / Errors / Runtime

---

## [1.0.0-doc] — Historical Archive

> See `docs/ark-lexnorm-架构设计与开发规范1.0.md` (kept as historical archive)

Key foundations:
- 16 architectural invariants (1.2 merged into 15)
- Performance baselines (1.2 retains)
- Match conflict rule (1.2 restores via D4)
- Detailed Processor specifications

---

## Versioning Notes

| Series | Status | Notes |
|---|---|---|
| **1.0.x** | Historical Archive | Original spec |
| **1.1.x** | Historical Archive | Second iteration |
| **1.2.x** | Current Working Baseline | Merged spec, decisions locked |
| **v1.0.0** | ⏳ Pending | First code release (after M12) |
| **v1.0.0+** | Future | Backward-compatible additions |

API freeze (v1.0):
- `Processor` interface (`Name`, `Process`)
- `Engine.New` signature
- 4 sentinel errors
- `Span` / `Result` / `Change` field semantics

---

## References

- Specification: [`docs/ark-lexnorm-架构设计与开发规范1.2.md`](docs/ark-lexnorm-架构设计与开发规范1.2.md)
- Agent Guide: [`.agents/AGENTS.md`](.agents/AGENTS.md)
- Roadmap: [`docs/17-开发实施路线.md`](docs/17-开发实施路线.md)
- Review Checklist: [`.agents/REVIEW.md`](.agents/REVIEW.md)
