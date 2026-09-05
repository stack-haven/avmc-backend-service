# ark-lexnorm 文本词法规范引擎

## 架构设计与开发规范 1.2（合并权威版）

> **文档定位**：本文件是 `1.0` 与 `1.1` 的合并权威版，作为正式开发的工作基线。
>
> **来源**：
> - `1.0` §1~§81（保留为历史归档，承载 Phase 10 条 Coding Agent 约束、16 架构不变量 I1~I16、10 阶段 Phase 1~10、Match 冲突规则、Benchmark 基线等）
> - `1.1` §1~§63（保留为历史归档，承载 Runtime Snapshot、ProfileResolver、LexiconSource/Compose、高可用体系、M1~M16 里程碑、20 条执行规则等）
>
> **正式开发文档**：`docs/` 目录下 18 份拆分文档（`00-` 至 `17-`）为本规范的工作基线。
>
> **Module**：`github.com/stack-haven/lexnorm`
> **中文名**：文本词法规范引擎
> **简称**：文本规范引擎
> **核心依赖**：仅 Go 标准库
> **目标版本**：v1.2

---

## 🤖 Coding Agent 阅读协议

> **任何 Coding Agent（Claude Code / Codex / 其他）进入本仓库开始任务前必读。**

### 必读文档（顺序不可乱）

1. **入口**：仓库根 `CLAUDE.md`（精简版）
2. **项目指南**：`.agents/AGENTS.md`（详细）
3. **规则**：`.agents/RULES.md`（含 15 架构不变量 + D1-D7 决策约束）
4. **设计**：`.agents/DESIGN.md`（产品边界）
5. **Review 清单**：`.agents/REVIEW.md`（**36 项阻断项，必须逐项核对**）
6. **拆解文档索引**：`docs/README.md`
7. **本规范（1.2）**：当前文件

### 中断后恢复协议

1. 读 `.agents/AGENTS.md` §5「中断后恢复协议」
2. 读 `docs/17-开发实施路线.md` §6「关键里程碑」 → 定位当前 M
3. 读本文件「版本与决策日志」下方表 → 确认 D1-D7
4. 读当前 M 对应文档（`docs/02 ~ 16`）
5. 读现有代码与测试

### 修改本规范的硬约束

- 修改 §42 架构不变量 → 同步更新 `.agents/RULES.md` §2
- 修改决策 → 追加新决策（**D8+**）→ 同步更新 `.agents/RULES.md` §3
- 修改包结构 → 同步更新 `docs/01-架构总览与包结构.md`
- 修改 API 冻结规则 → 同步更新 `.agents/RULES.md` §5

### Code Review 阻断项

详细清单见 `.agents/REVIEW.md`，**合计 36 项阻断项**：

- S 系列（命名）6 项
- A 系列（架构不变量）15 项
- D 系列（决策日志）7 项
- DET 系列（确定性）8 项

---

## 版本与决策日志

---

## 版本与决策日志

### 版本沿革

| 版本 | 日期 | 状态 | 主要变化 |
|---|---|:--:|---|
| 1.0 | 初版 | 历史归档 | 81 节 / 2865 行；建立 Processor / Pipeline / Engine / State / Lexicon 基础模型与 8 步默认流程 |
| 1.1 | 修订版 | 历史归档 | 63 节 / 3062 行；新增 Runtime Snapshot、ProfileResolver、LexiconSource/Compose、HA 体系、20 条执行规则、16 里程碑 |
| **1.2** | **合并权威** | **正式工作基线** | **本文件**：`1.1` 主体 + `1.0` 缺失章节（Match 冲突规则）+ 3 项冲突解决 |

### 1.2 决策记录（与 1.1 的差异）

| ID | 主题 | 1.1 原方案 | 1.2 决议 | 理由 |
|:--:|---|---|---|---|
| D1 | LLM 在默认顺序中的位置 | 列入"推荐默认顺序"第 8 位 | **保留为可选扩展，不列入默认顺序** | 与 1.0 一致；避免误导用户把 LLM 当作引擎必备能力 |
| D2 | `New(...)` 返回值 | `func New(...) *Engine` | **`func New(...) (*Engine, error)`** | 配置校验失败应在构造时立即 fail-fast，不延迟到首次 Normalize |
| D3 | Result 字段 | 移除 Original / Duration / Steps / Err | **全部保留**：`Original`、`Duration`、`Steps` 是有价值元数据；`Err` 作为 `Errors` 的聚合视图并存 | 不丢失审计/性能/可调试性 |
| D4 | Match 冲突规则 | **1.1 完全缺失** | **从 1.0 §63 引入**：Longest Match First → Higher Priority → Stable Lexicographical | 确定性契约不可缺少，否则 Fuzzy 与 Alias 命中重叠时无法复现 |

### 1.2 新增章节（吸收 1.0 缺失）

| ID | 章节 | 来源 |
|:--:|---|---|
| D5 | §22 **Match 冲突规则** | 1.0 §63 |
| D6 | §23 **Match 冲突示例** | 1.0 §63 + 1.1 扩展 |
| D7 | §24 **Result 全字段合并** | 1.0 §28 + 1.1 §24 字段合集 |

---

# 1. 项目定位

`ark-lexnorm` 是一个面向开发者和学习者开放的**通用文本词法规范引擎**。

引擎接收原始文本，通过可组合的 Processor 按确定的处理顺序完成：

- 噪声清洗
- 口语/语气词处理
- 别名归一化
- 确定性纠错
- 拼音/同音纠错
- 模糊纠错
- 上下文纠错

LLM Refine 作为可选扩展，不在 Standard Preset 内（**D1 决策**）。

### 典型应用（**展示场景**，不是定义）

1. ASR 语音识别结果规范化
2. 会议录音转写文本规范化
3. 企业内部名称、组织、术语规范化
4. 搜索关键词规范化
5. NLP/Agent 输入预处理
6. 文档内容清洗与术语统一

ASR、会议记录等属于应用场景，不属于核心领域模型。

---

# 2. 核心设计原则

## 2.1 确定性优先

文本修改风险与知识确定性负相关。Processor 默认按确定性降序执行。

**推荐默认顺序**（**不含 LLM**，D1 决策）：

```text
Normalize
    ↓
Disfluency
    ↓
Alias
    ↓
Deterministic
    ↓
Pinyin
    ↓
Fuzzy
    ↓
Context
```

高确定性处理结果必须能够被低确定性 Processor 识别并保护。

## 2.2 可控

调用方必须能够：

- 启用/禁用 Processor
- 自定义 Processor
- 自定义 Pipeline
- 自定义执行顺序
- 调整 Processor 参数
- 配置 Apply / Suggest / Skip
- 提供初始 Protected Span
- 指定 Profile
- 指定 Lexicon
- 使用不同运行时快照

**不得通过隐藏规则改变用户已经明确配置的执行行为。**

## 2.3 可解释

每一次文本变化都必须能够回答：

- 修改了什么？修改前/后是什么？
- 哪个 Processor 修改的？版本是什么？
- 为什么修改？使用了哪个规则 / 词条？
- 置信度是多少？是否实际应用？
- 本次运行使用了哪个 Profile / Lexicon Snapshot / Pipeline？
- 各 Processor 的版本是什么？

## 2.4 可组合

Processor 是最小能力单元。每个 Processor：

- 可以独立使用
- 可以组合到 Pipeline
- 可以被 Engine 调用
- 可以被用户替换 / 禁用 / 重新排序

**内置 Processor 与用户自定义 Processor 在架构层面没有特殊地位。**

## 2.5 可降级

任何非核心 Processor 发生错误，**不得默认丢失原始文本**：

```text
Processor Error
    ↓
保留当前文本
    ↓
记录 Error / Change / Event
    ↓
Result.Status = Partial
    ↓
继续后续 Processor（如果策略允许）
```

最终业务是否允许将 `Partial` 结果交给下游 Agent / 文档系统等，由业务层决定。

## 2.6 无业务耦合

核心包不得出现：

```text
Tenant / TenantID / UserTenant
ASR / HR / Employee / Meeting / Agent
词库中心 / 企业组织架构
```

这些均属于业务系统。核心包只提供：

```text
Profile / Lexicon / LexiconSource
Processor / Pipeline
State / Result / Runtime
```

业务系统负责把自己的业务上下文转换成上述通用模型。

---

# 3. 总体架构

```text
                    Application
                         │
                         │
              Profile / Runtime Snapshot
                         │
                         ▼
                    ┌─────────┐
                    │  Engine │
                    └────┬────┘
                         │
                         ▼
                    ┌─────────┐
                    │ Pipeline│
                    └────┬────┘
                         │
          ┌──────────────┼──────────────┐
          ▼              ▼              ▼
      Processor       Processor       Processor
          │              │              │
          └──────────────┼──────────────┘
                         ▼
                       State
                         │
          ┌──────────────┼──────────────┐
          ▼              ▼              ▼
        Text          Lexicon        Config
                         │
                         ▼
                    Snapshot
                         │
                         ▼
                       Result
```

### 核心职责边界

| 组件      | 核心职责                   |
| --------- | -------------------------- |
| Engine    | 运行入口、运行时协调       |
| Pipeline  | Processor 编排             |
| Processor | 单一规范化能力             |
| State     | 单次执行过程中的工作状态   |
| Profile   | 规范化上下文标识           |
| Lexicon   | 规范化知识快照             |
| Result    | 最终执行结果               |
| Change    | 文本变更记录               |
| Runtime   | 一次运行所绑定的不可变快照 |

---

# 4. 核心对象模型

## 4.1 Processor

Processor 是引擎最小执行单元。

```go
type Processor interface {
    Name() string
    Process(ctx context.Context, s *State) error
}
```

**要求**：

1. 无隐式全局状态
2. 可以独立运行
3. 可以被 Pipeline 组合
4. 可以被 Engine 调用
5. 不得依赖具体业务系统
6. 不得直接修改 State 内部字段
7. 文本修改必须通过 `state.Replace`
8. 建议支持 Processor Version（用于结果追溯，详见 §22）

## 4.2 Pipeline

Pipeline 是 Processor 的组合器，同时本身也是 Processor。

```go
type Pipeline interface {
    Processor
    Processors() []Processor
}

type pipeline struct {
    processors []Processor
}
```

**Pipeline 必须**：

- 顺序明确
- 可裁剪
- 可插入
- 可替换
- 可独立执行
- 不依赖 Engine

> **1.2 升级**：1.0 中 Pipeline 是 struct（具体类型），1.1/1.2 升级为 interface，允许用户实现自定义 Pipeline（如条件分支、并行执行）。

## 4.3 Engine

Engine 是 Facade。

```go
type Engine struct {
    // immutable runtime configuration
}

// 1.2 决议 D2：保留 error 返回
func New(opts ...Option) (*Engine, error)

func (e *Engine) Normalize(
    ctx context.Context,
    text string,
    opts ...CallOption,
) (Result, error)
```

**Engine 必须满足**：

- 并发安全
- 长生命周期
- 无请求级可变状态
- 无业务租户状态
- 无用户状态
- 可以水平扩展
- 可以被多个请求并发调用

## 4.4 Runtime Snapshot

一次 Normalize 调用必须绑定一个完整的运行时快照。

```go
type Runtime struct {
    Profile           Profile
    Lexicon           Lexicon
    Pipeline          Pipeline
    Config            Config
    ProfileVersion    string
    LexiconVersion    string
    PipelineVersion   string
    ProcessorVersions map[string]string
}
```

**Runtime 在一次请求开始时确定。**

请求执行过程中：

```text
Resolve Runtime
      ↓
固定 Runtime Snapshot
      ↓
State 创建
      ↓
Processor 依次执行
      ↓
Result
```

**不得出现**：

```text
Processor A 使用 Lexicon V1
Processor B 使用 Lexicon V2
```

> 同一次 Normalize 调用必须使用一致的 Snapshot。

## 4.5 Profile

```go
type ProfileID string

type Profile struct {
    ID      ProfileID
    Version string
}
```

Profile 不代表 Tenant。Profile 可以表达：

- 不同业务场景
- 不同规范化规则
- 不同术语集合
- 不同 Processor Pipeline
- 不同处理策略

业务系统可以建立：

```text
业务上下文
    ↓
ProfileID
    ↓
Runtime Snapshot
    ↓
ark-lexnorm
```

核心包**不负责 Profile 的持久化和权限**。

## 4.6 ProfileResolver

为了支持同一个进程中同时服务多个 Profile，不要求每个 Profile 建立独立 Engine。

```go
type ProfileResolver interface {
    Resolve(
        ctx context.Context,
        id ProfileID,
    ) (Runtime, error)
}
```

Resolver 可以由业务系统实现。核心包不得自行根据用户身份推断 Profile。

---

# 5. State

State 是单次执行的可变工作空间。**不允许跨请求共享**。

```go
type State struct {
    // private
}
```

### 公开方法（1.2 升级：Replace/Suggest/Lock 改为 error 返回）

```go
func (s *State) Text() string
func (s *State) Lexicon() Lexicon
func (s *State) Config() Config

func (s *State) Replace(span Span, to string, meta ChangeMeta) error
func (s *State) Suggest(span Span, to string, meta ChangeMeta) error
func (s *State) Lock(span Span) error
func (s *State) IsLocked(span Span) bool
```

> **1.2 与 1.1 一致**：移除 1.0 的 `Original() / Profile() / Changes()` 三个方法。
> `Original` 信息保留在 `Result.Original` 字段（**D3 决策**）；`Profile` 在 `Runtime` 中；`Changes` 通过 `Result.Changes` 暴露。

所有文本修改必须通过 `State.Replace`，禁止 `strings.ReplaceAll` / `regexp.ReplaceAllString` 作为 Processor 的最终修改入口。

`Replace` 统一维护：Span · UTF-8 offset · Protected Span · Change · Processor · Rule · Confidence · Provenance。

---

# 6. Span

```go
type Span struct {
    Start int
    End   int
}
```

区间：`[Start, End)`。

**Start / End 使用 UTF-8 字节偏移**（**1.2 决议：相对于 Original**，吸收 1.0 §23 严格表述）。

所有 Processor 必须遵守统一 Span 约定。

---

# 7. Protected Span

Protected Span 用于防止高确定性结果被低确定性 Processor 再次修改。

### 示例

```text
原始：
小田今天参加会议

Alias Processor：
小田 → 田华

结果：
田华今天参加会议

Lock：
[0, 6)
```

后续 Fuzzy Processor 不得将 `田华` 再次修改。

### 保护机制必须支持

- 初始保护
- Processor 产生保护
- Replace 后级联保护
- 区间重叠判断
- 查询是否锁定

---

# 8. Change

Change 是文本规范化的核心审计对象。

```go
type Change struct {
    Span             Span
    From             string
    To               string
    Action           Action
    Kind             ChangeKind        // 1.2 重命名: Kind → ChangeKind
    Source           string            // 1.2 决议: 1.0 枚举 → 1.1 string
    RuleID           string            // 1.1 新增
    EntryID          string            // 1.1 新增
    Processor        string            // 1.1 新增
    ProcessorVersion string            // 1.1 新增
    Confidence       float64
    Applied          bool
    Reason           string
}
```

Change 必须能够表达：

```text
建议修改 (Applied=false, Action=ActionSuggest)
实际修改 (Applied=true, Action=ActionReplace/ActionRemove)
跳过修改 (不产生 Change 记录)
```

---

# 9. Decision

```text
Apply
Suggest
Skip
```

| Decision | Action | Applied |
|---|---|:--:|
| Apply | `ActionReplace` / `ActionRemove` | `true` |
| Suggest | `ActionSuggest` | `false` |
| Skip | — | — |

```go
type Decision int

const (
    DecisionSkip Decision = iota
    DecisionSuggest
    DecisionApply
)
```

具体策略由 Processor / Config 决定。

---

# 10. Action

```go
type Action uint8

const (
    ActionReplace Action = iota
    ActionRemove
    ActionSuggest
)
```

未来新增 Action 必须保持向后兼容：只能追加，不重用已弃用编号。

---

# 11. Lexicon

Lexicon 是规范化知识的只读快照。

```go
type Lexicon interface {
    Entry(id EntryID) (Entry, bool)
    Lookup(text string) (Entry, bool)
    Relations(text string) []Relation
    All(yield func(Entry) bool)
    Matcher() *Matcher
    Len() int
    Version() string
}
```

Lexicon 不负责：权限 / 用户 / 租户 / HR API / 数据同步 / 数据库存储。

---

# 12. LexiconSource

不同业务系统可以提供不同来源的规范化知识。

```go
type LexiconSource interface {
    Version() string
    Entries(yield func(Entry) bool)
    Relations(yield func(Relation) bool)
}
```

来源可以是：平台标准数据 + 业务系统数据 + 用户配置数据 + 外部系统同步数据。

> 核心包不定义具体业务名称。

---

# 13. Lexicon Composition

```go
func Compose(sources ...LexiconSource) (Lexicon, error)
```

### 组合过程

```text
Sources
 ↓
Entry Merge
 ↓
Relation Merge
 ↓
冲突检查
 ↓
ID Index / Exact Index / Aho-Corasick / Pinyin Index / Fuzzy Index / Relation Index
 ↓
Immutable Snapshot
```

**冲突必须显式处理，禁止静默覆盖。**

---

# 14. Lexicon Builder

构建过程中生成：ID Index / Exact Match Index / Alias Index / Aho-Corasick / Pinyin Index / N-Gram Index / Fuzzy Index / Relation Index。

构建完成后：`Builder → Immutable Lexicon`，运行阶段禁止修改。

---

# 15. Lexicon Store 与热更新

```go
type Store struct {
    current atomic.Pointer[Lexicon]
}
```

### 更新流程

```text
External Source
      ↓
Build
      ↓
Validate
      ↓
New Lexicon Snapshot
      ↓
Atomic Swap
      ↓
New Requests → V2
Old Requests → V1
```

### 要求

1. 构建失败不能影响当前版本
2. 校验失败不能替换
3. 替换必须原子完成
4. 正在执行的请求继续使用旧 Snapshot
5. 新请求使用新 Snapshot
6. **不允许请求中途切换 Snapshot**

---

# 16. Version 管理

| 对象 | Version 字段 | 用途 |
|---|---|---|
| Profile | `Profile.Version` | 业务场景配置版本 |
| Lexicon | `Lexicon.Version()` | 知识版本 |
| Pipeline | `Pipeline.Version` | 流程版本 |
| Processor | `Processor.Version()` | 实现版本 |
| Result | `RuntimeInfo` 聚合 | 完整可重放快照 |

确保同一文本可以在历史环境中重现处理链。

---

# 17. Result（1.2 合并字段，D3 决策）

```go
type Result struct {
    Text        string           // 规范化后文本
    Original    string           // 原始文本（1.0 保留）
    Status      Status
    Changes     []Change         // 全部 Change（Applied + Suggested + Skipped）
    Suggestions []Change         // Applied=false 的 Change（独立字段，1.1 新增）
    Errors      []error          // 全部 Processor 错误（1.1 新增）
    Err         error            // errors.Join(Errors...) 聚合（1.0 保留）
    Duration    time.Duration    // 总耗时（1.0 保留）
    Steps       []StepTiming     // 每步耗时（1.0 保留，确定顺序、零分配）
    Runtime     RuntimeInfo      // 完整运行快照（1.1 新增）
}

type Status int

const (
    StatusSuccess Status = iota
    StatusPartial
    StatusCanceled
    StatusFailed         // 1.1 新增
)

type RuntimeInfo struct {
    ProfileID         ProfileID
    ProfileVersion    string
    LexiconVersion    string
    PipelineVersion   string
    ProcessorVersions map[string]string
}
```

### Result 必须能够完整说明

```text
输入
 ↓
使用什么 Profile
 ↓
使用什么 Lexicon
 ↓
使用什么 Pipeline
 ↓
使用哪些 Processor（含版本）
 ↓
发生了什么 Change
 ↓
最终文本
 ↓
最终状态
```

---

# 18. Status 语义

| Status | 含义 |
|---|---|
| `StatusSuccess` | 所有必要 Processor 正常完成 |
| `StatusPartial` | 部分 Processor 失败，但仍产生可用结果 |
| `StatusCanceled` | 调用被 Context 取消 |
| `StatusFailed` | 无法形成有效结果（如基础错误：Lexicon 缺失、Runtime 解析失败） |

**默认原则**：单个 Processor Failure → `Partial`；但对于无法继续运行的基础错误 → `Failed`。

---

# 19. 默认 Processor

| Processor | 确定性 |
|---|:--:|
| Normalize | Deterministic |
| Disfluency | Deterministic |
| Alias | Deterministic |
| Deterministic | Deterministic |
| Pinyin | High |
| Fuzzy | Medium |
| Context | Medium |
| LLM Refine | Unknown（**可选扩展，不在 Standard Preset 内**，D1 决策） |

---

# 20. Certainty 与 Order

```go
type Certainty int

const (
    CertaintyHigh Certainty = iota
    CertaintyMedium
    CertaintyLow
)
```

> **1.2 决议**：1.0 的 `CertaintyUnknown` 与 `CertaintyDeterministic` 两档被取消，统一为 3 档（与 1.1 一致）。

> **Certainty 不得覆盖用户显式配置的 Pipeline Order。**

```text
显式 Pipeline Order > 默认推荐 Order > Certainty 推导
```

---

# 21. Preset

Preset 是标准 Pipeline 模板，例如：`PresetDefault` / `PresetHighAccuracy` / `PresetFast`。

Preset ≠ Engine 内置行为。用户选择 Preset 后仍然可以：

- 删除 Processor
- 添加 Processor
- 修改 Processor
- 调整顺序

---

# 22. Match 冲突规则（**D4 决议，从 1.0 §63 引入**）

**1.2 重大补回**：1.1 完全缺失此章节，本节从 1.0 §63 引入，作为确定性契约的强制部分。

当多个候选在同一位置命中时，必须按以下顺序消解冲突：

### 默认规则：最长匹配优先

```text
Longest Match First
```

### 相同长度：优先级最高者优先

```text
Higher Priority
```

### 仍相同：稳定字典序

```text
Stable Lexicographical Ordering
```

### 冲突示例

```text
Text:    "张三丰张"
Lexicon: [
    {Canonical: "张三丰", Priority: 1.0, Order: "1"},
    {Canonical: "张三",   Priority: 1.0, Order: "2"},
]

Result:
  命中 [0, 6) → "张三丰"   ← Longest 优先
  不命中 [6, 9) → "丰张"（或继续匹配）
```

```text
Lexicon: [
    {Canonical: "周丽群", Priority: 1.0, Order: "1"},
    {Canonical: "周丽群", Priority: 1.0, Order: "2"},  // 优先级相同
]

Result:
  优先 Order = "1"（Stable Lexicographical）
```

### 禁止

- 依赖 `map iteration order`
- 依赖 `goroutine scheduling`
- 使用 `math/rand` 作为 tie-breaker

---

# 23. Match 冲突示例（1.2 扩展）

### 多 LexiconSource 冲突

当 `Compose(SourceA, SourceB)` 时：

```text
Source A: 小田 → 田华 (Priority: 1.0)
Source B: 小田 → 田强 (Priority: 1.0)

→ 冲突！
→ Build 阶段返回 ErrConflict
→ 禁止静默覆盖
```

业务层必须明确优先级策略（如 Source 顺序、Pinned Version 锁定）。

---

# 24. Registry

```go
type Descriptor struct {
    Name      string
    Certainty Certainty
    New       func(cfg json.RawMessage) (Processor, error)
    Default   func() any
}
```

Registry 不是 Processor 执行的必要条件。第三方 Processor 不要求修改核心代码。

---

# 25. Middleware

```go
type Handler func(
    ctx context.Context,
    s *State,
) error

type Middleware func(Handler) Handler
```

### 推荐 Middleware

```text
Recover
Timing
Timeout
Hooks
```

Middleware 不负责业务逻辑。

---

# 26. Recover Middleware

```text
Processor Panic
 ↓
Recover
 ↓
记录 Error
 ↓
保持当前文本
 ↓
Status = Partial
 ↓
继续 / 终止由策略决定
```

Panic 必须转换为普通 Error。

---

# 27. Hook

```go
type Event struct {
    Type      string
    Processor string
    Duration  time.Duration
    Changes   []Change
    Err       error
    Result    Result
}

type Hook interface {
    OnEvent(context.Context, Event)
}
```

**Hook 不得改变核心执行结果。**

---

# 28. 配置

- 必须显式校验；禁止非法值静默 Clamp
- 必须可序列化（JSON / YAML）
- 核心包使用 `encoding/json`，**不得引入第三方配置依赖**

---

# 29. 错误体系

```go
var (
    ErrInvalidConfig = errors.New("lexnorm: invalid config")
    ErrInvalidSpan   = errors.New("lexnorm: invalid span")
    ErrConflict      = errors.New("lexnorm: conflict")
    ErrRuntime       = errors.New("lexnorm: runtime resolution failed")
)
```

支持 `errors.Is` / `errors.As`。**不得引入 Kratos Error。**

> **1.2 决议**：取消 1.0 的 `ErrProcessorNotFound` / `ErrTextTooLarge`；保留 1.0 的 `ErrInvalidConfig`；新增 1.1 的 `ErrInvalidSpan` / `ErrConflict` / `ErrRuntime`。

---

# 30. 文本修改原则

所有 Processor 必须：

```text
发现候选
 ↓
计算 Span
 ↓
检查 Protected Span
 ↓
计算 Decision
 ↓
State.Replace / Suggest
 ↓
记录 Change
```

禁止 `strings.ReplaceAll()` / `regexp.ReplaceAllString()` 作为 Processor 的最终修改入口。这些工具可用于候选计算，但不能绕过 State。

---

# 31. 多 Profile 并发模型

```text
                 Engine
                    │
       ┌────────────┼────────────┐
       ▼            ▼            ▼
   Profile A    Profile B    Profile C
       │            │            │
   Snapshot A   Snapshot B   Snapshot C
```

要求：

- Engine 不保存请求级 Profile 状态
- State 不跨请求共享
- Runtime 不可变
- Lexicon 不可变
- Pipeline 推荐不可变
- Config 不可变

---

# 32. 多业务上下文隔离

核心包不实现业务授权：

```text
请求
 ↓
身份认证
 ↓
业务授权
 ↓
确定 Profile
 ↓
加载 Runtime
 ↓
ark-lexnorm
```

**禁止** `ark-lexnorm` 读取用户身份、自行判断权限。规范化引擎只负责文本处理，不负责业务安全边界。

---

# 33. 外部系统数据接入

业务层负责转换：

```text
External System
      ↓
Adapter
      ↓
LexiconSource
      ↓
Lexicon Builder
      ↓
Immutable Lexicon
```

核心包不得直接依赖外部 API。

---

# 34. 高可用架构

### 34.1 Engine HA

Engine 无状态，任意实例可处理任意请求。

### 34.2 Lexicon HA

使用 Immutable Snapshot + Atomic Swap。外部数据源不可用时继续使用最后有效 Snapshot；新版本构建失败则保留 V1。

### 34.3 Request Consistency

一次请求锁定 Runtime V1；即使中途发布 Lexicon V2，本请求仍使用 V1；下一请求才使用 V2。

### 34.4 Processor HA

LLM 不可用时降级为 Partial 或 Skip；具体策略由业务层 Processor 决定。

---

# 35. 故障矩阵

| 故障                   | 行为                     |
| ---------------------- | ------------------------ |
| Processor Error        | Partial                  |
| Processor Panic        | Recover → Partial       |
| Lexicon Build Error    | 保持旧版本               |
| Lexicon Validate Error | 保持旧版本               |
| Snapshot Swap          | 原子切换                 |
| 外部数据源不可用       | 使用最后有效 Snapshot    |
| Context Cancel         | Canceled                 |
| LLM 不可用             | Partial 或 Skip          |
| Pipeline 配置非法      | 启动/构建阶段失败        |
| Runtime 不存在         | 返回明确错误             |
| Lexicon 冲突           | Build 失败或显式策略处理 |

---

# 36. 性能目标

| 场景 | 目标 |
|---|---:|
| 45 字 / 2000 词条 | **< 200 µs** |
| 内存 | **< 32 KB** |
| allocations | **< 200** |
| 1000 字 / 10000 词条 | **< 3 ms** |

通过 Aho-Corasick / Exact Index / N-Gram / ID Index / Immutable Lexicon / Interval Set / 单次文本扫描 / sync.Pool 实现。

**性能指标属于目标值，必须通过 Benchmark 验证，不得作为未经测试的实现承诺。**

---

# 37. 并发安全要求

```bash
go test -race ./...
```

- Engine 可并发
- Lexicon 可并发读
- Pipeline 可并发
- State 不共享
- Result 不共享
- Runtime Snapshot 不可变

禁止：请求 A 修改 Lexicon 时请求 B 同时读取 Lexicon。

---

# 38. Object Pool

对高频临时对象可以使用 `sync.Pool`：State / Change Buffer / Candidate Buffer / Match Buffer。

**必须保证**：请求结束后完整 Reset，再放回 Pool。禁止残留 Profile / Lexicon / Text / Change / Error / Protected Span。

---

# 39. 包结构

```text
ark-lexnorm/
├── engine.go            ├── option.go            ├── call_option.go
├── runtime.go           ├── profile.go           ├── state.go
├── result.go            ├── change.go            ├── span.go
├── decision.go          ├── status.go            ├── processor.go
├── pipeline.go          ├── middleware.go        ├── hook.go
├── registry.go          ├── preset.go            ├── config.go
├── errors.go
│
├── lexicon/
│   ├── lexicon.go       ├── builder.go           ├── source.go
│   ├── compose.go       ├── matcher.go           ├── store.go
│   └── index.go
│
├── processor/
│   ├── normalize/       ├── disfluency/          ├── alias/
│   ├── deterministic/   ├── pinyin/              ├── fuzzy/
│   ├── context/         └── llm/
│
├── internal/
│   ├── interval/        ├── text/                ├── pool/
│   └── ...
│
├── examples/            ├── testdata/            └── docs/
```

> **1.2 决议**：1.0 的独立 `match/` 子包被合并到 `internal/`，与 1.1 一致。匹配算法（Levenshtein / N-Gram / Replace / IntervalSet）属于内部实现，对外不暴露。

---

# 40. 测试规范

| 类型 | 覆盖 |
|---|---|
| Unit | Span / Replace / Lock / Decision / Processor / Pipeline / Lexicon / Runtime / Result |
| Golden | Input → Expected Output |
| Benchmark | 延迟 / 内存 / Allocation |
| Fuzz | Unicode / Span / Replace / Alias / Fuzzy / Context |
| Race | `go test -race ./...` |
| Integration | Engine + Runtime + Lexicon + Pipeline + Processor |

---

# 41. 业务场景验收测试

### 场景 A：ASR

```text
输入：小田今天帮我查一下个种子的情况
Lexicon: 小田 → 田华 / 个种子 → 颗种籽
期望：田华今天帮我查一下颗种籽的情况
Change 数：2
```

### 场景 B：会议

```text
输入：张总说让老王和小田明天处理市场部的事情
Lexicon: 张总 → 张强 / 老王 → 王强 / 小田 → 田华
期望：张强说让王强和田华明天处理市场部的事情
Change 数：3
```

### 场景 C：保护

Alias Processor 完成 `小田 → 田华` 后锁定 `[0, 6)`。后续 Fuzzy Processor 不得修改 `田华`。

### 场景 D：Lexicon 热更新

请求 A 锁定 Lexicon V1；中途发布 V2；请求 A 仍用 V1；请求 B 用 V2。

### 场景 E：Processor 故障

Context Processor 失败 → Previous Text 保留 + Status = Partial，不返回空文本。

### 场景 F：多 Profile 并发

请求 A → Profile A，请求 B → Profile B；两个请求不得读取对方 Runtime / Lexicon / Config。

---

# 42. 架构不变量（15 条，1.2 合并版）

| # | 不变量 | 来源 |
|:--:|---|---|
| 1 | Processor 可以独立运行 | 1.1 不变量 1 |
| 2 | Pipeline 本身是 Processor | 1.1 不变量 2 |
| 3 | Engine 不承载业务状态 | 1.1 不变量 3 |
| 4 | State 不跨请求共享 | 1.1 不变量 4 |
| 5 | Lexicon Runtime 只读 | 1.1 不变量 5 |
| 6 | 文本修改只能通过 State.Replace | 1.1 不变量 6 |
| 7 | Protected Span 必须阻止后续非法覆盖 | 1.1 不变量 7 |
| 8 | 一次请求必须使用一致 Runtime Snapshot | 1.1 不变量 8 |
| 9 | 相同 Input + 相同 Snapshot 应产生确定性结果 | 1.1 不变量 9 |
| 10 | Processor 失败不能默认导致原始文本丢失 | 1.1 不变量 10 |
| 11 | Profile 不等于 Tenant | 1.1 不变量 11 |
| 12 | 核心包不得依赖业务系统 | 1.1 不变量 12 |
| 13 | 外部 Lexicon 数据必须经过 Builder 才能进入运行时 | 1.1 不变量 13 |
| 14 | Lexicon 更新必须原子发布 | 1.1 不变量 14 |
| 15 | 旧 Snapshot 必须支持正在执行的请求完成 | 1.1 不变量 15 |

**1.2 决议**：与 1.1 一致（15 条）。**1.0 的 I3 / I5 / I6 / I7 / I8 / I13 / I14 / I16 已被合并或隐含**（详见 `17-开发实施路线.md` §3）。

---

# 43. 里程碑（吸收 1.0 的 10 Phase + 1.1 的 16 M，1.2 合并为 12 Milestone）

| M | 主题 | 内容 | 验收 |
|:--:|---|---|---|
| M1 | 项目骨架 | go.mod / License / README / package layout / CI / 基础错误 / 基础测试 | `go test ./...` + `go vet ./...` |
| M2 | 核心 Value Objects | Span / Profile / Decision / Status / Change / RuntimeInfo / Result | 单测全绿 |
| M3 | State | Text / Replace / Suggest / Lock / IsLocked / Protected Span / Change tracking | UTF-8 / Span 边界 / 多次 Replace / 重叠 Replace / Protected Span / Replace 后 Offset |
| M4 | Processor / Pipeline | Processor / Pipeline / Pipeline Builder / Pipeline Version / Processor Version | Processor / Pipeline 独立可执行 |
| M5 | Runtime / Engine | Runtime / Engine / New（返回 error）/ Normalize / Option / CallOption / WithRuntime / WithProfile / WithLexicon / WithPipeline | Engine 并发 / 多 Profile 并发 / Runtime 隔离 / Context Cancel |
| M6 | Lexicon | Entry / Variant / Relation / Lexicon / LexiconSource / Builder / Compose / Matcher / Version / Exact / ID / Aho-Corasick | LexiconSource 多源合并 |
| M7 | Lexicon Store | Immutable Snapshot / Atomic Swap / Last Known Good / Version / Build Validate / Publish | V1→V2 / 请求中切换 / 并发读写 / 失败回滚 |
| M8 | 基础 Processor | Normalize / Disfluency / Alias / Deterministic | 单测 + Benchmark |
| M9 | 智能匹配 Processor | Pinyin / Fuzzy / Context（支持 Apply/Suggest/Skip 三档） | 单测 + Benchmark + Fuzz |
| M10 | Middleware / Hook / Registry / Preset | Recover / Timing / Timeout / Hook / Event / Descriptor / Registry / Preset | Registry 不强依赖 |
| M11 | LLM Processor（可选扩展） | 只定义 Processor 接口；提供 Example | 不进 Standard Preset |
| M12 | 性能 / 高可用 / 质量门禁 | Aho-Corasick / Fuzzy / N-Gram / Span / Pool / Engine HA / Lexicon HA / Request Consistency / Processor HA | Benchmark 达标 + 故障矩阵验证 |

---

# 44. Coding Agent 执行规则（1.2 合并 20 条）

1. 先实现核心抽象，再实现具体 Processor。
2. 每完成一个 M 必须保证测试通过。
3. 不得为了实现具体业务而向核心包增加 Tenant、ASR、HR、Employee 等概念。
4. 不得绕过 `State.Replace` 直接修改文本。
5. 不得让 Processor 直接持有可变 Lexicon。
6. 不得让请求共享 State。
7. 不得让请求过程中切换 Runtime Snapshot。
8. 不得使用全局可变业务状态。
9. 不得为了方便引入第三方核心依赖。
10. 新增公开 API 必须同步增加 Example 或测试。
11. 修改核心接口必须检查所有 Processor、Pipeline、Engine 和测试。
12. 性能优化必须有 Benchmark 数据支撑。
13. 任何并发实现必须通过 Race Test。
14. 任何 Snapshot 发布必须先 Build、Validate，再 Atomic Swap。
15. 任何 Processor Failure 必须明确处理策略，不得静默吞错。
16. 所有可观测信息必须通过 Result / Change / Hook 等机制暴露。
17. 不得将业务系统的数据源直接注入核心执行链路。
18. **Runtime 是一次请求的一致性边界**。
19. **Lexicon Snapshot 是知识一致性边界**。
20. **Profile 是规范化上下文标识，不承担业务权限职责**。

---

# 45. 公开 API 稳定性

```go
type Processor interface
type Pipeline interface         // 1.2 升级: interface 化
type Lexicon interface
type LexiconSource interface    // 1.1 新增
type ProfileResolver interface  // 1.1 新增
type Profile
type Runtime
type Result
type Change
type Span
```

**避免暴露**：内部索引结构 / Matcher 内部节点 / State 内部文本缓存 / Interval Tree / Object Pool / Atomic 实现细节。

---

# 46. 最终架构结论

`ark-lexnorm` 的核心模型最终确定为：

```text
Profile
    │
    ▼
Runtime Snapshot
    │
    ├── Lexicon
    ├── Pipeline
    └── Config
            │
            ▼
          Engine
            │
            ▼
         Pipeline
            │
            ▼
        Processor
            │
            ▼
           State
            │
            ├── Text
            ├── Protected Span
            └── Change
            │
            ▼
          Result
            │
            ├── Text / Original / Status
            ├── Changes / Suggestions / Errors / Err
            ├── Duration / Steps
            └── RuntimeInfo
```

该架构必须同时满足：

```text
通用 / 可组合 / 可扩展 / 可解释 / 可控
确定性 / 可降级 / 并发安全 / Snapshot 一致性
多 Profile 隔离 / Lexicon 原子热更新 / 水平扩展 / 零业务耦合
```

---

# 47. 拆解文档映射

本规范的**正式工作基线**为 `docs/` 目录下 18 份拆分文档。每份文档与本规范的章节对应关系：

| 拆解文档 | 1.2 章节 | 主题 |
|---|---|---|
| `00-项目定位与设计原则.md` | §1, §2, §46 | 项目定位 / 8 原则 / 最终架构 |
| `01-架构总览与包结构.md` | §3, §39, §45 | 总体架构 / 包结构 / API 稳定性 |
| `02-核心领域模型.md` | §4, §10, §20 | 核心对象 / Action / Certainty |
| `03-Processor接口与生命周期.md` | §4.1 | Processor 契约 |
| `04-Pipeline与执行顺序.md` | §4.2, §21 | Pipeline / Preset |
| `05-State与保护区机制.md` | §5, §6, §7 | State / Span / Protected Span |
| `06-Lexicon与热更新.md` | §11~§15, §34 | Lexicon 全栈 |
| `07-Engine与Profile.md` | §4.3~§4.6 | Engine / Profile / Resolver / Runtime |
| `08-横切能力Middleware与Hook.md` | §25, §26, §27 | Middleware / Recover / Hook |
| `09-Registry与动态装配.md` | §24 | Registry / Descriptor |
| `10-配置校验与错误体系.md` | §28, §29 | Config / Errors |
| `11-确定性与匹配冲突消解.md` | §22, §23, §36 | **Match 冲突规则（1.2 补回）** / Determinism |
| `12-内置Processor规范.md` | §19, §34.4 | 7 内置 + LLM |
| `13-应用场景与Pipeline模板.md` | §41 | 业务场景 |
| `14-性能设计与算法优化.md` | §36, §37, §38 | 性能 / 并发 / Pool |
| `15-测试策略与质量工程.md` | §37, §40 | 测试矩阵 |
| `16-开源工程治理.md` | §39, §45 | 开源工程 |
| `17-开发实施路线.md` | §42, §43, §44 | 里程碑 / 不变量 / 执行规则 |

---

# 48. 与历史版本的关系

| 文件 | 状态 | 保留原因 |
|---|---|---|
| `ark-lexnorm-架构设计与开发规范1.0.md` | 历史归档 | 保留 81 节原始论述，供对比追溯 |
| `ark-lexnorm-架构设计与开发规范1.1.md` | 历史归档 | 保留 63 节原始论述，供对比追溯 |
| `ark-lexnorm-架构设计与开发规范1.2.md`（本文件） | **正式工作基线** | 1.1 主体 + 1.0 Match 规则 + 3 项冲突解决 |

`docs/` 目录下 18 份拆解文档的内容基线 = 本文件 (1.2)。

---

**下一步**：按本规范刷新 `docs/00-` 至 `docs/17-` 共 18 份文档，确保内容、引用、字段一致。
