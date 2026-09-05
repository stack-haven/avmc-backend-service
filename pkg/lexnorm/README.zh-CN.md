# ark-lexnorm

> 零依赖、确定性、可组合的 Go 文本规范化引擎。

[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8)](https://go.dev)

[English Version](README.md)

---

## 项目定位

`ark-lexnorm` 是一个**通用文本规范化引擎**，通过可组合的处理单元链处理原始文本。

适用场景：

- **ASR 转写文本规范化** — 清理语音识别结果
- **OCR 文本纠错** — 修正识别错误
- **会议 / 客服录音转写** — 统一术语
- **搜索关键词归一化** — 提升召回与精度
- **NLP / Agent 输入预处理** — 下游任务前清洗
- **文档清洗** — 术语标准化

引擎**领域中立**：不假设 ASR、HR、CRM 等特定业务上下文。业务知识通过 `LexiconSource` 注入，规范化流程通过 `Pipeline` 完全可定制。

---

## 核心特性

- ✅ **可组合** — 每个 `Processor` 都是独立单元，可单独使用
- ✅ **确定性** — 相同输入 + 相同快照始终产生相同输出
- ✅ **并发安全** — 多个 goroutine 可共享同一 `Engine` 实例
- ✅ **Lexicon 热更新** — 原子快照切换 + Last Known Good 回退
- ✅ **可观测** — 完整 `Result.RuntimeInfo`（Profile / Lexicon / Pipeline / Processor 版本）
- ✅ **多 Profile** — 单一引擎支持多个规范化上下文
- ✅ **零依赖** — `go list -deps` 输出仅含 Go 标准库
- ✅ **开源** — Apache-2.0

---

## 架构

```
                  ┌──────────────┐
                  │   Engine     │  ← 对外 Facade
                  │   Runtime    │
                  └──────┬───────┘
                         ▼
                  ┌──────────────┐
                  │  Pipeline    │  ← interface（1.2 决议）
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

### 内置 Processor（标准顺序）

```
Normalize → Disfluency → Alias → Deterministic → Pinyin → Fuzzy → Context
```

> **注意**：LLM Refine 作为**可选扩展**（`processor/llm`）提供，**不在** Standard Preset 内。详见 [`docs/12-内置Processor规范.md`](docs/12-内置Processor规范.md)。

---

## 安装

```bash
go get github.com/stack-haven/lexnorm
```

> **状态**：发布前。当前处于活跃开发阶段。

---

## 快速开始

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
    // 1. 构建 Lexicon
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

    // 2. 构建 Pipeline
    pipeline := lexnorm.NewPipeline(
        normalize.New(),
        alias.New(lex),
        deterministic.New(),
    )

    // 3. 创建 Engine
    engine, err := lexnorm.New(
        lexnorm.WithLexicon(lex),
        lexnorm.WithPipeline(pipeline),
    )
    if err != nil {
        panic(err)
    }

    // 4. 规范化
    result, err := engine.Normalize(context.Background(), "呃，小田明天帮我查一下个种子的情况")
    if err != nil {
        panic(err)
    }

    fmt.Println(result.Text)
    // 输出: 田华明天帮我查一下颗种籽的情况

    fmt.Println(result.Runtime.LexiconVersion)  // 审计追溯
    fmt.Println(result.Runtime.PipelineVersion)
}
```

---

## 文档

- 📘 **[`docs/README.md`](docs/README.md)** — 文档索引与阅读路径
- 🏛 **[`docs/ark-lexnorm-架构设计与开发规范1.2.md`](docs/ark-lexnorm-架构设计与开发规范1.2.md)** — 权威规范（48 节）
- 🤖 **[`.agents/AGENTS.md`](.agents/AGENTS.md)** — Coding Agent 入口

### 阅读路径

| 角色 | 阅读顺序 |
|---|---|
| **核心开发者** | `00 → 01 → 02 → 03 → 04 → 05 → 06 → 10 → 11 → 17` |
| **业务开发者** | `00 → 02 → 04 → 12 → 13 → 17` |
| **扩展开发者** | `00 → 02 → 03 → 04 → 09 → 11 → 17` |

---

## 贡献

详见 [`CONTRIBUTING.md`](CONTRIBUTING.md)。所有贡献必须：

1. 通过所有 CI 检查（`go test -race ./...`, `go vet ./...`）
2. 符合 **15 条架构不变量**（见 [`.agents/REVIEW.md`](.agents/REVIEW.md)）
3. **不引入**业务术语（`tenant`、`ASR`、`HR` 等）
4. **不引入**第三方依赖

---

## 许可证

Apache License 2.0 — 见 [`LICENSE`](LICENSE)。

Copyright 2024 The Ark Authors。

---

## 状态

| 版本 | 状态 | 说明 |
|---|---|---|
| v1.0.0 | ⏳ 进行中 | 文档阶段完成；M1 代码实施待启动 |

---

## 相关项目

`ark-lexnorm` 抽取自 **stack-haven/avmc-backend-service** 内部的文本规范化基础设施。
