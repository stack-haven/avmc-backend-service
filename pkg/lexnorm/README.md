# ark-lexnorm

> A zero-dependency, deterministic, composable Go library for text normalization.

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8)](https://go.dev)

---

## Overview

`ark-lexnorm` is a **general-purpose text normalization engine** that processes raw text through a composable chain of small, well-defined processing units.

It is designed for scenarios such as:

- **ASR transcript normalization** — cleaning up speech-to-text output
- **OCR text correction** — fixing recognition errors
- **Meeting / customer service transcription** — unifying terminology
- **Search query normalization** — improving recall and precision
- **NLP / Agent input preprocessing** — cleaning before downstream tasks
- **Document cleaning** — terminology standardization

The engine is intentionally **domain-neutral**: it does not assume ASR, HR, CRM, or any specific business context. Domain-specific knowledge is plugged in via `LexiconSource`, and the normalization flow is fully customizable via `Pipeline`.

---

## Features

- ✅ **Composable** — every `Processor` is a small unit that can be used independently
- ✅ **Deterministic** — same input + same snapshot always produces the same output
- ✅ **Concurrent-safe** — multiple goroutines can share an `Engine` instance
- ✅ **Hot-reloadable Lexicon** — atomic snapshot swap with last-known-good fallback
- ✅ **Observable** — full `Result.RuntimeInfo` (Profile / Lexicon / Pipeline / Processor versions)
- ✅ **Multi-Profile** — serve different normalization contexts from a single engine
- ✅ **Zero dependencies** — `go list -deps` output contains only the Go standard library
- ✅ **Open source** — Apache-2.0

---

## Architecture

```
                  ┌──────────────┐
                  │   Engine     │  ← Facade
                  │   Runtime    │
                  └──────┬───────┘
                         ▼
                  ┌──────────────┐
                  │  Pipeline    │  ← interface (1.2)
                  └──────┬───────┘
                         │
            ┌────────────┼────────────┐
            ▼            ▼            ▼
       Processor A  Processor B  Processor C
            │            │            │
            └────────────┼────────────┘
                         ▼
                       State
                         │
              ┌──────────┼──────────┐
              ▼          ▼          ▼
           Lexicon    Profile    Config
                         │
                         ▼
                       Result
```

### Built-in Processors (Standard Order)

```
Normalize → Disfluency → Alias → Deterministic → Pinyin → Fuzzy → Context
```

> **Note**: LLM Refine is available as an **optional extension** (`processor/llm`), but is **not** part of the standard preset. See [`docs/12-内置Processor规范.md`](docs/12-内置Processor规范.md) for details.

---

## Installation

```bash
go get github.com/stack-haven/lexnorm
```

> **Status**: Pre-release. Currently under active development.

---

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "github.com/stack-haven/lexnorm"
    "github.com/stack-haven/lexnorm/lexicon"
    "github.com/stack-haven/lexnorm/processor/normalize"
    "github.com/stack-haven/lexnorm/processor/alias"
    "github.com/stack-haven/lexnorm/processor/deterministic"
)

func main() {
    // 1. Build Lexicon
    lex, _ := lexicon.Build(
        lexicon.WithEntries([]lexicon.Entry{
            {ID: "1", Text: "田华", Variants: []lexicon.Variant{
                {Text: "小田", Kind: lexicon.VariantAlias, Confidence: 1.0},
            }},
            {ID: "2", Text: "颗种籽", Variants: []lexicon.Variant{
                {Text: "个种子", Kind: lexicon.VariantCorrection, Confidence: 1.0},
            }},
        }),
    )

    // 2. Build Pipeline
    pipeline := lexnorm.NewPipeline(
        normalize.New(),
        alias.New(lex),
        deterministic.New(),
    )

    // 3. Create Engine
    engine, err := lexnorm.New(
        lexnorm.WithLexicon(lex),
        lexnorm.WithPipeline(pipeline),
    )
    if err != nil {
        panic(err)
    }

    // 4. Normalize
    result, err := engine.Normalize(context.Background(), "呃，小田明天帮我查一下个种子的情况")
    if err != nil {
        panic(err)
    }

    fmt.Println(result.Text)
    // Output: 田华明天帮我查一下颗种籽的情况

    fmt.Println(result.Runtime.LexiconVersion)  // Audit trail
    fmt.Println(result.Runtime.PipelineVersion)
}
```

---

## Documentation

- 📘 **[`docs/README.md`](docs/README.md)** — Document index and reading paths
- 🏛 **[`docs/ark-lexnorm-架构设计与开发规范1.2.md`](docs/ark-lexnorm-架构设计与开发规范1.2.md)** — Authoritative specification (Chinese, 48 sections)
- 🤖 **[`.agents/AGENTS.md`](.agents/AGENTS.md)** — Coding agent entry point
- 🚀 **[`example/`](example/)** — 8 runnable beginner programs (Quick Start by example)

### Reading Paths

| Role | Read |
|---|---|
| **Core developer** | `00 → 01 → 02 → 03 → 04 → 05 → 06 → 10 → 11 → 17` |
| **Business developer** | `00 → 02 → 04 → 12 → 13 → 17` |
| **Extension developer** | `00 → 02 → 03 → 04 → 09 → 11 → 17` |

---

## Contributing

See [`CONTRIBUTING.md`](CONTRIBUTING.md). All contributions must:

1. Pass all CI checks (`go test -race ./...`, `go vet ./...`)
2. Comply with **15 architectural invariants** (see [`.agents/REVIEW.md`](.agents/REVIEW.md))
3. Not introduce business-specific terms (`tenant`, `ASR`, `HR`, etc.)
4. Not add third-party dependencies

---

## License

Apache License 2.0 — see [`LICENSE`](LICENSE).

Copyright 2024 The Ark Authors.

---

## Status

| Version | Status | Notes |
|---|---|---|
| v1.0.0 | ⏳ In progress | Documentation phase complete; M1 code implementation pending |

---

## Related Projects

`ark-lexnorm` is extracted from **stack-haven/avmc-backend-service** as a standalone open-source package.
