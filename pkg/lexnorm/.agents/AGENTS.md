# ark-lexnorm — Agent 项目指南

> 本文件是 Claude Code、Codex 等 coding agent 进入 ark-lexnorm 仓库根目录的统一入口。
> **修改代码或文档前必须先读本文件。**

`ark-lexnorm` 是一个面向开发者与学习者开放的 **Go 文本词法规范引擎**。开源仓库地址：`github.com/stack-haven/lexnorm`。

---

## 1. 项目定位

ark-lexnorm 是一个面向开发者与学习者开放的 **通用文本词法规范引擎**。核心场景：

- ASR / OCR 结果规范化
- 会议 / 客服录音转写
- 内部人员 / 术语统一
- 搜索关键词归一化
- NLP / Agent 输入预处理
- 文档清洗与术语统一

LLM Refine 是**可选扩展**，**不在 Standard Preset 内**（决策 D1）。

引擎通过可组合 Processor 完成：
**Clean → Disfluency → Alias → Deterministic → Pinyin → Fuzzy → Context**

---

## 2. 仓库结构

```text
github.com/stack-haven/lexnorm               # 当前独立开源仓库
├── CLAUDE.md                                # Claude Code 入口（精简）
├── CLAUDE.md                                # Claude Code 入口（精简）
├── README.md / README.zh-CN.md              # 公共 README（开源后可见）
├── CONTRIBUTING.md                          # 贡献指南
├── LICENSE                                  # Apache-2.0
├── CHANGELOG.md                             # 版本日志
├── go.mod / Makefile                        # Go module + CI 脚本
│
├── .agents/                                 # Agent 配置源（Claude Code + Codex 共用）
│   ├── AGENTS.md                            # 本文件
│   ├── RULES.md                             # 开发与架构规则
│   ├── DESIGN.md                            # 设计原则与产品边界
│   └── REVIEW.md                            # Code Review 检查清单
│
├── docs/                                    # 全部文档（规范 + 拆解）
│   ├── README.md                            # 文档索引 + Agent 阅读协议
│   ├── ark-lexnorm-架构设计与开发规范1.2.md  # 当前权威规范（48 节）
│   ├── ark-lexnorm-架构设计与开发规范1.0.md  # 历史归档（保留）
│   ├── ark-lexnorm-架构设计与开发规范1.1.md  # 历史归档（保留）
│   ├── 00-项目定位与设计原则.md
│   ├── 01-架构总览与包结构.md
│   ├── 02-核心领域模型.md
│   ├── ... (03 ~ 17)
│   └── 17-开发实施路线.md
│
├── *.go                                     # Go 核心 API（errors / processor / pipeline / state ...）
├── internal/interval/                       # Sorted Interval Set（M3）
└── lexicon/                                 # Lexicon interface（M3 声明，M6 实现）
```

---

## 3. 必读入口（顺序不可乱）

### 3.1 必读

1. `.agents/AGENTS.md`（**本文件**）
2. `.agents/RULES.md`
3. `.agents/DESIGN.md`
4. `.agents/REVIEW.md`
5. `docs/README.md`

### 3.2 推荐阅读顺序（实现类任务）

1. `.agents/AGENTS.md`
2. `.agents/RULES.md`
3. `.agents/DESIGN.md`
4. `.agents/REVIEW.md`
5. `docs/README.md`
6. `../docs/ark-lexnorm-架构设计与开发规范1.2.md`（权威规范）
7. `docs/00-项目定位与设计原则.md`
8. `docs/01-架构总览与包结构.md`
9. `docs/17-开发实施路线.md`（**当前阶段 → 目标 M**）
10. 对应 M 的文档（`02 ~ 16`）
11. 现有代码（若有）

### 3.3 文档维护类任务

只需读：

1. `.agents/AGENTS.md`
2. `docs/README.md`
3. `../docs/ark-lexnorm-架构设计与开发规范1.2.md` 头部（含决策日志）

---

## 4. 当前开发状态

### 4.1 文档阶段（**当前**）

- ✅ 1.2 权威规范已发布（48 节）
- ✅ 18 份拆解文档已刷新
- ✅ 决策日志 D1-D7 已收口
- ✅ Agent 规则体系建立（本文件）
- ⏳ **代码实施 M1~M12 待启动**

### 4.2 里程碑概览

| M | 主题 | 状态 |
|:--:|---|:--:|
| M1 | 项目骨架 | ⏳ 待启动 |
| M2 | 核心 Value Objects | — |
| M3 | State | — |
| M4 | Processor / Pipeline | — |
| M5 | Runtime / Engine | — |
| M6 | Lexicon | — |
| M7 | Lexicon Store（HA） | — |
| M8 | 基础 Processor（4 个） | — |
| M9 | 智能匹配 Processor（3 个） | — |
| M10 | Middleware / Hook / Registry / Preset | — |
| M11 | LLM Processor（**可选**） | — |
| M12 | 性能 / HA / 质量门禁 | — |

详细清单见 [`docs/17-开发实施路线.md`](../docs/17-开发实施路线.md) §6。

---

## 5. 中断后恢复协议

按以下顺序恢复上下文：

1. 读本文件 → 确认项目定位
2. 读 [`docs/17-开发实施路线.md`](../docs/17-开发实施路线.md) §6「关键里程碑」 → 定位当前 M
3. 读 `../docs/ark-lexnorm-架构设计与开发规范1.2.md` 头部「决策日志」 → 确认 D1-D7
4. 读当前 M 对应的文档（`02 ~ 16`）
5. 读 `tests/` 与 `*_test.go` → 现有测试覆盖范围
6. 继续当前 M 的开发

**禁止**：
- ❌ 跳过阶段直接进入 M5+ 的开发
- ❌ 并行推进多个 M
- ❌ 不读 `1.2 头部决策日志` 就修改 API

---

## 6. 决策日志摘要（**D1-D7**）

完整决策日志见 `../docs/ark-lexnorm-架构设计与开发规范1.2.md` §0。这里只列摘要：

| ID | 决策 | 影响 |
|:--:|---|---|
| **D1** | LLM 保留为**可选扩展**，不在 Standard Preset | `00 §1`、`12 §9`、`17 M11` |
| **D2** | `New(...) (*Engine, error)` 保留 error 返回 | `07 §2`、`10 §5`、`17 §4.4` |
| **D3** | Result **保留全部字段**：Original / Duration / Steps / Err + 1.1 的 Suggestions / Errors / Runtime | `02 §10`、`17 §4.4` |
| **D4** | Match 冲突规则**从 1.0 补回**：Longest → Priority → Lex 三层 | `04 §5`、`11 §2`、`17 §4.3` |
| D5 | 删 `ErrProcessorNotFound` / `ErrTextTooLarge`；保留 `ErrInvalidConfig`；新增 `ErrInvalidSpan` / `ErrConflict` / `ErrRuntime` | `10 §1` |
| D6 | Pipeline **升级为 interface**（1.0 struct → 1.2 interface） | `04 §2`、`17 M4` |
| D7 | EventType 保留 `uint8` 枚举（1.1 改为 string 后 1.2 回退） | `08 §3` |

**Code Review 必须逐项校验这些决策是否被违反。**

---

## 7. 技术栈

| 维度 | 选型 |
|---|---|
| 语言 | Go 1.22+ |
| 依赖 | **仅标准库**（`go list -deps` 验证） |
| 模块路径 | `github.com/stack-haven/lexnorm` |
| 测试 | `testing` + `-race` + `-fuzz` + `-bench` |
| Lint | `go vet` + 后续可选 `golangci-lint` |
| License | Apache-2.0 |
| 文档 | Markdown，中英双语 |

---

## 8. 开源仓库

仓库地址：`github.com/stack-haven/lexnorm`

发布流程：

```text
github.com/stack-haven/lexnorm  (独立开源仓库)
   ↓ （git tag）
v1.0.0
   ↓ （Go proxy / pkg.go.dev）
公共 API 文档
```

- 当前阶段：已独立仓库，零第三方依赖，CI 全绿
- 下一阶段：v1.0.0 tag → pkg.go.dev 自动索引

---

## 9. 起源背景

本仓库从内部文本规范化基础设施抽离并开源，独立 module，零第三方依赖。

| 维度 | 内部历史上下文 | ark-lexnorm 当前状态 |
|---|---|---|
|---|---|---|
| 配置文件 | avmc 根 `.agents/AGENTS.md` | 本仓库 `.agents/AGENTS.md` |
| 命名规范 | Ark Platform 业务术语 | 通用领域术语（**禁止 ASR/HR/Tenant 等业务词**） |
| 依赖 | kratos / ent / wire / ... | **零第三方** |
| 测试规范 | testify/mock + biz 测试 | `testing` + `-race` + `-fuzz` |

详见 `.agents/DESIGN.md` §2「核心包与父项目的边界」。

---

## 10. 自检清单

进入项目后回答：

- [ ] 是否已读 `RULES.md` / `DESIGN.md` / `REVIEW.md`？
- [ ] 是否已读 `docs/README.md`？
- [ ] 是否确认当前 M 与对应文档？
- [ ] 是否已读 1.2 头部决策日志（D1-D7）？
- [ ] 是否了解 LLM 在 Standard Preset **不可出现**？
- [ ] 是否了解 `New(...)` 必须返回 error？
- [ ] 是否了解 Match 冲突规则是 **Longest → Priority → Lex**？
- [ ] 是否了解核心包**仅依赖 stdlib**？

任一项不通过 → 不可开始写代码。

---

## 11. 相关文档

- 规则：[.agents/RULES.md](RULES.md)
- 设计：[.agents/DESIGN.md](DESIGN.md)
- Review：[.agents/REVIEW.md](REVIEW.md)
- 文档索引：[../docs/README.md](../docs/README.md)
- 权威规范：[../docs/ark-lexnorm-架构设计与开发规范1.2.md](../docs/ark-lexnorm-架构设计与开发规范1.2.md)
- 实施路线：[../docs/17-开发实施路线.md](../docs/17-开发实施路线.md)
