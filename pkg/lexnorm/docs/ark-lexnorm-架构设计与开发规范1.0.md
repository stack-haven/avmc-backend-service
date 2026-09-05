# ark-lexnorm 文本词法规范引擎

## 架构设计与开发规范

> Module：`github.com/stack-haven/lexnorm`
> 中文名：文本词法规范引擎
> 简称：文本规范引擎
> 定位：面向 Go 生态的开源、可组合、可扩展、确定性的文本词法规范基础设施
> 核心依赖：仅 Go 标准库
> 目标版本：v1.0

---

# 1. 项目目标

`ark-lexnorm` 是一个通用文本词法规范工具包。

它不定义具体业务，只提供：

* 文本规范能力；
* Processor 扩展机制；
* Pipeline 编排能力；
* Profile 规范上下文；
* Lexicon 规范知识；
* State 运行状态；
* Result 可解释结果；
* Protection 保护机制；
* Decision 决策分级；
* Middleware 横切能力；
* Hook 观察能力；
* Registry 动态装配能力；
* Preset 标准流程模板。

核心目标：

> **规范能力由 `ark-lexnorm` 提供，规范流程由开发者决定。**

用户既可以使用单个 Processor，也可以组合多个 Processor；既可以使用内置 Processor，也可以自行实现 Processor。

---

# 2. 核心设计原则

以下原则属于项目架构不变量，开发过程中不得随意违反。

## 2.1 Processor Independence

每一个 Processor 都必须能够独立执行。

Processor：

* 不依赖 Engine；
* 不依赖 Pipeline；
* 不依赖具体业务；
* 不要求 Registry；
* 不要求 Preset；
* 可以单独测试；
* 可以单独被调用。

内置 Processor 与用户自定义 Processor 遵循完全一致的接口契约。

---

## 2.2 Composition over Inheritance

通过组合 Processor 构造复杂规范能力，而不是通过继承构造规范体系。

```text
Processor
    ↓
Pipeline
    ↓
Engine
```

Pipeline 是 Processor 的组合器。

---

## 2.3 Explicit over Implicit

优先采用显式设计：

* 显式 Processor；
* 显式 Pipeline；
* 显式 Profile；
* 显式 Lexicon；
* 显式配置；
* 显式错误策略；
* 显式 LLM；
* 显式保护机制。

禁止依赖隐式全局状态完成核心业务行为。

---

## 2.4 Deterministic

在以下条件一致时：

```text
Input
Profile
Lexicon Version
Processor Version
Processor Config
Pipeline Order
```

必须得到可复现结果。

---

## 2.5 Controlled

文本规范按照确定性进行分层。

Standard Preset 默认采用：

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

LLM 属于可选扩展能力。

需要特别明确：

> Standard Preset 的顺序是默认策略，不是框架强制顺序。

用户显式构建 Pipeline 时，以用户指定顺序为准。

---

## 2.6 Explainable

所有实际发生的文本修改都必须能够追溯：

```text
修改位置
原始文本
目标文本
Processor
Action
Kind
Source
Confidence
Reason
是否实际应用
```

---

## 2.7 Degradable

默认情况下，单个 Processor 执行失败不能导致原始文本丢失。

默认错误策略：

```text
Processor Error
    ↓
记录错误
    ↓
继续后续 Processor
    ↓
Result.Status = Partial
```

支持显式切换：

```text
FailFast
```

---

## 2.8 Zero Business Dependency

核心库不得出现业务领域概念。

禁止核心代码依赖：

```text
tenant
租户
ASR
OCR
CRM
HR
企业
客户
订单
业务数据库
业务权限
```

使用：

```text
Profile
Lexicon
Term
Variant
Processor
Pipeline
State
Result
```

表达通用能力。

---

# 3. 总体架构

```text
┌──────────────────────────────────────────────┐
│                 Application                  │
│                                              │
│ ASR / OCR / Search / CRM / Document / Other │
└──────────────────────┬───────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────┐
│                    Engine                    │
│                                              │
│ Runtime / Profile / Lexicon / Pipeline       │
│ Middleware / Hook / Concurrency / Lifecycle │
└──────────────────────┬───────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────┐
│                   Pipeline                   │
│                                              │
│ Processor → Processor → Processor → ...      │
└──────────────────────┬───────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────┐
│                  Processor                   │
│                                              │
│ Built-in Processor / Custom Processor        │
└──────────────────────┬───────────────────────┘
                       │
                       ▼
┌──────────────────────────────────────────────┐
│                    State                     │
│                                              │
│ Text / Profile / Lexicon / Config            │
│ Protection / Changes / Runtime Metadata       │
└──────────────────────────────────────────────┘
```

核心关系：

```text
Processor
    ▲
    │
Pipeline
    ▲
    │
Engine
```

其中 Pipeline 本身也是 Processor。

---

# 4. 核心对象模型

## 4.1 Processor

最小规范能力单元。

```go
type Processor interface {
    Name() string
    Process(context.Context, *State) error
}
```

---

## 4.2 Pipeline

多个 Processor 的组合器。

```go
type Pipeline struct {
    // private
}

func NewPipeline(processors ...Processor) *Pipeline

func (p *Pipeline) Name() string

func (p *Pipeline) Process(
    context.Context,
    *State,
) error

func (p *Pipeline) Processors() []Processor
```

Pipeline 必须实现 Processor。

---

## 4.3 Engine

整个运行体系的 Facade。

Engine 负责：

* 生命周期；
* Profile；
* Lexicon；
* Pipeline；
* Middleware；
* Hook；
* 并发安全；
* 热更新；
* Result。

Engine 不负责实现具体规范算法。

```go
type Engine struct {
    // private
}

func New(opts ...Option) (*Engine, error)

func (e *Engine) Normalize(
    ctx context.Context,
    text string,
    opts ...CallOption,
) (Result, error)
```

---

## 4.4 Profile

Profile 表示一套规范运行上下文的具名身份。

例如：

```text
default
asr
ocr
search
document
development
production
```

Profile 不是 Tenant 的改名。

它是通用的运行上下文标识。

Profile 不负责：

* 数据库存储；
* 权限；
* 用户；
* 生命周期；
* 多租户。

---

## 4.5 Lexicon

Lexicon 是规范知识来源。

```go
type Lexicon interface {
    Entry(id EntryID) (Entry, bool)
    Lookup(text string) (Entry, bool)
    Relations(text string) []Relation
    All(func(Entry) bool)
    Matcher() *Matcher
    Len() int
    Version() string
}
```

Lexicon 不负责：

* 数据库存储；
* 同步；
* 权限；
* API；
* 多租户；
* 生命周期。

---

## 4.6 State

State 是单次规范运行的工作状态。

```go
type State struct {
    // private
}
```

State 是 Request Scoped 对象。

同一个 State 不允许被多个 Goroutine 并发修改。

---

## 4.7 Result

Result 是规范执行最终结果。

```go
type Result struct {
    Text      string
    Original  string
    Changes   []Change
    Status    Status
    Duration  time.Duration
    Steps     []StepTiming
    Err       error
}
```

---

# 5. Processor 规范

## 5.1 接口

```go
type Processor interface {
    Name() string
    Process(context.Context, *State) error
}
```

该接口必须保持极简。

---

## 5.2 Processor 生命周期

Processor 应尽量保持无状态。

推荐：

```text
Processor
    ↓
Read State
    ↓
Calculate
    ↓
State.Replace / Suggest
    ↓
Return
```

禁止：

```text
Processor
    ↓
修改 Engine
    ↓
修改 Pipeline
    ↓
修改共享 Lexicon
```

---

## 5.3 Processor 不得直接修改文本

禁止：

```go
strings.ReplaceAll(...)
```

直接改变 State 外部文本。

所有修改必须通过：

```go
state.Replace(...)
```

所有建议必须通过：

```go
state.Suggest(...)
```

---

# 6. Processor 独立运行

Processor 必须支持脱离 Engine、Pipeline 单独使用。

```go
p := alias.New(lexicon)

state := lexnorm.NewState(
    text,
    lexnorm.WithProfile("default"),
    lexnorm.WithLexicon(lexicon),
)

err := p.Process(ctx, state)

result := state.Result()
```

这一能力属于正式 API 契约，而不是示例功能。

---

# 7. Pipeline

Pipeline 是 Composite Processor。

```text
Pipeline
 ├── Processor A
 ├── Processor B
 ├── Processor C
 └── Processor D
```

执行：

```text
State
 ↓
Processor A
 ↓
Processor B
 ↓
Processor C
 ↓
Processor D
 ↓
Result
```

Pipeline 不应该知道 Processor 的具体业务含义。

---

# 8. 自定义 Pipeline

用户完全控制流程。

例如：

```go
pipeline := lexnorm.NewPipeline(
    clean.New(),
    alias.New(lexicon),
    fuzzy.New(config),
)
```

只使用一个 Processor：

```go
pipeline := lexnorm.NewPipeline(
    alias.New(lexicon),
)
```

甚至：

```go
pipeline := lexnorm.NewPipeline(
    fuzzy.New(config),
)
```

不要求必须运行完整 Standard Preset。

---

# 9. 自定义 Processor

开发者只需要实现 Processor：

```go
type MyProcessor struct{}

func (p *MyProcessor) Name() string {
    return "my-processor"
}

func (p *MyProcessor) Process(
    ctx context.Context,
    state *lexnorm.State,
) error {

    // custom normalization logic

    return nil
}
```

直接加入：

```go
pipeline := lexnorm.NewPipeline(
    clean.New(),
    &MyProcessor{},
    fuzzy.New(config),
)
```

不得要求用户修改核心 Engine。

---

# 10. Engine

推荐使用方式：

```go
engine, err := lexnorm.New(
    lexnorm.WithProfile("default"),
    lexnorm.WithLexicon(lexicon),
    lexnorm.WithPipeline(pipeline),
)

if err != nil {
    return err
}

result, err := engine.Normalize(
    ctx,
    "呃，佘丽群明天开会",
)
```

Engine 只负责运行环境。

---

# 11. Engine 与 Pipeline 的职责边界

Engine：

```text
Runtime
Profile
Lexicon
Pipeline
Middleware
Hook
Concurrency
Lifecycle
```

Pipeline：

```text
Processor Composition
Execution Order
Processor Execution
```

Processor：

```text
Specific Normalization Logic
```

不得混淆三者职责。

---

# 12. Preset

Preset 是标准 Pipeline 模板。

Preset 不是 Engine 的硬编码流程。

例如：

```go
preset.Standard()
```

返回：

```text
Normalize
Disfluency
Alias
Deterministic
Pinyin
Fuzzy
Context
```

用户可以：

```go
pipeline := lexnorm.NewPipeline(
    preset.Standard()...,
)
```

也可以完全不用 Preset。

---

# 13. Standard Pipeline

标准流程：

```text
1. Normalize
2. Disfluency
3. Alias
4. Deterministic
5. Pinyin
6. Fuzzy
7. Context
```

LLM：

```text
Optional Processor
```

不作为核心依赖。

---

# 14. Certainty

Processor 可以声明确定性级别。

```go
type CertaintyLevel uint8

const (
    CertaintyUnknown CertaintyLevel = iota
    CertaintyLow
    CertaintyMedium
    CertaintyHigh
    CertaintyDeterministic
)
```

Certainty 表达：

> Processor 的规范结果具有什么程度的确定性。

Certainty 不等于 Pipeline Order。

---

# 15. Order

Pipeline 顺序由 Pipeline 显式决定。

例如：

```go
NewPipeline(
    A,
    B,
    C,
)
```

必须：

```text
A → B → C
```

Certainty 可以辅助 Standard Preset 设计，但不能覆盖用户明确指定的顺序。

原则：

> **Explicit Pipeline Order > Automatic Ordering**

---

# 16. Profile

Profile 推荐模型：

```text
Profile
 ├── Lexicon
 ├── Pipeline
 ├── Config
 ├── Decision Policy
 └── Protection Policy
```

但这些对象不应该强制全部封装进 Profile 类型。

Profile 本身只承担 identity / context 语义。

---

# 17. Lexicon

Lexicon 表示规范知识。

核心实体：

```text
Term
Variant
Relation
Rule
Policy
```

不要在 Lexicon API 中引入具体业务概念。

---

# 18. Lexicon Builder

```go
builder := lexicon.NewBuilder()

builder.
    AddEntry(...).
    AddRelation(...)

lexicon, err := builder.Build()
```

Build 阶段完成：

```text
Entry ID Index
Aho-Corasick Matcher
n-gram Index
Relation Index
Deterministic Ordering
```

构建完成后：

> Runtime Read Only。

---

# 19. Lexicon Snapshot

Lexicon 必须支持不可变 Snapshot。

运行期：

```text
Request
   ↓
Lexicon Snapshot
   ↓
Read Only
```

更新：

```text
New Lexicon
     ↓
Build
     ↓
Atomic Swap
```

旧请求继续使用旧 Snapshot。

新请求使用新 Snapshot。

---

# 20. State

State 公开能力：

```go
func (s *State) Text() string

func (s *State) Original() string

func (s *State) Profile() Profile

func (s *State) Lexicon() lexicon.Lexicon

func (s *State) Config() Config

func (s *State) Replace(
    span Span,
    to string,
    meta ChangeMeta,
)

func (s *State) Suggest(
    span Span,
    to string,
    meta ChangeMeta,
)

func (s *State) Lock(span Span)

func (s *State) IsLocked(span Span) bool

func (s *State) Changes() []Change
```

State 内部字段必须保持私有。

---

# 21. Replace

`Replace` 是文本规范一致性的核心入口。

所有 Processor 的文本修改必须经过：

```go
state.Replace(...)
```

它统一负责：

```text
Span
Protection
Change
Offset
Decision
Provenance
```

这样避免各 Processor 重复实现文本修改逻辑。

---

# 22. Protected Span

保护区：

```text
Protected Span
```

语义：

> 已确定的文本区域，不允许低确定性 Processor 继续修改。

例如：

```text
原文：
佘丽群明天开会

Alias：
佘丽群 → 周丽群

Lock：
[0, 9)
```

后续：

```text
Fuzzy
Context
LLM
```

不得覆盖该区域。

---

# 23. Span

```go
type Span struct {
    Start int
    End   int
}
```

采用半开区间：

```text
[Start, End)
```

位置统一采用：

> Original UTF-8 字节偏移。

---

# 24. Change

```go
type Change struct {
    Span       Span
    From       string
    To         string
    Action     Action
    Kind       Kind
    Source     Source
    Confidence float64
    Applied    bool
    Reason     string
}
```

Change 必须能够区分：

```text
Applied
Suggested
Skipped
```

---

# 25. Action

使用枚举，不使用自由字符串。

```go
type Action uint8

const (
    ActionReplace Action = iota
    ActionRemove
    ActionSuggest
)
```

未来新增 Action 必须保持向后兼容。

---

# 26. Decision

标准决策分级：

```text
Apply
Suggest
Skip
```

### Apply

满足自动规范条件。

### Suggest

有较强候选，但不满足自动修改条件。

### Skip

置信度或条件不足，不处理。

---

# 27. Confidence

Confidence 使用：

```go
float64
```

约定：

```text
0.0 ≤ Confidence ≤ 1.0
```

不得在配置非法时静默 Clamp。

非法配置应该返回 Error。

---

# 28. Result

Result：

```go
type Result struct {
    Text      string
    Original  string
    Changes   []Change
    Status    Status
    Duration  time.Duration
    Steps     []StepTiming
    Err       error
}
```

状态：

```go
type Status uint8

const (
    StatusSuccess Status = iota
    StatusPartial
    StatusCanceled
)
```

提供：

```go
func (r Result) Applied() []Change
func (r Result) Suggestions() []Change
```

---

# 29. Middleware

Middleware 用于横切能力。

```go
type Handler func(
    context.Context,
    *State,
) error

type Middleware func(Handler) Handler
```

典型 Middleware：

```text
Recover
Timing
Timeout
WithHooks
```

例如：

```go
lexnorm.WithMiddleware(
    lexnorm.Recover(),
    lexnorm.Timing(),
)
```

Processor 不负责：

* panic recovery；
* timing；
* tracing；
* metrics；
* logging。

---

# 30. Hook

Hook 是观察机制。

```go
type Hook interface {
    OnEvent(context.Context, Event)
}
```

Event：

```go
type Event struct {
    Type      EventType
    Processor string
    Duration  time.Duration
    Changes   int
    Err       error
    Result    *Result
}
```

事件：

```text
PipelineStart
ProcessorStart
ProcessorEnd
PipelineEnd
```

Hook 不得修改 State。

---

# 31. Registry

Registry 是可选动态装配机制。

它不是 Processor 使用前提。

普通代码：

```go
pipeline := lexnorm.NewPipeline(
    clean.New(),
    alias.New(lexicon),
)
```

不需要 Registry。

Registry 主要用于：

```text
JSON
YAML
动态配置
插件发现
服务端装配
```

---

# 32. Descriptor

```go
type Descriptor struct {
    Name      string
    Certainty CertaintyLevel
    New       func(json.RawMessage) (Processor, error)
}
```

Registry：

```go
type Registry struct {
    // private
}

func NewRegistry() *Registry

func (r *Registry) Register(
    descriptor Descriptor,
) error

func (r *Registry) Create(
    name string,
    config json.RawMessage,
) (Processor, error)
```

---

# 33. Registry 设计原则

禁止：

```text
Registry → Processor → Registry
```

Registry 只能负责：

```text
Name
 ↓
Descriptor
 ↓
Processor
```

Processor 本身不依赖 Registry。

---

# 34. LLM

LLM 是普通 Processor。

核心库不得绑定具体模型 SDK。

例如：

```go
type LLMProcessor struct {
    client Client
}
```

使用：

```go
pipeline := lexnorm.NewPipeline(
    clean.New(),
    alias.New(lexicon),
    fuzzy.New(config),
    llm.New(client),
)
```

核心库不负责：

* API Key；
* 模型选择；
* Token 成本；
* Prompt 管理；
* 模型供应商；
* 模型服务生命周期。

---

# 35. 内置 Processor

核心 Processor：

```text
processor/
├── clean/
├── disfluency/
├── alias/
├── deterministic/
├── pinyin/
├── fuzzy/
└── context/
```

LLM：

```text
processor/llm/
```

作为扩展能力。

---

# 36. Normalize Processor

负责基础文本归一。

典型能力：

```text
控制字符
不可见字符
异常空白
重复标点
Unicode 基础归一
```

确定性：

```text
Deterministic
```

---

# 37. Disfluency Processor

负责处理不流畅文本成分。

例如：

```text
呃
额
嗯
啊
那个
然后
```

规则必须可配置。

不得把具体业务规则硬编码为核心知识。

---

# 38. Alias Processor

负责：

```text
Variant → Canonical
```

例如：

```text
Variant A
Variant B
Variant C
        ↓
Canonical
```

数据来自 Lexicon。

---

# 39. Deterministic Processor

负责明确的确定性映射：

```text
From → To
```

例如：

```text
个种籽 → 颗种籽
```

Processor 本身不理解业务语义。

---

# 40. Pinyin Processor

负责同音、拼音近似规范。

依赖：

```text
Lexicon
Pinyin Index
```

结果必须能够提供：

```text
Confidence
Source
Reason
```

---

# 41. Fuzzy Processor

推荐执行：

```text
Candidate Filtering
        ↓
n-gram
        ↓
Length Filter
        ↓
Edit Distance
        ↓
Threshold
        ↓
Decision
```

禁止对整个 Lexicon 进行无条件暴力匹配。

---

# 42. Context Processor

根据上下文规则进行规范。

输入：

```text
Context
Candidate
Rule
```

输出：

```text
Apply
Suggest
Skip
```

属于推断型 Processor。

---

# 43. 典型 Standard 流程

```text
Original Text
      │
      ▼
Normalize
      │
      ▼
Disfluency
      │
      ▼
Alias
      │
      ▼
Deterministic
      │
      ▼
Pinyin
      │
      ▼
Fuzzy
      │
      ▼
Context
      │
      ▼
Result
```

每一步都可以：

* 启用；
* 禁用；
* 替换；
* 插入；
* 删除；
* 自定义。

---

# 44. ASR 场景

ASR 是重点应用场景，但不是 `ark-lexnorm` 的定义。

典型：

```text
ASR Result
    ↓
ark-lexnorm
    ↓
Clean
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
    ↓
Normalized Text
```

核心库不得出现 ASR 专用 API。

---

# 45. OCR 场景

OCR 可以使用：

```text
Normalize
Deterministic
Pinyin
Fuzzy
```

并通过自定义 Processor 增加：

```text
形近字
版面相关规则
领域术语
```

---

# 46. Search 场景

搜索 query 可以使用：

```text
Normalize
Alias
Deterministic
Pinyin
Fuzzy
```

并根据 Search 场景自定义 Pipeline。

---

# 47. 文档术语规范场景

文档检查可以：

```text
Normalize
Alias
Deterministic
Pinyin
Fuzzy
```

同时：

```text
不自动 Apply
```

而是：

```text
全量 Suggest
```

因此 Processor 与 Pipeline 必须允许不同 Decision Policy。

---

# 48. 包结构

最终目录：

```text
github.com/stack-haven/lexnorm/
│
├── go.mod
├── LICENSE
├── README.md
├── README.zh-CN.md
├── CHANGELOG.md
├── CONTRIBUTING.md
├── SECURITY.md
├── CODE_OF_CONDUCT.md
│
├── doc.go
│
├── engine.go
├── pipeline.go
├── processor.go
├── state.go
├── profile.go
├── config.go
├── result.go
├── middleware.go
├── hook.go
├── registry.go
├── errors.go
│
├── preset/
│   ├── standard.go
│   ├── asr.go
│   └── ocr.go
│
├── lexicon/
│   ├── lexicon.go
│   ├── memory.go
│   ├── builder.go
│   └── ahocorasick.go
│
├── match/
│   ├── span.go
│   ├── interval.go
│   ├── levenshtein.go
│   ├── ngram.go
│   └── replace.go
│
├── processor/
│   ├── clean/
│   ├── disfluency/
│   ├── alias/
│   ├── deterministic/
│   ├── pinyin/
│   ├── fuzzy/
│   ├── context/
│   └── llm/
│
├── hooks/
│   ├── slog.go
│   └── metrics.go
│
├── internal/
│
├── examples/
│
└── testdata/
```

---

# 49. 依赖方向

必须保持：

```text
engine
  ↓
pipeline
  ↓
processor
  ↓
state
  ↓
lexicon / match
```

禁止反向依赖：

```text
processor → engine      ❌
processor → pipeline   ❌
lexicon → engine       ❌
match → processor      ❌
match → engine         ❌
```

---

# 50. 第三方依赖

主模块：

```text
github.com/stack-haven/lexnorm
```

必须只依赖 Go Standard Library。

禁止核心模块直接依赖：

```text
kratos
gorm
ent
redis
mysql
otel
LLM SDK
```

需要第三方基础设施时使用独立 submodule。

例如：

```text
github.com/stack-haven/lexnorm/hooks/otel
```

---

# 51. Go 工程设计哲学

参考 Go-Kratos、Go Micro 等工程体系，但不依赖其代码。

重点借鉴：

## Interface First

```go
Processor
Lexicon
Hook
```

---

## Dependency Inversion

高层依赖接口。

---

## Explicit Construction

```go
New(...)
WithLexicon(...)
WithPipeline(...)
WithMiddleware(...)
```

---

## Small Interfaces

Processor 保持：

```go
Name()
Process()
```

避免巨大接口。

---

## Composition

通过：

```go
Pipeline(
    ProcessorA,
    ProcessorB,
)
```

组合能力。

---

# 52. 并发模型

Engine：

```text
Thread Safe
```

Processor：

```text
Prefer Stateless
```

State：

```text
Request Scoped
Single Owner
```

Lexicon：

```text
Immutable
Read Only
Shared
```

典型模型：

```text
Engine
 ├── Request A → State A
 ├── Request B → State B
 ├── Request C → State C
 └── Request D → State D
```

禁止共享 State。

---

# 53. 性能设计

Lexicon Build：

```text
O(Build)
```

Runtime：

```text
Read Only
```

主要优化：

```text
Aho-Corasick
n-gram Index
ID Map
Immutable Snapshot
IntervalSet
Single-pass Replace
sync.Pool
```

---

# 54. Aho-Corasick

用于多模式匹配。

Build 时构建：

```text
Trie
+
Failure Link
```

Runtime：

```text
Single Scan
```

目标：

```text
O(N + Matches)
```

而不是：

```text
O(Entries × Text)
```

---

# 55. Fuzzy Matching

必须避免：

```text
每个词条
 ×
每个字符窗口
 ×
Levenshtein
```

推荐：

```text
n-gram Candidate
      ↓
Length Filter
      ↓
Distance
      ↓
Threshold
```

---

# 56. Protection Interval

Protected Span 必须使用高效区间结构。

推荐：

```text
Sorted Interval Set
```

目标：

```text
IsLocked = O(log n)
```

避免：

```text
O(n²)
```

---

# 57. 热更新

推荐：

```go
type Store struct {
    current atomic.Pointer[Lexicon]
}
```

更新：

```go
store.Swap(newLexicon)
```

原则：

```text
Old Request → Old Snapshot
New Request → New Snapshot
```

不阻塞正常规范调用。

---

# 58. Error

核心错误：

```go
var (
    ErrProcessorNotFound = errors.New(
        "lexnorm: processor not found",
    )

    ErrInvalidConfig = errors.New(
        "lexnorm: invalid config",
    )

    ErrTextTooLarge = errors.New(
        "lexnorm: text exceeds limit",
    )
)
```

Processor 错误：

```go
type ProcessorError struct {
    Name string
    Op   string
    Err  error
}
```

必须支持：

```go
errors.Is
errors.As
```

核心库不得引入 Kratos Error。

业务系统需要 Kratos 错误码时，在业务侧转换。

---

# 59. Config

Config 必须显式校验。

```go
func (c Config) Validate() error
```

非法配置：

```text
New → error
```

不得：

```text
非法值
 ↓
静默 Clamp
 ↓
继续执行
```

如果需要宽容行为，应显式提供：

```text
Lenient Configuration
```

---

# 60. Configuration 与业务规则

业务规则不能写死在 Processor。

例如：

```yaml
processors:
  - name: fuzzy
    config:
      auto_threshold: 0.80

  - name: deterministic
    config:
      rules:
        - from: "个种籽"
          to: "颗种籽"
```

Processor 负责执行规则。

业务系统负责管理规则来源。

---

# 61. Processor 配置原则

每个 Processor：

```text
Config
 ↓
Validate
 ↓
Build
 ↓
Runtime Read Only
```

运行期间不修改配置。

配置更新通过重新创建 Processor / Pipeline / Engine 或对应 Snapshot 完成。

---

# 62. Determinism

以下因素必须固定：

```text
Lexicon Ordering
Candidate Ordering
Match Conflict Resolution
Pipeline Ordering
Processor Configuration
```

发生多个候选时必须有确定性的 Tie-breaker。

禁止依赖：

```text
map iteration order
goroutine scheduling
random order
```

---

# 63. Match 冲突规则

默认：

```text
Longest Match First
```

相同长度：

```text
Higher Priority
```

仍相同：

```text
Stable Lexicographical Ordering
```

所有冲突消解规则必须确定。

---

# 64. 结果不可变原则

Result 对外表现为值语义。

内部运行完成后：

```text
State
 ↓
Result Snapshot
```

Result 不应继续关联可变 State。

---

# 65. Hook 安全

Hook 不得影响核心规范结果。

如果 Hook 失败：

```text
Hook Error
```

默认不影响：

```text
Processor
Pipeline
Result
```

Hook 属于旁路能力。

---

# 66. Middleware 顺序

Middleware 采用显式顺序。

例如：

```go
WithMiddleware(
    Recover(),
    Timing(),
    WithHooks(...),
)
```

执行顺序必须稳定。

Middleware 不得隐式改变 Processor Order。

---

# 67. 单 Processor 测试

每个 Processor 必须可以单独测试：

```go
state := NewState(...)
processor.Process(ctx, state)
```

测试不得强制启动完整 Engine。

---

# 68. Pipeline 测试

Pipeline 测试必须验证：

```text
Processor Order
Error Policy
Protection
Changes
Result
```

---

# 69. Engine 测试

Engine 测试：

```text
Concurrent Normalize
Profile
Lexicon Snapshot
Middleware
Hook
Hot Swap
Cancellation
```

---

# 70. Race Test

必须执行：

```bash
go test -race ./...
```

重点覆盖：

```text
100+ concurrent Normalize
```

Engine 可以共享。

State 不可以共享。

Lexicon 可以共享。

---

# 71. Fuzz Test

至少包含：

```text
UTF-8 边界
空文本
超长文本
组合字符
Emoji
非法 UTF-8
空 Lexicon
重复规则
冲突规则
Protected Span
```

核心：

```go
FuzzNormalize
```

---

# 72. Golden Test

需要建立：

```text
testdata/*.golden
```

覆盖：

```text
单 Processor
Standard Pipeline
Custom Pipeline
Protection
Decision
Provenance
Error
```

Golden 结果必须可复现。

---

# 73. Benchmark

必须覆盖：

```text
Clean
Disfluency
Alias
Deterministic
Pinyin
Fuzzy
Context
Pipeline
Engine
```

并持续进行性能回归。

目标基线：

| 场景                 |      目标 |
| -------------------- | --------: |
| 45 字 / 2000 词条    | < 200 µs |
| 内存                 |   < 32 KB |
| allocations          |     < 200 |
| 1000 字 / 10000 词条 |    < 3 ms |

如果实际实现无法达到目标，应以 Benchmark 数据为准，不得为了达到目标而虚构性能结论。

---

# 74. 开源工程要求

必须包含：

```text
LICENSE
README.md
README.zh-CN.md
CHANGELOG.md
CONTRIBUTING.md
SECURITY.md
CODE_OF_CONDUCT.md
```

每个公共包需要完整 GoDoc。

所有导出 API 必须有规范注释。

---

# 75. Go 版本

初始版本：

```go
go 1.22
```

避免为了语言特性而过度提高最低版本。

如果实际使用 Go 1.23+ 特性，应重新评估最低版本，不得为了追求新 API 而破坏生态兼容性。

---

# 76. API 稳定性

开发阶段允许快速迭代。

v1.0 后重点 API 应冻结：

```text
Processor
Pipeline
Engine
State
Result
Lexicon
```

尤其：

```go
Processor.Process(...)
```

属于核心稳定接口。

---

# 77. 第一阶段开发顺序

Coding Agent 必须按照以下顺序实施，不要一次性大规模实现全部功能。

## Phase 1：Core

实现：

```text
Processor
State
Result
Change
Span
Profile
Config
Errors
```

完成：

```bash
go test ./...
go vet ./...
```

---

## Phase 2：Pipeline

实现：

```text
Pipeline
Processor Composition
ContinueOnError
FailFast
```

验证：

```text
Pipeline 可以运行任意 Processor
Pipeline 本身实现 Processor
```

---

## Phase 3：Protection

实现：

```text
Span
IntervalSet
Replace
Suggest
Lock
```

---

## Phase 4：Lexicon

实现：

```text
Lexicon
Builder
Memory Implementation
ID Index
Immutable Snapshot
```

---

## Phase 5：Processor

依次实现：

```text
Clean
Disfluency
Alias
Deterministic
Pinyin
Fuzzy
Context
```

每完成一个 Processor：

```text
Unit Test
Benchmark
Fuzz where applicable
```

---

## Phase 6：Preset

实现：

```text
preset.Standard
preset.ASR
preset.OCR
```

Preset 只负责组合 Processor。

---

## Phase 7：Engine

实现：

```text
New
Normalize
Profile
Lexicon
Pipeline
Middleware
Hook
```

---

## Phase 8：Registry

实现：

```text
Descriptor
Registry
Dynamic Construction
JSON Configuration
```

不要让 Registry 进入核心 Processor API。

---

## Phase 9：Performance

实现：

```text
Aho-Corasick
n-gram
Levenshtein
IntervalSet
Single-pass Replace
sync.Pool
```

通过 Benchmark 验证优化效果。

---

## Phase 10：Open Source

完成：

```text
README
GoDoc
Examples
Golden
Fuzz
Benchmark
CI
License
Security
Contribution Guide
```

---

# 78. Coding Agent 实施约束

Coding Agent 在开发过程中必须遵循：

### 约束 1

不要为了兼容旧业务代码而把业务概念带入 `ark-lexnorm`。

---

### 约束 2

不要将 `tenant` 改名后继续保留 Tenant 的语义。

统一使用：

```text
Profile
```

---

### 约束 3

不要把所有 Processor 强行绑定到 Engine。

---

### 约束 4

不要让 Pipeline 成为 Processor 的必经路径。

---

### 约束 5

不要让 Registry 成为 Processor 的必经路径。

---

### 约束 6

不要把 Standard Pipeline 写死在 Engine。

---

### 约束 7

不要让 Processor 直接修改共享文本。

统一使用：

```text
State.Replace
State.Suggest
```

---

### 约束 8

不要在 Core 中引入第三方依赖。

---

### 约束 9

不要为了性能提前牺牲 API 清晰度。

先：

```text
Correctness
 ↓
Determinism
 ↓
Benchmark
 ↓
Optimization
```

---

### 约束 10

不要通过隐藏全局变量实现跨请求状态。

---

# 79. 架构不变量

以下规则必须加入 Code Review Checklist。

```text
I1  Processor 不依赖 Engine

I2  Processor 不依赖 Pipeline

I3  Pipeline 可以包含任意 Processor

I4  Pipeline 本身实现 Processor

I5  用户 Processor 与内置 Processor 使用相同接口

I6  Registry 不是 Processor 使用前提

I7  Preset 不是 Engine 内部硬编码

I8  Engine 不实现具体规范算法

I9  文本修改必须通过 State

I10 State 不允许跨请求共享

I11 Lexicon Runtime 只读

I12 相同输入必须产生可复现结果

I13 Hook 不得修改规范结果

I14 Middleware 不得隐式改变 Processor 顺序

I15 Core 不得依赖业务领域

I16 Core 只依赖 Go Standard Library
```

---

# 80. 最终开发者心智模型

开发者只需要理解：

```text
                 Knowledge
                     │
                  Lexicon
                     │
                     ▼
Processor ───────► State ───────► Result
                     ▲
                     │
                  Pipeline
                     ▲
                     │
                   Engine
```

职责：

```text
Lexicon
    提供知识

Processor
    使用知识完成一种规范能力

Pipeline
    组合 Processor 并决定执行顺序

Profile
    标识规范运行上下文

State
    保存一次运行状态

Engine
    管理运行环境

Result
    描述最终规范结果
```

---

# 81. 最终架构定义

`ark-lexnorm` 不定义为：

> 一个固定包含若干文本处理步骤的工具。

而定义为：

> **一个以 Processor 为最小规范能力单元，以 Pipeline 为能力组合机制，以 Profile + Lexicon 为规范上下文，以 Engine 为运行 Facade，以 Result 为可解释输出的 Go 文本词法规范基础设施。**

最终核心关系：

```text
                  ┌─────────────┐
                  │   Engine    │
                  │   Runtime   │
                  └──────┬──────┘
                         │
                         ▼
                  ┌─────────────┐
                  │  Pipeline   │
                  │  Composite  │
                  └──────┬──────┘
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

最终架构宣言：

> **规范能力由 `ark-lexnorm` 提供，规范流程由开发者决定。**
>
> 可以只使用一个 Processor，也可以组合完整 Pipeline；可以使用内置能力，也可以编写自己的 Processor；可以使用 Standard Preset，也可以完全定义自己的规范流程。
>
> `ark-lexnorm` 不规定你的业务，只提供一套确定、可组合、可解释、可扩展的文本词法规范基础设施。
