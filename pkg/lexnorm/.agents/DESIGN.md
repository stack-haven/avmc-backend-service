# ark-lexnorm · 设计原则与产品边界

> 定义"什么属于 ark-lexnorm，什么不属于"。
> 任何 PR 修改产品边界 / 引入新设计模式前必读。

---

## 1. 核心定位

ark-lexnorm 是 **通用文本词法规范引擎**，不是 ASR / OCR / 文档 / 知识图谱 / Agent 框架。

### 1.1 一句话定义

> 一个以 Processor 为最小规范能力单元，以 Pipeline 为能力组合机制，以 Profile + Lexicon 为规范上下文，以 ProfileResolver + Runtime 为多 Profile 路由与一致性快照，以 Engine 为运行 Facade，以 Result（含完整 RuntimeInfo）为可解释输出的 Go 文本词法规范基础设施。

### 1.2 三句不能违反的设计声明

1. **规范能力由 ark-lexnorm 提供，规范流程由开发者决定。**
2. **可以只使用一个 Processor，也可以组合完整 Pipeline。**
3. **可以使用内置能力，也可以编写自己的 Processor。**

---

## 2. 核心包与父项目的边界

### 2.1 与 avmc 父项目的边界

| 维度 | avmc 父项目 | ark-lexnorm |
|---|---|---|
| 业务领域 | 多租户 SaaS 平台 | 通用文本规范 |
| 数据隔离 | tenantID | Profile |
| 业务对象 | User / Tenant / Role / Menu | 无 |
| 业务词汇 | 部门 / 客户 / 项目 / 套餐 | 禁止出现 |
| 核心依赖 | kratos / ent / wire | **零** |
| 错误体系 | kratos errors | sentinel errors + 类型化 |
| 配置来源 | Wire ProviderSet | Engine Option |

### 2.2 与具体业务系统的边界

ark-lexnorm **不假设**：

- 上游是 ASR 还是人工录入
- 下游是 Agent / 文档 / 数据库
- 是否存在权限系统
- 是否需要审计
- 是否需要分布式追踪

业务系统负责：

```text
请求 → 业务鉴权 → 确定 ProfileID → ProfileResolver
       ↓
ark-lexnorm.Normalize()
       ↓
Result → 业务系统决定如何使用
```

### 2.3 与未来扩展的边界

| 场景 | ark-lexnorm 提供 | 业务系统提供 |
|---|---|---|
| ASR 集成 | Normalize / Alias / Pinyin / Fuzzy Processor | ASR Client / 业务术语库 / Adapter |
| 多语言 | **不在 v1 范围** | — |
| 自研 Processor | `Processor` 接口 + `Descriptor` | 具体实现 |
| LLM Refine | `processor/llm` 包 + Client interface（**可选扩展，D1**） | 实际 LLM SDK / Prompt / Token 管理 |
| 审计 | `Result.RuntimeInfo` | 日志 / 监控 / 业务审计 |

---

## 3. 六大设计原则

> 完整版见 [`docs/00-项目定位与设计原则.md`](../docs/00-项目定位与设计原则.md) §2。这里只列设计层面的精炼。

### 3.1 确定性优先

```text
Clean → Disfluency → Alias → Deterministic → Pinyin → Fuzzy → Context
```

高确定性处理结果必须能够被低确定性 Processor 识别并保护。**不含 LLM**（D1）。

### 3.2 可控

调用方必须能够：

- 启用/禁用 / 自定义 / 重排 Processor
- 自定义 Pipeline（1.2 起 Pipeline 是 **interface**）
- 配置 Apply / Suggest / Skip
- 提供初始 Protected Span
- 指定 Profile / Lexicon / Runtime

**不得通过隐藏规则改变用户已经明确配置的行为。**

### 3.3 可解释

每次文本变化必须能回答 6 个问题（见 [`docs/00`](../docs/00-项目定位与设计原则.md) §2.3）。`Result.RuntimeInfo` 强制包含 Profile / Lexicon / Pipeline / Processor 完整版本信息。

### 3.4 可组合

- Processor 独立可调用
- Pipeline 独立可调用
- 内置与自定义 Processor 接口一致
- 用户可在 Pipeline 中插入任意自定义 Processor

### 3.5 可降级

非核心 Processor 失败 → 保留当前文本 → 记录 Error → `Result.Status = Partial`。**不得默认丢失原始文本**（不变量 10）。

### 3.6 无业务耦合

核心包**禁止**出现 tenant / ASR / OCR / HR / Employee / Meeting / Agent / Document / 产品 等业务词。

---

## 4. 模块设计原则

### 4.1 单一职责

| 模块 | 唯一职责 |
|---|---|
| `Processor` | 一种规范能力 |
| `Pipeline` | 组合 Processor 决定顺序 |
| `State` | 一次请求的工作区 |
| `Lexicon` | 知识容器 |
| `Engine` | 运行环境 Facade |
| `Profile` | 规范化上下文标识 |
| `Result` | 规范化结果 |

**职责越界 = 架构违规**。

### 4.2 依赖倒置

```text
Processor ← Pipeline
       ← Engine
Pipeline ← Engine
Lexicon  ← Engine / Processor / Pipeline
Profile  ← Engine
```

> Engine 持有 Runtime，Runtime 持有 Lexicon / Pipeline / Config / Versions。

### 4.3 不可变优先

| 对象 | 是否可变 |
|---|:--:|
| Lexicon | ❌ Build 后只读 |
| Runtime | ❌ 一次请求锁定 |
| Pipeline | ❌ 推荐只读 |
| Profile | ❌ |
| State | ✅（单 goroutine 独占） |
| Result | ❌ 值语义 |

### 4.4 接口隔离

```go
// 必须实现
type Processor interface {
    Name() string
    Process(context.Context, *State) error
}

// 可选实现
type Versioner interface { Version() string }
type CertaintyReporter interface { Certainty() Certainty }
```

**Optional Interface 通过类型断言**检测，不强制。

---

## 5. 公共 API 设计原则

### 5.1 最小化

`Processor` 接口只有 2 个方法。**绝不**为了方便而增加方法。

### 5.2 向后兼容

v1.0 后**永久冻结**：

- `Processor` 接口
- `Engine.New` / `Engine.Normalize` 签名
- 4 个 error sentinel
- `Span` / `Result` / `Change` 核心字段

可新增：**可选方法 / Optional Interface / 新错误 / 新 Result 字段**。
不可修改：**已存在字段语义 / 已存在方法签名 / 已存在错误含义**。

### 5.3 不变性 + 可推导

```go
// ✅ 不变性
type Span struct { Start, End int }   // 半开区间，Original UTF-8 字节偏移
type Change struct { Span Span; From, To string; Applied bool }

// ✅ 可推导
type Result struct {
    Text        string
    Changes     []Change
    Suggestions []Change       // 推导: Changes where !Applied
    Steps       []Step         // 推导: Change.Meta.StepName
    Err         error          // 推导: errors.Join(Errors...)
}
```

### 5.4 显式优于隐式

```go
// ✅ 显式
WithPipeline(p)            // 用户明确指定
WithProfileID("default")   // 用户明确指定

// ❌ 隐式
WithMagicString("profile=default;lex=v1;pipe=std")  // 不允许
```

---

## 6. 错误设计原则

### 6.1 三层结构

```
sentinel (ErrXxx)         // 静态识别
type-asserted (ProcessorError)  // 携带上下文
fmt.Errorf("...: %w")     // 包装
```

### 6.2 命名

| 类型 | 命名 | 示例 |
|---|---|---|
| Sentinel | `Err<Concept>` | `ErrInvalidConfig` |
| 类型化 | `<Concept>Error` | `ProcessorError` |

### 6.3 不要

- ❌ 在 Error 中携带敏感数据（用户文本 / Token）
- ❌ 用 `err.Error()` 字符串匹配代替 `errors.Is/As`
- ❌ 把 Error 作为正常控制流（应用 `nil` 表示成功）

---

## 7. 性能设计原则

### 7.1 顺序

```
Correctness → Determinism → Benchmark → Optimization
```

**先保证正确性，再谈性能。**

### 7.2 数据结构选型

| 场景 | 推荐 | 避免 |
|---|---|---|
| O(1) 查找 | `map[K]V` | 切片线性扫描 |
| 多模式匹配 | Aho-Corasick | 每个 Pattern 单独匹配 |
| 区间冲突 | `interval.Set` (Sorted Interval Set) | 数组扫描 |
| 高频分配 | `sync.Pool` | 每次 new |

### 7.3 零分配目标

- 热点路径避免 `string` / `[]byte` 频繁转换
- 使用 `unsafe.String` 仅在已验证安全的内部路径
- Benchmark 数据保存在 `docs/14-性能设计与算法优化.md`

### 7.4 基线

| 场景 | 目标 |
|---|---:|
| 45 字 / 2000 词条 | < 200 µs / < 32 KB / < 200 allocs |
| 1000 字 / 10000 词条 | < 3 ms |

---

## 8. 并发设计原则

### 8.1 不可变共享

Engine / Pipeline / Lexicon / Runtime / Profile / Result **可被任意 goroutine 并发共享**。

### 8.2 独占可变

State **仅单 goroutine 独占**。**绝不跨请求共享**。

### 8.3 原子切换

Lexicon 热更新：`atomic.Pointer[Lexicon]`。

### 8.4 不变量

- **不变性**：Lexicon / Runtime / Pipeline 不写
- **独占性**：State 单 owner
- **原子性**：Snapshot 切换原子完成
- **隔离性**：多 Profile 并发互不污染

---

## 9. 可观测性原则

### 9.1 必须暴露的信息

- `Result.RuntimeInfo`：Profile / Lexicon / Pipeline / Processor 完整版本
- `Result.Changes`：所有 Apply + Suggest
- `Result.Suggestions`：仅 Suggest
- `Result.Steps`：每步耗时
- `Result.Errors`：所有错误
- `Change.RuleID` / `EntryID` / `Processor` / `ProcessorVersion`

### 9.2 可选扩展

- Hook（OnEvent）：`PipelineStart` / `ProcessorStart` / `ProcessorEnd` / `PipelineEnd`
- Middleware：Recover / Timing / Timeout / Tracing

### 9.3 不暴露

- 用户文本原文到日志（除非显式开启 Debug）
- 内部中间状态
- 业务侧凭据 / Token

---

## 10. 开源治理原则

### 10.1 License

Apache-2.0。**所有新文件**必须包含 Apache 头部。

### 10.2 第三方依赖

**禁止**。如必须，推到独立 submodule。

### 10.3 兼容性

- v1.0 API 永久冻结
- SemVer 严格遵循
- 任何 breaking change 必须 major bump

### 10.4 公开性

- 公共 README 中英双语
- 每个导出符号有 GoDoc
- Example 函数可运行
- Benchmark 数据公开

---

## 11. 反模式（绝对禁止）

| 反模式 | 说明 | 替代 |
|---|---|---|
| ❌ `tenantID` 入参 | 业务耦合 | `ProfileID` |
| ❌ `asr.Result` 返回 | 业务类型 | `lexnorm.Result` |
| ❌ Processor 内部 `init()` | 隐式全局状态 | 显式 `New()` |
| ❌ `func init()` 注册 Processor | Registry 不强依赖 | `Registry.Register` |
| ❌ 错误吞掉 `return nil` | 违反可降级语义 | 透传或记日志 |
| ❌ `map` 迭代顺序参与输出 | 违反确定性 | `sort.SliceStable` |
| ❌ `math/rand` 用于决策 | 违反确定性 | 确定性 tie-breaker |
| ❌ Goroutine 调度依赖 | 违反确定性 | 显式同步 |
| ❌ `time.Now()` 用于决策 | 违反确定性 | 仅作 Duration 元数据 |
| ❌ `panic` 表示正常错误 | 异常处理错位 | 返回 error |
| ❌ 直接 `strings.ReplaceAll` 改 State | 违反不变量 6 | `State.Replace` |
| ❌ Lexicon 修改 | 违反不变量 5 | 重新 Build |
| ❌ Goroutine 共享 State | 违反不变量 4 | 同步原语 |

---

## 12. 自检清单

任何设计决策前回答：

- [ ] 是否违反 §3 任一原则？
- [ ] 是否违反 §11 任一反模式？
- [ ] 是否引入业务词汇？
- [ ] 是否破坏不变量？
- [ ] 是否依赖全局可变状态？
- [ ] 是否引入第三方依赖？
- [ ] 是否对 v1.0 API 做 breaking change？
- [ ] 是否在 Standard Preset 中包含 LLM（违反 D1）？

---

## 13. 相关文档

- 原则：[`docs/00-项目定位与设计原则.md`](../docs/00-项目定位与设计原则.md)
- 规则：[RULES.md](RULES.md)
- 边界：§2 本文件
- 决策日志：[`../docs/ark-lexnorm-架构设计与开发规范1.2.md`](../docs/ark-lexnorm-架构设计与开发规范1.2.md) §0
