# 07 · Engine 与 Profile

> 源节：§4.3~§4.6 · §31 多 Profile 并发模型 · §32 多业务上下文隔离 · §34 高可用
> 适用阶段：Phase 7
> 受众：核心开发者
> 关键性：Engine 是用户面对的 Facade，API 形态直接影响开源采纳率

---

## 1. Engine 定位

整个运行体系的 **Facade**。

### Engine 负责

- 生命周期
- Profile 路由
- Lexicon
- Pipeline
- Middleware
- Hook
- 并发安全
- 热更新
- Result

### Engine 不负责

- 具体规范算法实现
- 词表内容
- 业务权限
- 持久化

---

## 2. Engine API（**1.2 修订**）

```go
type Engine struct {
    // immutable runtime configuration
}

// 1.2 决议 D2：保留 error 返回（fail-fast）
func New(opts ...Option) (*Engine, error)

func (e *Engine) Normalize(
    ctx context.Context,
    text string,
    opts ...CallOption,
) (Result, error)
```

### 1.0 → 1.2 New 签名对照

| 版本 | 签名 | 行为 |
|---|---|---|
| 1.0 | `func New(opts ...Option) (*Engine, error)` | fail-fast，构造期校验 |
| 1.1 | `func New(opts ...Option) *Engine` | 延迟到首次 Normalize |
| **1.2** | **`func New(opts ...Option) (*Engine, error)`** | **fail-fast，构造期校验**（D2 决议：更安全） |

### 推荐使用方式

```go
engine, err := lexnorm.New(
    lexnorm.WithProfile("default"),
    lexnorm.WithLexicon(lexicon),
    lexnorm.WithPipeline(pipeline),
)

if err != nil {
    return err  // 构造期失败立即返回
}

result, err := engine.Normalize(
    ctx,
    "呃，佘丽群明天开会",
)
```

---

## 3. Engine 与 Pipeline 的职责边界

| 维度 | Engine | Pipeline | Processor |
|---|---|---|---|
| Runtime | ✅ | ❌ | ❌ |
| Profile | ✅ | ❌ | ❌ |
| Lexicon | ✅ | ❌ | ❌ |
| Pipeline 组合 | ❌ | ✅ | ❌ |
| Execution Order | ❌ | ✅ | ❌ |
| Processor 执行 | ❌ | ✅ | ✅ |
| 具体规范算法 | ❌ | ❌ | ✅ |
| Middleware | ✅ | ❌ | ❌ |
| Hook | ✅ | ✅ | ❌ |

> **不得混淆三者职责。** 任何职责越界都属于架构违规。

---

## 4. Profile

```go
type ProfileID string

type Profile struct {
    ID      ProfileID
    Version string
}
```

### 推荐模型

```text
Profile
 ├── ID + Version
 ├── Lexicon
 ├── Pipeline
 ├── Config
 ├── Decision Policy
 └── Protection Policy
```

但**这些对象不应该强制全部封装进 Profile 类型**——Profile 本身只承担 identity / context 语义。

### Profile 不负责

- 数据库存储
- 权限
- 用户
- 生命周期
- 多租户

### 调用方路由示例（多 Profile）

```go
profiles := map[string]lexnorm.ProfileBundle{
    "default": {Lexicon: lexA, Pipeline: pipeStd, Config: cfgStd},
    "asr":     {Lexicon: lexB, Pipeline: pipeASR, Config: cfgASR},
    "ocr":     {Lexicon: lexC, Pipeline: pipeOCR, Config: cfgOCR},
}

eng, _ := lexnorm.New(
    lexnorm.WithProfiles(profiles),  // 注入多 Profile
    lexnorm.WithDefaultProfile("default"),
)

// 调用时按字符串选择
res, _ := eng.Normalize(ctx, text, lexnorm.WithProfileName("asr"))
```

---

## 5. ProfileResolver（**1.1 新增 / 1.2 采纳**）

为支持同一进程同时服务多个 Profile，**不要求每个 Profile 建立独立 Engine**。

```go
type ProfileResolver interface {
    Resolve(
        ctx context.Context,
        id ProfileID,
    ) (Runtime, error)
}
```

### 业务系统实现示例

```go
type myResolver struct {
    store *profileStore
}

func (r *myResolver) Resolve(ctx context.Context, id lexnorm.ProfileID) (lexnorm.Runtime, error) {
    p, err := r.store.Load(ctx, id)
    if err != nil {
        return lexnorm.Runtime{}, lexnorm.ErrRuntime
    }
    return lexnorm.Runtime{
        Profile:           p.Profile,
        Lexicon:           p.Lexicon,
        Pipeline:          p.Pipeline,
        Config:            p.Config,
        ProfileVersion:    p.Version,
        LexiconVersion:    p.Lexicon.Version(),
        PipelineVersion:   p.Pipeline.Version(),
        ProcessorVersions: p.ProcessorVersions,
    }, nil
}

eng, _ := lexnorm.New(lexnorm.WithProfileResolver(&myResolver{store: ps}))
```

### 路由流程

```text
请求
 ↓
业务鉴权（业务层负责）
 ↓
确定 ProfileID
 ↓
ProfileResolver
 ↓
Runtime Snapshot
 ↓
Engine.Normalize(...)
```

> **核心包不得自行根据用户身份推断 Profile**（架构不变量 20）。

---

## 6. Runtime Snapshot

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

**Runtime 在一次请求开始时确定，请求过程中不得切换 Snapshot。**

> 架构不变量 8。

---

## 7. Engine Option 总览

```go
// 核心配置
WithProfile(name string) Option
WithLexicon(lex lexicon.Lexicon) Option
WithPipeline(p Pipeline) Option

// 多 Profile（与 ProfileResolver 二选一）
WithProfiles(bundles map[string]ProfileBundle) Option
WithDefaultProfile(name string) Option
WithProfileResolver(r ProfileResolver) Option

// 横切能力
WithMiddleware(mw ...Middleware) Option
WithHooks(h ...Hook) Option

// 行为策略
WithErrorPolicy(p ErrorPolicy) Option
WithConcurrency(n int) Option

// 生命周期
WithPreset(p Preset) Option
```

### 校验

`New(...)` 必须对所有 Option 进行合法性校验：

- 非法配置返回 `error`（**D2 决议：fail-fast**）
- 必填项缺失返回 `ErrInvalidConfig`
- Profile 名称不存在返回 `ErrInvalidConfig`
- Pipeline 为空返回 `ErrInvalidConfig`

---

## 8. Normalize 调用流程

```text
Normalize(ctx, text, opts...)
  │
  ├─ 1. 解析 opts（CallOption）→ callConfig
  ├─ 2. 决定本次 Runtime
  │     ├─ 优先 CallOption.WithRuntime
  │     └─ 否则 Engine default Runtime
  ├─ 3. NewState(text, runtime)
  ├─ 4. Apply Middleware Chain（外→内）
  │     └─ 内层 Handler：执行 Pipeline
  │         └─ Pipeline.Process(state)
  │             └─ Processor A → B → C → ...
  ├─ 5. Pipeline 完成后 state.Result() → Result
  ├─ 6. 填充 Result.Runtime（从 callConfig.runtime 拷贝）
  ├─ 7. Trigger Hook (PipelineEnd)
  └─ 8. 返回 Result
```

### 关键不变量

- **Engine 本身不持有 Request Scoped 状态**
- **Normalize 可并发调用**
- **同一 Engine 实例可被任意多 goroutine 共享**

---

## 9. CallOption

Normalize 接受 `...CallOption`，用于**单次调用**级别的覆盖：

```go
type CallOption interface {
    apply(*callConfig)
}

// 1.2 支持的 CallOption
WithProfile(profile Profile)
WithLexicon(lexicon Lexicon)
WithPipeline(pipeline Pipeline)
WithConfig(config Config)
WithRuntime(runtime Runtime)   // 整体覆盖
```

### 原则

- Engine 提供默认 Runtime
- CallOption 可以指定本次调用 Runtime
- CallOption **不得修改 Engine 全局状态**
- Runtime **必须在调用开始时确定**
- 调用过程中**不得改变 Runtime**

### 推荐优先级

```text
Call Runtime > Engine Runtime
```

---

## 10. 并发模型

```text
Engine
 ├── Request A → State A → Result A
 ├── Request B → State B → Result B
 ├── Request C → State C → Result C
 └── Request D → State D → Result D
```

| 对象 | 是否可并发共享 |
|---|:--:|
| Engine | ✅ |
| Pipeline | ✅ |
| Processor | ✅ |
| Lexicon | ✅ |
| Runtime | ✅（不可变） |
| ProfileResolver | ✅ |
| State | ❌（单 goroutine 独占） |
| Result | ✅（值语义） |

---

## 11. 多 Profile 并发模型（**1.1 §35**）

```text
                 Engine
                    │
       ┌────────────┼────────────┐
       ▼            ▼            ▼
   Profile A    Profile B    Profile C
       │            │            │
   Snapshot A   Snapshot B   Snapshot C
```

**要求**：

- Engine 不保存请求级 Profile 状态
- State 不跨请求共享
- Runtime 不可变
- Lexicon 不可变
- Pipeline 推荐不可变
- Config 不可变

---

## 12. 多业务上下文隔离（**1.1 §36**）

**核心包不实现业务授权**：

```text
请求
 ↓
身份认证（业务层）
 ↓
业务授权（业务层）
 ↓
确定 Profile
 ↓
加载 Runtime
 ↓
ark-lexnorm
```

**禁止**：

```text
ark-lexnorm
 ↓
读取用户身份
 ↓
自行判断权限
```

> 规范化引擎只负责文本处理，**不负责业务安全边界**（架构不变量 20）。

---

## 13. 高可用架构（**1.1 §40**）

### 13.1 Engine HA

Engine 无状态：

```text
                Load Balancer
                     │
       ┌─────────────┼─────────────┐
       ▼             ▼             ▼
    Engine A      Engine B      Engine C
```

任何 Engine 实例都可以处理任意请求。

### 13.2 Lexicon HA

Lexicon 使用 Immutable Snapshot + Atomic Swap。外部数据源不可用时继续使用最后有效 Snapshot；新版本构建失败保留 V1。

详见 [06-Lexicon与热更新](06-Lexicon与热更新.md) §9。

### 13.3 Request Consistency

一次请求锁定 Runtime V1；中途发布 V2 时本请求仍使用 V1；下一请求才使用 V2。

### 13.4 Processor HA

Processor 不得依赖单例外部服务。例如 LLM Processor：

```text
LLM unavailable
 ↓
Processor Error
 ↓
Partial
 ↓
保留已有文本
```

是否重试 / 切换 Provider / 跳过 LLM，由业务层 Processor 自身或调用方策略决定。

---

## 14. 故障矩阵（**1.1 §41**）

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

## 15. Engine 测试规范

Engine 测试必须覆盖：

| 项 | 验证内容 |
|---|---|
| Concurrent Normalize | 100+ goroutine 并发，零 race |
| New error return | 非法配置返回 error（D2 决议） |
| Profile 路由 | 不同 ProfileName 走不同 Pipeline / Lexicon |
| ProfileResolver | 业务层 Resolver 正确返回 Runtime |
| Lexicon Snapshot | 热更新期间正在进行的请求结果稳定 |
| Middleware | panic recover / timing / tracing 正常触发 |
| Hook | PipelineStart / ProcessorStart / ProcessorEnd / PipelineEnd 全部触发 |
| Cancellation | ctx 取消时立即退出，返回 Canceled 状态 |
| HA 故障矩阵 | 11 种故障场景全部验证 |

---

## 16. 自检清单

- [ ] Engine 是否承担了具体规范算法？（违规）
- [ ] New(...) 是否返回 error（D2 决议）？
- [ ] Normalize 是否并发安全？
- [ ] Option 校验是否在 `New` 阶段就完成？
- [ ] Profile 是否膨胀为包含所有绑定的大对象？
- [ ] CallOption 是否意外修改了 Engine 状态？
- [ ] 热更新是否会被 `Normalize` 阻塞？
- [ ] Middleware 链是否在 Normalize 入口统一应用？
- [ ] 是否实现了 ProfileResolver 抽象？
- [ ] 是否实现了 Last Known Good Lexicon 模式？

---

## 17. 相关文档

- 上游：[02-核心领域模型](02-核心领域模型.md) §4
- Lexicon：[06-Lexicon与热更新](06-Lexicon与热更新.md)
- Middleware/Hook：[08-横切能力Middleware与Hook](08-横切能力Middleware与Hook.md)
- 测试：[15-测试策略与质量工程](15-测试策略与质量工程.md)
