# Contributing to ark-lexnorm

> Thank you for your interest in contributing!
> This guide covers development workflow, code standards, and submission process.

---

## 📚 Required Reading (in order)

1. **[`.agents/AGENTS.md`](.agents/AGENTS.md)** — Project overview and entry point
2. **[`.agents/RULES.md`](.agents/RULES.md)** — Naming, architecture invariants, decisions (D1-D7)
3. **[`.agents/REVIEW.md`](.agents/REVIEW.md)** — Code Review checklist (36 blocking items)
4. **[`docs/README.md`](docs/README.md)** — 18 split documents

**Do not start coding before reading these documents.**

---

## 🚀 Quick Start

### Prerequisites

- Go 1.22 or later
- Git
- `gofmt` / `goimports` (usually bundled with Go)

### Setup

```bash
git clone https://github.com/stack-haven/lexnorm
cd lexnorm
go test ./...
go test -race ./...
go vet ./...
```

All three must pass before any PR.

---

## 🎯 Development Workflow

### 1. Pick a Task

Check [`docs/17-开发实施路线.md`](docs/17-开发实施路线.md) §6 for current milestone (M1-M12).

Tasks follow this lifecycle:

```
[ ] Planned → [~] In Progress → [x] Complete
```

### 2. Create a Branch

```bash
git checkout -b <scope>/<type>-<short-desc>
# Examples:
git checkout -b engine/feat-new-engine-error
git checkout -b processor/feat-llm-extension
git checkout -b docs/update-1.3-spec
```

### 3. Implement

Follow the rules in [`.agents/RULES.md`](.agents/RULES.md):

- **15 architectural invariants** (Section §2)
- **7 decisions** D1-D7 (Section §3)
- **Zero dependencies** (Section §4)
- **Naming** (Section §1): no `tenant` / `ASR` / `HR` / etc.

### 4. Test

```bash
# Required
go test ./...
go test -race ./...

# Recommended for new APIs
go test -fuzz=FuzzXxx ./...          # Fuzz
go test -bench=BenchmarkXxx ./...   # Benchmark

# Coverage
go test -coverprofile=cover.out ./...
go tool cover -func=cover.out | grep total
```

Targets:

- Core packages: ≥ 90% coverage
- Overall: ≥ 80% coverage

### 5. Lint

```bash
go vet ./...
gofmt -l .                            # No output = OK
```

### 6. Document

For any change to public API, decisions, or invariants:

- Update corresponding `docs/0X-*.md`
- Update `docs/ark-lexnorm-架构设计与开发规范1.X.md` if it changes a spec
- Add `CHANGELOG.md` entry
- Add GoDoc to exported symbols

### 7. Commit

Follow [`.agents/RULES.md` §9](.agents/RULES.md):

```
<scope>(<type>): <subject>

<body>

<footer>
```

**Type**: `feat` / `fix` / `refactor` / `docs` / `test` / `chore` / `perf`

**Example**:

```
engine(feat): implement New() with error return (D2)

- New(opts ...Option) (*Engine, error)
- Construct-time validation: ErrInvalidConfig on illegal config
- Documented in docs/07-Engine与Profile.md §2

Refs: D2
```

### 8. Submit PR

PR description must include:

- Summary of changes
- Linked decisions (if any): `Refs: D2`
- Linked milestone: `Refs: M5`
- Test results

---

## ✅ PR Checklist

The following **must all** be checked:

### Blocking Items

- [ ] **S01** — No business terms (`tenant` / `ASR` / `OCR` / `HR` / `Employee` / `Meeting` / `Agent` / `Document`)
- [ ] **A01-A15** — All 15 architectural invariants respected
- [ ] **D1-D7** — All 7 decisions respected (or new decision D8+ proposed)
- [ ] **DET01-DET08** — Determinism rules respected
- [ ] `go test ./...` passes
- [ ] `go test -race ./...` passes
- [ ] `go vet ./...` passes

### Quality Items

- [ ] New APIs have GoDoc + Example
- [ ] New APIs accepting string have Fuzz
- [ ] Core Processors have Benchmark
- [ ] Coverage ≥ 80% (core ≥ 90%)
- [ ] Documentation updated
- [ ] `CHANGELOG.md` updated

Full checklist: [`.agents/REVIEW.md`](.agents/REVIEW.md)

---

## 🏛 Architecture Invariants (15)

Quick reference — see [`.agents/RULES.md` §2](.agents/RULES.md) for full text.

1. Processor can run independently
2. Pipeline implements Processor interface
3. Engine does not hold business state
4. State not shared across requests
5. Lexicon runtime is read-only
6. Text modification only via `State.Replace/Suggest`
7. Protected Span blocks later overwrites
8. One request uses one consistent Runtime Snapshot
9. Same Input + Same Snapshot → deterministic Result
10. Processor failure does not lose original text
11. Profile ≠ Tenant
12. Core package has no business dependencies
13. External Lexicon data must go through Builder
14. Lexicon updates are atomic
15. Old Snapshot supports in-flight requests

---

## 📋 Decisions (D1-D7)

Quick reference — see [`.agents/RULES.md` §3](.agents/RULES.md) for full text.

| ID | Decision | Reversible? |
|:--:|---|:--:|
| **D1** | LLM not in Standard Preset | Requires D8+ |
| **D2** | `New(...) (*Engine, error)` | Requires D8+ |
| **D3** | Result retains all fields (Original/Duration/Steps/Err) | Requires D8+ |
| **D4** | Match conflict: Longest → Priority → Lex | Requires D8+ |
| D5 | 4 sentinel errors | Requires D8+ |
| D6 | Pipeline is interface | Requires D8+ |
| D7 | EventType is uint8 enum | Requires D8+ |

> ⚠️ Changing any D1-D7 default requires a new decision (D8+) and must be documented in `docs/ark-lexnorm-架构设计与开发规范1.2.md` §0.

---

## 🚫 What We Don't Accept

- ❌ Introducing third-party dependencies
- ❌ Breaking v1.0 API (Processor interface, error sentinels, Span/Result fields)
- ❌ Adding business-specific terms to core packages
- ❌ Using `tenant` / `ASR` / `OCR` / `HR` / `Employee` / `Customer` / `Meeting` / `Agent` / `Document`
- ❌ Map iteration participating in output (violates determinism)
- ❌ `math/rand` for tie-breaking (violates determinism)
- ❌ `time.Sleep` / goroutine scheduling for control flow (violates determinism)
- ❌ State shared across goroutines (violates invariant 4)
- ❌ Panic for normal errors (use `error` returns)

---

## 💬 Communication

- **Issues**: GitHub Issues
- **Discussions**: GitHub Discussions
- **Security**: See [`SECURITY.md`](SECURITY.md) (TBD)

---

## 📜 License

By contributing, you agree that your contributions will be licensed under **Apache License 2.0**.

---

## 🙏 Acknowledgments

`ark-lexnorm` is extracted from `stack-haven/avmc-backend-service` (internal infrastructure project).
Thank you to all internal contributors who made this open-source release possible.
