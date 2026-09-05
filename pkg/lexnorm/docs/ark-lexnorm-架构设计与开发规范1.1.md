# ark-lexnorm 文本词法规范引擎

## 架构设计与开发规范

**项目名称**：ark-lexnorm
**Go Module**：`github.com/stack-haven/lexnorm`
**中文名称**：文本词法规范引擎
**简称**：文本规范引擎
**定位**：通用、可组合、可解释、可确定性的文本词法规范化引擎
**核心依赖**：仅 Go 标准库
**目标版本**：v1.1

---

# 1. 项目定位

`ark-lexnorm` 是一个面向开发者和学习者开放的通用文本词法规范引擎。

引擎接收原始文本，通过可组合的 Processor 按确定的处理顺序完成：

- 噪声清洗
- 口语/语气词处理
- 别名归一化
- 确定性纠错
- 拼音/同音纠错
- 模糊纠错
- 上下文纠错
- 可选的 LLM 精修

典型输入：

```text
原始文本
    ↓
ark-lexnorm
    ↓
规范化文本 + 变更记录 + 状态 + 运行快照信息
```

典型应用包括：

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

文本修改风险与知识确定性负相关。

因此 Processor 默认按照：

```text
高确定性
    ↓
低确定性
```

执行。

推荐默认顺序：

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
    ↓
LLM Refine
```

高确定性处理结果必须能够被低确定性 Processor 识别并保护。

---

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

不得通过隐藏规则改变用户已经明确配置的执行行为。

---

## 2.3 可解释

每一次文本变化都必须能够回答：

- 修改了什么？
- 修改前是什么？
- 修改后是什么？
- 哪个 Processor 修改的？
- 为什么修改？
- 使用了哪个规则？
- 使用了哪个词条？
- 置信度是多少？
- 是否实际应用？
- 本次运行使用了哪个 Profile？
- 本次运行使用了哪个 Lexicon Snapshot？
- 使用了哪个 Pipeline？
- 各 Processor 的版本是什么？

---

## 2.4 可组合

Processor 是最小能力单元。

每个 Processor：

- 可以独立使用
- 可以组合到 Pipeline
- 可以被 Engine 调用
- 可以被用户替换
- 可以被禁用
- 可以被重新排序

内置 Processor 与用户自定义 Processor 在架构层面没有特殊地位。

---

## 2.5 可降级

任何非核心 Processor 发生错误，不得默认丢失原始文本。

原则：

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

最终业务是否允许将 `Partial` 结果交给下游 Agent、文档系统等，由业务层决定。

---

## 2.6 无业务耦合

核心包不得出现：

- Tenant
- TenantID
- UserTenant
- ASR
- HR
- Employee
- Meeting
- Agent
- 词库中心
- 企业组织架构

这些均属于业务系统。

核心包只提供：

```text
Profile
Lexicon
Processor
Pipeline
State
Result
Runtime Snapshot
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

核心职责边界：

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

要求：

1. 无隐式全局状态
2. 可以独立运行
3. 可以被 Pipeline 组合
4. 可以被 Engine 调用
5. 不得依赖具体业务系统
6. 不得直接修改 State 内部字段
7. 文本修改必须通过 State.Replace
8. 建议支持 Processor Version

Processor Version 用于结果追溯。

---

# 5. Pipeline

Pipeline 是 Processor 的组合器，同时本身也是 Processor。

```go
type Pipeline interface {
    Processor
    Processors() []Processor
}
```

推荐实现：

```go
type pipeline struct {
    processors []Processor
}
```

执行：

```text
Pipeline
 ├── Normalize
 ├── Disfluency
 ├── Alias
 ├── Deterministic
 ├── Pinyin
 ├── Fuzzy
 ├── Context
 └── LLM
```

Pipeline 必须：

- 顺序明确
- 可裁剪
- 可插入
- 可替换
- 可独立执行
- 不依赖 Engine

---

# 6. Engine

Engine 是 Facade。

```go
type Engine struct {
    // immutable runtime configuration
}
```

核心 API：

```go
func New(opts ...Option) *Engine

func (e *Engine) Normalize(
    ctx context.Context,
    text string,
    opts ...CallOption,
) (Result, error)
```

Engine 必须满足：

- 并发安全
- 长生命周期
- 无请求级可变状态
- 无业务租户状态
- 无用户状态
- 可以水平扩展
- 可以被多个请求并发调用

---

# 7. Runtime Snapshot

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

Runtime 在一次请求开始时确定。

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

不得出现：

```text
Processor A 使用 Lexicon V1
Processor B 使用 Lexicon V2
```

同一次 Normalize 调用必须使用一致的 Snapshot。

---

# 8. Profile

Profile 用于表示一个规范化运行上下文。

```go
type ProfileID string

type Profile struct {
    ID      ProfileID
    Version string
}
```

Profile 不代表 Tenant。

Profile 可以表达：

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

例如：

```text
ASR 场景
    Profile: asr-default

会议记录场景
    Profile: meeting-document

搜索场景
    Profile: search-normalization
```

核心包不负责 Profile 的持久化和权限。

---

# 9. Runtime Profile Resolver

为了支持同一个进程中同时服务多个 Profile，不允许要求每个 Profile 建立独立 Engine。

推荐由应用层负责 Runtime Snapshot 的解析。

```go
type ProfileResolver interface {
    Resolve(
        ctx context.Context,
        id ProfileID,
    ) (Runtime, error)
}
```

Resolver 可以由业务系统实现。

例如：

```text
请求
 ↓
业务鉴权
 ↓
确定 ProfileID
 ↓
ProfileResolver
 ↓
Runtime Snapshot
 ↓
Engine.Normalize(...)
```

核心包不得自行根据用户身份推断 Profile。

---

# 10. CallOption

Normalize 支持调用级运行时覆盖。

例如：

```go
type CallOption interface {
    apply(*callOptions)
}
```

支持：

```go
WithProfile(profile Profile)
WithLexicon(lexicon Lexicon)
WithPipeline(pipeline Pipeline)
WithConfig(config Config)
WithRuntime(runtime Runtime)
```

原则：

- Engine 提供默认 Runtime
- CallOption 可以指定本次调用 Runtime
- CallOption 不得修改 Engine 全局状态
- Runtime 必须在调用开始时确定
- 调用过程中不得改变 Runtime

推荐优先级：

```text
Call Runtime
    >
Engine Runtime
```

---

# 11. State

State 是单次执行的可变工作空间。

State 不允许跨请求共享。

推荐接口：

```go
type State struct {
    // private
}
```

公开方法：

```go
func (s *State) Text() string

func (s *State) Lexicon() Lexicon

func (s *State) Config() Config

func (s *State) Replace(
    span Span,
    to string,
    meta ChangeMeta,
) error

func (s *State) Suggest(
    span Span,
    to string,
    meta ChangeMeta,
) error

func (s *State) Lock(span Span) error

func (s *State) IsLocked(span Span) bool
```

所有文本修改必须通过：

```go
State.Replace(...)
```

禁止 Processor 直接：

```go
strings.ReplaceAll(...)
```

原因：

`Replace` 是统一维护：

- Span
- UTF-8 offset
- Protected Span
- Change
- Processor
- Rule
- Confidence
- Provenance

的唯一入口。

---

# 12. Span

统一采用：

```go
type Span struct {
    Start int
    End   int
}
```

区间：

```text
[Start, End)
```

必须明确：

> Start / End 使用 UTF-8 字节偏移。

示例：

```text
文本：
你好世界

Span：
[0, 6)
```

所有 Processor 必须遵守统一 Span 约定。

---

# 13. Protected Span

Protected Span 用于防止高确定性结果被低确定性 Processor 再次修改。

例如：

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

后续 Fuzzy Processor 不得将：

```text
田华
```

再次修改。

保护机制必须支持：

- 初始保护
- Processor 产生保护
- Replace 后级联保护
- 区间重叠判断
- 查询是否锁定

---

# 14. Change

Change 是文本规范化的核心审计对象。

推荐：

```go
type Change struct {
    Span             Span
    From             string
    To               string
    Action           Action
    Kind             ChangeKind
    Source           string
    RuleID            string
    EntryID           string
    Processor        string
    ProcessorVersion string
    Confidence       float64
    Applied          bool
    Reason            string
}
```

Change 必须能够表达：

```text
建议修改
实际修改
跳过修改
```

---

# 15. Decision

Processor 的候选结果不能简单等同于实际修改。

定义：

```text
Apply
Suggest
Skip
```

含义：

### Apply

直接修改文本。

### Suggest

记录建议，但不修改文本。

### Skip

发现候选，但根据策略不执行。

推荐：

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

# 16. Lexicon

Lexicon 是规范化知识的只读快照。

核心接口：

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

Lexicon 负责：

- 标准词
- 别名
- 变体
- 纠错词
- 同音关系
- 模糊匹配数据
- 关系数据

Lexicon 不负责：

- 权限
- 用户
- 租户
- HR API
- 数据同步
- 数据库存储

---

# 17. Lexicon Source

不同业务系统可以提供不同来源的规范化知识。

抽象为：

```go
type LexiconSource interface {
    Version() string
    Entries(yield func(Entry) bool)
    Relations(yield func(Relation) bool)
}
```

来源可以是：

```text
平台标准数据
    +
业务系统数据
    +
用户配置数据
    +
外部系统同步数据
```

但这些来源在 `ark-lexnorm` 内部不定义具体业务名称。

业务层可以：

```text
Source A
Source B
Source C
      ↓
Lexicon Builder
      ↓
Immutable Lexicon Snapshot
```

---

# 18. Lexicon Composition

多个 Source 可以组合成一个 Lexicon。

推荐：

```go
func Compose(
    sources ...LexiconSource,
) (Lexicon, error)
```

组合过程：

```text
Sources
 ↓
Entry Merge
 ↓
Relation Merge
 ↓
冲突检查
 ↓
ID Index
 ↓
Exact Index
 ↓
Aho-Corasick
 ↓
Pinyin Index
 ↓
Fuzzy Index
 ↓
Immutable Snapshot
```

冲突必须显式处理。

禁止静默覆盖。

例如：

```text
Source A:
小田 → 田华

Source B:
小田 → 田强
```

Builder 必须返回冲突错误，或依据明确的优先级策略处理。

---

# 19. Lexicon Builder

Builder 用于构建不可变 Lexicon。

```go
type Builder struct {
    // private
}
```

构建过程中生成：

- ID Index
- Exact Match Index
- Alias Index
- Aho-Corasick Matcher
- Pinyin Index
- N-Gram Index
- Fuzzy Index
- Relation Index

构建完成后：

```text
Builder
    ↓
Immutable Lexicon
```

运行阶段禁止修改。

---

# 20. Lexicon Store 与热更新

运行时必须使用不可变 Snapshot。

推荐：

```go
type Store struct {
    current atomic.Pointer[Lexicon]
}
```

更新流程：

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

要求：

1. 构建失败不能影响当前版本
2. 校验失败不能替换
3. 替换必须原子完成
4. 正在执行的请求继续使用旧 Snapshot
5. 新请求使用新 Snapshot
6. 不允许请求中途切换 Snapshot

---

# 21. Lexicon Version

每个 Lexicon 必须拥有 Version。

例如：

```text
lexicon-v20260904-001
```

Result 必须记录：

```text
LexiconVersion
```

用于：

- 问题复现
- 审计
- 回滚
- 质量分析
- 线上问题定位

---

# 22. Processor Version

每个 Processor 推荐具有稳定版本标识。

Result 保存：

```go
map[string]string
```

例如：

```text
normalize: v1
alias: v3
pinyin: v2
fuzzy: v4
context: v1
```

确保同一文本可以在历史环境中重现处理链。

---

# 23. Pipeline Version

Pipeline 必须有稳定版本。

Pipeline Version 应与：

- Processor 名称
- Processor 顺序
- Processor Version
- Processor 配置

共同构成运行快照。

推荐：

```text
pipeline-v20260904-001
```

---

# 24. Result

推荐：

```go
type Result struct {
    Text        string
    Status      Status
    Changes     []Change
    Suggestions []Change
    Errors      []error
    Runtime     RuntimeInfo
}
```

RuntimeInfo：

```go
type RuntimeInfo struct {
    ProfileID        ProfileID
    ProfileVersion   string
    LexiconVersion   string
    PipelineVersion  string
    ProcessorVersions map[string]string
}
```

Result 必须能够完整说明：

```text
输入
 ↓
使用什么 Profile
 ↓
使用什么 Lexicon
 ↓
使用什么 Pipeline
 ↓
使用哪些 Processor
 ↓
发生了什么 Change
 ↓
最终文本
 ↓
最终状态
```

---

# 25. Status

推荐：

```go
type Status int

const (
    StatusSuccess Status = iota
    StatusPartial
    StatusCanceled
    StatusFailed
)
```

语义：

### Success

所有必要 Processor 正常完成。

### Partial

部分 Processor 失败，但仍产生可用结果。

### Canceled

调用被 Context 取消。

### Failed

无法形成有效结果。

默认原则：

```text
单个 Processor Failure
    ↓
Partial
```

但对于无法继续运行的基础错误，可以直接 Failed。

---

# 26. 默认 Processor

标准实现建议包含：

## Normalize

处理：

- Unicode 基础规范化
- 空白规范化
- 标点统一
- 基础噪声

---

## Disfluency

处理：

- 口头语
- 重复语
- 无意义语气词

例如：

```text
嗯那个我们就是然后开始
```

---

## Alias

处理：

```text
小明 → 张明
老王 → 王强
张总 → 张强
```

---

## Deterministic

处理确定性规则：

```text
功课 → 攻克
个种子 → 颗种籽
```

---

## Pinyin

处理：

- 同音
- 拼音相近
- 音近字

---

## Fuzzy

处理：

- 编辑距离
- N-Gram
- 模糊匹配

---

## Context

根据上下文判断候选词。

---

## LLM Refine

LLM 只是 Processor。

核心包不定义：

- LLM Provider
- Token 计费
- Prompt 管理
- 模型选择策略
- API Key
- 重试策略

调用方通过自定义 Processor 接入。

---

# 27. Certainty 与 Order

Processor 可以声明确定性等级：

```go
type Certainty int

const (
    CertaintyHigh Certainty = iota
    CertaintyMedium
    CertaintyLow
)
```

但是：

> Certainty 不得覆盖用户显式配置的 Pipeline Order。

即：

```text
显式 Pipeline Order
        >
默认推荐 Order
        >
Certainty 推导
```

Certainty 主要用于：

- 默认 Pipeline 构建
- 文档说明
- 结果解释
- 保护策略

而不是运行时强行重排用户 Pipeline。

---

# 28. Preset

Preset 是标准 Pipeline 模板。

例如：

```go
PresetDefault
PresetHighAccuracy
PresetFast
```

Preset 可以定义：

```text
Processor
Order
Default Config
```

但：

```text
Preset ≠ Engine 内置行为
```

用户选择 Preset 后仍然可以：

- 删除 Processor
- 添加 Processor
- 修改 Processor
- 调整顺序

---

# 29. Registry

Registry 用于 Processor 注册与发现。

```go
type Descriptor struct {
    Name    string
    Order   int
    New     func(cfg json.RawMessage) (Processor, error)
    Default func() any
}
```

Registry 不是 Processor 执行的必要条件。

用户可以直接：

```go
p := NewMyProcessor()
pipeline := NewPipeline(p)
```

也可以：

```text
Registry
 ↓
Descriptor
 ↓
Processor
```

第三方 Processor 不要求修改核心代码。

---

# 30. Middleware

Processor Pipeline 外层可以提供 Middleware。

```go
type Handler func(
    ctx context.Context,
    s *State,
) error

type Middleware func(Handler) Handler
```

推荐 Middleware：

```text
Recover
Timing
Timeout
Hooks
```

Middleware 不负责业务逻辑。

---

# 31. Recover Middleware

Processor Panic 不得导致整个进程异常。

流程：

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
继续/终止由策略决定
```

Panic 必须转换为普通 Error。

---

# 32. Hook

Hook 用于观察运行过程。

```go
type Event struct {
    Type      string
    Processor string
    Duration  time.Duration
    Changes   []Change
    Err       error
    Result    Result
}
```

接口：

```go
type Hook interface {
    OnEvent(context.Context, Event)
}
```

Hook 可用于：

- Metrics
- Logging
- Trace
- Debug
- Quality analysis

Hook 不得改变核心执行结果。

---

# 33. 配置

配置必须显式校验。

禁止：

```text
非法值 → 自动 Clamp
```

例如：

```text
Threshold = 1.5
```

应该：

```text
返回配置错误
```

而不是自动改成：

```text
1.0
```

配置必须能够序列化。

推荐：

```text
JSON / YAML
```

核心包使用：

```go
encoding/json
```

不得引入第三方配置依赖。

---

# 34. 文本修改原则

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

禁止：

```go
strings.ReplaceAll()
regexp.ReplaceAllString()
```

作为 Processor 的最终修改入口。

这些工具可以用于候选计算，但不能绕过 State。

---

# 35. 多 Profile 并发模型

一个 Engine 可以服务多个 Profile：

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

# 36. 多业务上下文隔离

核心包不实现业务授权。

业务层负责：

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

禁止：

```text
ark-lexnorm
 ↓
读取用户身份
 ↓
自行判断权限
```

原因：

规范化引擎只负责文本处理，不负责业务安全边界。

---

# 37. 外部系统数据接入

例如外部系统存在：

```text
人员
别名
称呼
组织
部门
术语
```

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

# 38. ASR 场景实现

业务系统：

```text
ASR
 ↓
原始文本
 ↓
确定 Profile
 ↓
Resolve Runtime
 ↓
ark-lexnorm
 ↓
Result
 ↓
Agent
```

例如：

```text
ASR：
小田今天帮我查一下个种子的情况

规范化：
田华今天帮我查一下颗种籽的情况
```

Result：

```text
Text:
田华今天帮我查一下颗种籽的情况

Changes:
小田 → 田华
个种子 → 颗种籽

Status:
Success

Runtime:
ProfileVersion
LexiconVersion
PipelineVersion
ProcessorVersions
```

Agent 是否允许执行：

```text
Result.Status == Success
```

或者：

```text
Result.Status == Partial
```

属于业务层决策。

---

# 39. 会议文档场景实现

业务系统：

```text
会议录音
 ↓
ASR
 ↓
原始转写
 ↓
外部系统同步人员/组织数据
 ↓
Adapter
 ↓
LexiconSource
 ↓
Lexicon Snapshot
 ↓
ark-lexnorm
 ↓
规范化文本
 ↓
会议文档
```

例如：

```text
原文：
张总说让老王和小田明天处理一下市场部的事情。

规范化：
张强说让王强和田华明天处理一下市场部的事情。
```

所有替换均产生 Change。

会议系统可以利用：

```text
Change
+
RuntimeInfo
```

实现：

- 文档追溯
- 人名纠错审核
- 规范化质量分析
- 问题复现

---

# 40. 高可用架构

## 40.1 Engine HA

Engine 无状态。

```text
                Load Balancer
                     │
       ┌─────────────┼─────────────┐
       ▼             ▼             ▼
    Engine A      Engine B      Engine C
```

任何 Engine 实例都可以处理任意请求。

---

## 40.2 Lexicon HA

Lexicon 使用：

```text
Immutable Snapshot
+
Atomic Swap
```

外部数据源不可用时：

```text
当前 Snapshot
      ↓
继续提供服务
```

新版本构建失败：

```text
V2 Build Failed
      ↓
继续使用 V1
```

不得因为词法数据更新失败导致整个规范化服务不可用。

---

## 40.3 Request Consistency

一次请求：

```text
Resolve Runtime V1
      ↓
State
      ↓
Processor A
      ↓
Processor B
      ↓
Processor C
```

即使此时：

```text
Lexicon V2 发布
```

本请求仍使用：

```text
Lexicon V1
```

下一请求才使用：

```text
Lexicon V2
```

---

## 40.4 Processor HA

Processor 不得依赖单例外部服务。

例如 LLM Processor：

```text
LLM unavailable
      ↓
Processor Error
      ↓
Partial
      ↓
保留已有文本
```

是否重试、切换 Provider、跳过 LLM，由业务层 Processor 自身或调用方策略决定。

---

# 41. 高可用故障矩阵

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

# 42. 性能目标

基准目标：

## 小文本

```text
45 chars
2000 terms
< 200µs
< 32KB
< 200 allocations
```

## 中等文本

```text
1000 chars
10000 terms
< 3ms
```

目标通过以下机制实现：

- Aho-Corasick
- Exact Index
- N-Gram
- ID Index
- Immutable Lexicon
- Interval Set
- 单次文本扫描
- sync.Pool

性能指标属于目标值，必须通过 Benchmark 验证，不得作为未经测试的实现承诺。

---

# 43. 并发安全要求

必须支持：

```go
go test -race ./...
```

要求：

- Engine 可并发
- Lexicon 可并发读
- Pipeline 可并发
- State 不共享
- Result 不共享
- Runtime Snapshot 不可变

禁止：

```text
请求 A 修改 Lexicon
请求 B 同时读取 Lexicon
```

---

# 44. 错误设计

只使用标准库错误机制。

推荐：

```go
var (
    ErrInvalidConfig = errors.New(...)
    ErrInvalidSpan   = errors.New(...)
    ErrConflict      = errors.New(...)
    ErrRuntime       = errors.New(...)
)
```

支持：

```go
errors.Is
errors.As
```

不得引入 Kratos Error。

---

# 45. Context

所有运行接口必须支持：

```go
context.Context
```

Processor 必须检查：

```go
ctx.Done()
```

尤其是：

- Context Processor
- LLM Processor
- 外部计算 Processor

取消后：

```text
Canceled
```

并保持已有结果可诊断。

---

# 46. 包结构

建议：

```text
ark-lexnorm/
├── engine.go
├── option.go
├── call_option.go
├── runtime.go
├── profile.go
├── state.go
├── result.go
├── change.go
├── span.go
├── decision.go
├── status.go
├── processor.go
├── pipeline.go
├── middleware.go
├── hook.go
├── registry.go
├── preset.go
├── config.go
├── errors.go
│
├── lexicon/
│   ├── lexicon.go
│   ├── builder.go
│   ├── source.go
│   ├── compose.go
│   ├── matcher.go
│   ├── store.go
│   └── index.go
│
├── processor/
│   ├── normalize/
│   ├── disfluency/
│   ├── alias/
│   ├── deterministic/
│   ├── pinyin/
│   ├── fuzzy/
│   ├── context/
│   └── llm/
│
├── internal/
│   ├── interval/
│   ├── text/
│   └── ...
│
├── examples/
├── testdata/
└── docs/
```

具体目录可以在实现阶段根据 Go package 边界调整，但必须保持：

```text
核心抽象
+
Lexicon
+
Processor
```

职责清晰。

---

# 47. API 稳定性原则

公开 API 必须克制。

优先：

```go
type Processor interface
type Pipeline interface
type Lexicon interface
type Profile
type Runtime
type Result
```

避免暴露：

- 内部索引结构
- Matcher 内部节点
- State 内部文本缓存
- Interval Tree
- Object Pool
- Atomic 实现细节

内部实现可以演进。

公开 API 应保持稳定。

---

# 48. Object Pool

对高频临时对象可以使用：

```go
sync.Pool
```

例如：

- State
- Change Buffer
- Candidate Buffer
- Match Buffer

但必须保证：

```text
Pool Object
    ↓
请求结束
    ↓
完整 Reset
    ↓
重新放回 Pool
```

禁止残留：

- Profile
- Lexicon
- Text
- Change
- Error
- Protected Span

---

# 49. Determinism

相同输入和相同 Runtime Snapshot 必须产生相同结果。

定义：

```text
Input
+
Profile Version
+
Lexicon Version
+
Pipeline Version
+
Processor Versions
+
Config
=
Deterministic Result
```

不允许 Processor 在默认模式下依赖：

- 当前时间
- 随机数
- 未排序 Map
- 不确定并发执行
- 外部实时数据

LLM Processor 属于天然非确定性能力，应明确标识其非确定性属性，并通过业务配置决定是否启用。

---

# 50. 测试规范

必须包含：

## Unit Test

覆盖：

- Span
- Replace
- Lock
- Decision
- Processor
- Pipeline
- Lexicon
- Runtime
- Result

## Golden Test

验证：

```text
Input
→ Expected Output
```

## Benchmark

验证：

- 延迟
- 内存
- Allocation

## Fuzz Test

重点：

- Unicode
- Span
- Replace
- Alias
- Fuzzy
- Context

## Race Test

```bash
go test -race ./...
```

## Integration Test

至少覆盖：

```text
Engine
+
Runtime
+
Lexicon
+
Pipeline
+
Processor
```

---

# 51. 业务场景验收测试

## 场景 A：ASR

输入：

```text
小田今天帮我查一下个种子的情况
```

Lexicon：

```text
小田 → 田华
个种子 → 颗种籽
```

期望：

```text
田华今天帮我查一下颗种籽的情况
```

必须生成两个 Change。

---

## 场景 B：会议

输入：

```text
张总说让老王和小田明天处理市场部的事情
```

Lexicon：

```text
张总 → 张强
老王 → 王强
小田 → 田华
```

期望：

```text
张强说让王强和田华明天处理市场部的事情
```

三个 Change。

---

## 场景 C：保护

```text
小田 → 田华
```

Alias Processor 完成后锁定。

后续 Fuzzy Processor：

```text
田华
```

不得再次修改。

---

## 场景 D：Lexicon 热更新

请求 A：

```text
Lexicon V1
```

请求处理中发布：

```text
Lexicon V2
```

请求 A 必须继续使用：

```text
V1
```

请求 B 使用：

```text
V2
```

---

## 场景 E：Processor 故障

Context Processor 失败：

```text
Previous Text
    ↓
保留
    ↓
Status = Partial
```

不得返回空文本。

---

## 场景 F：多 Profile 并发

同时：

```text
Request A → Profile A
Request B → Profile B
```

两个请求不得读取对方 Runtime、Lexicon、Config。

---

# 52. 开源规范

项目目标：

- pkg.go.dev 可直接阅读
- API 文档完整
- Example 完整
- README 完整
- License：Apache-2.0
- CI 完整
- Benchmark 可重复
- Fuzz Test 可执行

核心包：

```text
Zero Third-party Dependency
```

第三方依赖只能存在于：

- example
- adapter
- optional integration
- test tooling

不得进入核心运行链路。

---

# 53. README 必须包含

1. 项目定位
2. 安装方式
3. 最简单示例
4. Processor 示例
5. Pipeline 示例
6. Lexicon 示例
7. Runtime 示例
8. 自定义 Processor
9. 自定义 LexiconSource
10. 热更新示例
11. 性能 Benchmark
12. 高可用说明
13. API Reference
14. License

---

# 54. 最小使用示例

```go
lexicon, err := lexicon.Build(
    entries,
)

if err != nil {
    panic(err)
}

pipeline := NewPipeline(
    normalizeProcessor,
    aliasProcessor,
    deterministicProcessor,
    fuzzyProcessor,
)

runtime := Runtime{
    Profile:  profile,
    Lexicon:  lexicon,
    Pipeline: pipeline,
    Config:   config,
}

engine := New(
    WithRuntime(runtime),
)

result, err := engine.Normalize(
    ctx,
    "小田今天帮我查一下个种子的情况",
)

if err != nil {
    return err
}

fmt.Println(result.Text)
```

---

# 55. 自定义 Processor

开发者可以直接实现：

```go
type MyProcessor struct{}

func (p *MyProcessor) Name() string {
    return "my-processor"
}

func (p *MyProcessor) Process(
    ctx context.Context,
    s *State,
) error {
    // candidate detection
    // decision
    // s.Replace(...)
    return nil
}
```

然后：

```go
pipeline := NewPipeline(
    builtinProcessor,
    &MyProcessor{},
)
```

不需要：

- 修改核心代码
- 注册到官方仓库
- 修改 Engine
- 修改 Lexicon

---

# 56. 开发边界

核心项目明确不实现：

```text
数据库
Redis
Kafka
HTTP API
gRPC API
用户中心
权限中心
Tenant
HR
ASR
Agent
LLM Provider
词库管理后台
词库同步服务
反馈学习系统
```

这些可以在项目外围实现 Adapter。

---

# 57. Adapter 架构

推荐：

```text
ark-lexnorm
      ↑
      │
Adapter
      ↑
Business System
```

例如：

```text
HR Adapter
ASR Adapter
LLM Adapter
Database Adapter
```

Adapter 负责将外部世界转换成核心对象。

---

# 58. 运行生命周期

完整生命周期：

```text
Application
   │
   ├── Load Profile
   │
   ├── Load Lexicon Sources
   │
   ├── Build Lexicon
   │
   ├── Build Pipeline
   │
   ├── Create Runtime Snapshot
   │
   └── Create Engine
             │
             ▼
          Request
             │
             ▼
      Resolve Runtime
             │
             ▼
         Create State
             │
             ▼
       Execute Pipeline
             │
             ▼
         Build Result
             │
             ▼
          Return
```

---

# 59. Runtime 构建原则

Runtime 必须在进入执行阶段前完成：

```text
Profile
+
Lexicon
+
Pipeline
+
Config
```

运行阶段只读取。

不得：

```text
Processor 执行过程中
↓
动态读取数据库
↓
改变 Lexicon
↓
改变 Pipeline
```

---

# 60. 设计不变量

以下规则属于架构不变量。

## 不变量 1

Processor 可以独立运行。

## 不变量 2

Pipeline 本身是 Processor。

## 不变量 3

Engine 不承载业务状态。

## 不变量 4

State 不跨请求共享。

## 不变量 5

Lexicon Runtime 只读。

## 不变量 6

文本修改只能通过 State.Replace。

## 不变量 7

Protected Span 必须阻止后续非法覆盖。

## 不变量 8

一次请求必须使用一致 Runtime Snapshot。

## 不变量 9

相同 Input + 相同 Snapshot 应产生确定性结果。

## 不变量 10

Processor 失败不能默认导致原始文本丢失。

## 不变量 11

Profile 不等于 Tenant。

## 不变量 12

核心包不得依赖业务系统。

## 不变量 13

外部 Lexicon 数据必须经过 Builder 才能进入运行时。

## 不变量 14

Lexicon 更新必须原子发布。

## 不变量 15

旧 Snapshot 必须支持正在执行的请求完成。

---

# 61. 开发步骤

开发必须按照以下顺序推进，不建议一次性实现所有 Processor。

## M1：项目骨架

完成：

- Go Module
- License
- README
- package layout
- CI
- 基础错误
- 基础测试

验收：

```bash
go test ./...
go vet ./...
```

---

## M2：核心 Value Objects

实现：

- Span
- Profile
- Decision
- Status
- Change
- RuntimeInfo
- Result

完成对应 Unit Test。

---

## M3：State

实现：

- Text
- Replace
- Suggest
- Lock
- IsLocked
- Protected Span
- Change tracking

重点测试：

- UTF-8
- Span 边界
- 多次 Replace
- 重叠 Replace
- Protected Span
- Replace 后 Offset

---

## M4：Processor / Pipeline

实现：

- Processor
- Pipeline
- Pipeline Builder
- Pipeline Version
- Processor Version

要求：

```text
Processor 独立可执行
Pipeline 独立可执行
```

---

## M5：Runtime / Engine

实现：

- Runtime
- Engine
- New
- Normalize
- Option
- CallOption
- WithRuntime
- WithProfile
- WithLexicon
- WithPipeline

重点测试：

```text
Engine 并发
多个 Profile 并发
Runtime 隔离
Context Cancel
```

---

## M6：Lexicon

实现：

- Entry
- Variant
- Relation
- Lexicon
- LexiconSource
- Builder
- Compose
- Matcher
- Version

实现：

- Exact Index
- ID Index
- Aho-Corasick

---

## M7：Lexicon Store

实现：

- Immutable Snapshot
- Atomic Swap
- Last Known Good
- Version
- Build Validate
- Publish

重点测试：

```text
V1 → V2
请求中切换
并发读写
失败回滚
```

---

## M8：基础 Processor

依次实现：

1. Normalize
2. Disfluency
3. Alias
4. Deterministic

先完成确定性能力。

---

## M9：智能匹配 Processor

依次实现：

1. Pinyin
2. Fuzzy
3. Context

所有 Processor 必须支持：

```text
Apply
Suggest
Skip
```

---

## M10：Middleware / Hook

实现：

- Recover
- Timing
- Timeout
- Hook
- Event

---

## M11：Registry / Preset

实现：

- Descriptor
- Registry
- Preset
- 默认 Pipeline

Registry 不得成为 Processor 的强依赖。

---

## M12：LLM Processor

只定义：

```text
Processor 接口
```

提供 Example。

不把具体 LLM Provider 放入核心包。

---

## M13：完整业务验收

必须完成：

### ASR

```text
原始文本
→ Alias
→ Deterministic
→ Pinyin
→ Fuzzy
→ Context
→ Result
```

### Meeting

```text
外部人员/组织数据
→ LexiconSource
→ Lexicon
→ Pipeline
→ Result
```

---

## M14：高可用验收

测试：

- Engine 多实例
- Runtime 隔离
- Lexicon Atomic Swap
- Old Snapshot 保持
- Processor Panic
- Processor Error
- External Source unavailable
- Context Cancel
- Partial Result

---

## M15：性能优化

执行：

```bash
go test -bench=. -benchmem ./...
```

重点优化：

- Aho-Corasick
- Fuzzy Candidate
- N-Gram
- Span
- Allocation
- State Pool
- Change Buffer

只有 Benchmark 数据证明优化有效后才能合入。

---

## M16：质量门禁

最终必须通过：

```bash
go test ./...
go test -race ./...
go vet ./...
go test -fuzz=Fuzz ./...
go test -bench=. -benchmem ./...
```

同时完成：

- API Example
- README
- API 文档
- Architecture 文档
- Benchmark
- License
- CHANGELOG

---

# 62. Coding Agent 执行规则

Coding Agent 必须遵循：

1. 先实现核心抽象，再实现具体 Processor。
2. 每完成一个 M 阶段必须保证测试通过。
3. 不得为了实现具体业务而向核心包增加 Tenant、ASR、HR、Employee 等概念。
4. 不得绕过 State.Replace 直接修改文本。
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
16. 所有可观测信息必须通过 Result、Change、Hook 等机制暴露。
17. 不得将业务系统的数据源直接注入核心执行链路。
18. Runtime 是一次请求的一致性边界。
19. Lexicon Snapshot 是知识一致性边界。
20. Profile 是规范化上下文标识，不承担业务权限职责。

---

# 63. 最终架构结论

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
            ├── Text
            ├── Changes
            ├── Status
            └── RuntimeInfo
```

该架构必须同时满足：

```text
通用
可组合
可扩展
可解释
可控
确定性
可降级
并发安全
Snapshot 一致性
多 Profile 隔离
Lexicon 原子热更新
水平扩展
零业务耦合
```

核心设计边界最终保持：

```text
业务系统
    │
    │ Profile / LexiconSource / Runtime
    ▼
ark-lexnorm
    │
    │ Result / Change / RuntimeInfo
    ▼
业务系统
```

`ark-lexnorm` 只解决：

> 如何在一个确定的规范化上下文中，使用一组确定的词法知识和 Processor，对文本进行可控、可组合、可解释、可追溯的规范化处理。

业务系统负责：

> 如何产生这个上下文、如何管理数据、如何授权、如何同步外部系统，以及如何消费规范化结果。
