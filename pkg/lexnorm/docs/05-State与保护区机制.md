# 05 · State 与保护区机制

> 源节：§5 State · §6 Span · §7 Protected Span
> 适用阶段：Phase 3
> 受众：核心开发者
> 关键性：**枢纽对象**，所有文本修改的唯一入口

---

## 1. State 定位

State 是单次规范运行的工作状态。

- **State 是 Request Scoped 对象**
- **同一个 State 不允许被多个 Goroutine 并发修改**
- **State 内部字段必须保持私有**

> 这是架构不变量 4（State 不跨请求共享）。

---

## 2. State 公开 API（**1.2 修订**）

```go
// 读
func (s *State) Text() string
func (s *State) Lexicon() Lexicon
func (s *State) Config() Config
func (s *State) IsLocked(span Span) bool

// 写（1.2 决议：改为 error 返回）
func (s *State) Replace(span Span, to string, meta ChangeMeta) error
func (s *State) Suggest(span Span, to string, meta ChangeMeta) error
func (s *State) Lock(span Span) error
```

### 1.0 → 1.2 API 变更

| 方法 | 1.0 | 1.2 | 变更说明 |
|---|:--:|:--:|---|
| `Text()` | ✅ | ✅ | 不变 |
| `Original()` | ✅ | ❌ | **移除**；信息在 `Result.Original` |
| `Profile()` | ✅ | ❌ | **移除**；信息在 `Runtime.Profile` |
| `Lexicon()` | ✅ | ✅ | 不变 |
| `Config()` | ✅ | ✅ | 不变 |
| `Changes()` | ✅ | ❌ | **移除**；通过 `Result.Changes` 暴露 |
| `Replace()` | 无返回 | **error** | **1.2 显式错误返回** |
| `Suggest()` | 无返回 | **error** | **1.2 显式错误返回** |
| `Lock()` | 无返回 | **error** | **1.2 显式错误返回** |
| `IsLocked()` | ✅ | ✅ | 不变 |

### 内部禁止的字段

```go
type State struct {
    // ❌ 不允许出现以下导出字段：
    // Text       string
    // Original   string
    // Changes    []Change
    // LockedSpans []Span
}
```

---

## 3. NewState 构造

```go
state := lexnorm.NewState(
    text,
    lexnorm.WithProfile("default"),
    lexnorm.WithLexicon(lexicon),
)
```

### 构造选项

| Option | 必填 | 说明 |
|---|:--:|---|
| `WithProfile(name)` | 否（默认 `default`） | Profile 标识 |
| `WithLexicon(lex)` | 否（可后续注入） | Lexicon 引用 |
| `WithConfig(cfg)` | 否 | Config |
| `WithInitialLocks(spans)` | 否 | 初始保护区 |

### 注意

- NewState 不依赖 Engine
- 同一 State 的所有调用必须来自同一 goroutine
- 1.2 起 `Profile` 选项实际对应 `Runtime.Profile` 注入（State 仅持有引用，不再有 `Profile()` 方法）

---

## 4. Replace（**§30 + 1.2 修订**）

`Replace` 是**文本规范一致性的核心入口**。

### 所有 Processor 的文本修改必须经过

```go
state.Replace(...)
```

### Replace 统一负责

```text
Span
Protection
Change
Offset
Decision
Provenance
```

> 这避免各 Processor 重复实现文本修改逻辑，
> 消除"先猜对后被改错"级联问题。

### Replace 内部流程（实现要点）

```text
1. 检查 span 边界合法性
2. 检查 span 是否与现有 Locked Span 重叠
   └─ 是：返回 ErrInvalidSpan（不允许覆盖已确定区间）
3. 写入 ChangeApplied（Applied = true）
4. 维护 Original → Text 的偏移映射
5. 把 Locked Span 区间内的已有 Change 标记为 applied
6. 返回 nil
```

### 签名（1.2 修订）

```go
type ChangeMeta struct {
    Kind       ChangeKind
    Source     string
    RuleID     string         // 1.1 新增
    EntryID    string         // 1.1 新增
    Confidence float64
    Reason     string
}

func (s *State) Replace(span Span, to string, meta ChangeMeta) error
```

### 错误返回

`Replace` 在以下场景返回非 nil error：

| 场景 | error |
|---|---|
| span 越界 | `ErrInvalidSpan` |
| span 与 Locked 区间重叠 | `ErrConflict` |
| Confidence 非法 | `ErrInvalidConfig` |
| `to` 与 `s.Text()[span.Start:span.End]` 长度不匹配（可选实现） | `ErrInvalidSpan` |

Processor 应当：

```go
if err := state.Replace(span, to, meta); err != nil {
    return &lexnorm.ProcessorError{
        Name: p.Name(),
        Op:   "replace",
        Err:  err,
    }
}
```

---

## 5. Suggest

Suggest 记录候选改写但不直接应用。

### 签名（1.2 修订）

```go
func (s *State) Suggest(span Span, to string, meta ChangeMeta) error
```

### Suggest 与 Replace 的区别

| 维度 | Replace | Suggest |
|---|---|---|
| 是否立即改文本 | 是 | 否 |
| Change.Applied | `true` | `false` |
| 进入 Locked Span | 是 | 否 |
| 后续 Processor 是否跳过 | 是 | 否 |
| 失败时 error | `ErrInvalidSpan` / `ErrConflict` | 通常 `nil`（除非元数据非法） |

---

## 6. Lock 与 Protected Span

### 保护区语义

> 已确定的文本区域，不允许低确定性 Processor 继续修改。

### 示例

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

**不得覆盖该区域。**

### Lock API（1.2 修订）

```go
func (s *State) Lock(span Span) error
func (s *State) IsLocked(span Span) bool
```

### Lock 自动触发时机

1. Processor 调用 `state.Replace` 后，**自动 Lock 该 span**
2. 用户可在初始构造时 `WithInitialLocks` 预 Lock

### Lock 冲突规则

| 场景 | 行为 |
|---|---|
| Lock 已 Lock 区间内重复 Lock | 允许（幂等） |
| Lock 未 Lock 区间 | 允许 |
| Replace 命中 Locked 区间 | **返回 `ErrConflict`**（1.2 显式错误） |
| Suggest 命中 Locked 区间 | 允许（写入 Change 但 Applied=false，不锁） |

### 1.2 与 1.0 行为差异

| 场景 | 1.0 | 1.2 |
|---|---|---|
| Replace 命中 Locked | 静默忽略或 panic（实现不一致） | **返回 `ErrConflict`** |
| Suggest 命中 Locked | 写入 Applied=false | 写入 Applied=false（一致） |

> **1.2 收益**：错误显式化，Processor / Pipeline 可统一处理。

---

## 7. Span 与区间

### Span 定义

```go
type Span struct {
    Start int
    End   int
}
```

### 约定

- **半开区间**：`[Start, End)`
- **Original UTF-8 字节偏移**：Span 始终指向 `Original` 文本的偏移，不指向 `Text` 当前偏移（**1.2 决议：吸收 1.0 §23 严格表述**）

### 高效区间结构

Protected Span 必须使用高效区间结构：

- 推荐：**Sorted Interval Set**（有序区间集合 + 二分）
- 目标：`IsLocked = O(log n)`
- 避免：`O(n²)` 线性扫描

> 实现见 `internal/interval/`。

---

## 8. 内部偏移映射（State 私有）

State 内部维护：

```text
Original[i:j)  →  Text[i':j')   // 一次 Replace 后的偏移
```

后续 Replace / Lock 引用 `Original` 偏移时，由 State 转换。

### Processor 视角

```go
// ✅ 正确：基于 Original 偏移
sp := state.Lexicon().Lookup(text)
state.Replace(Span{Start: 0, End: 3}, "新文本", meta)

// ❌ 错误：手动维护 Text 当前偏移
```

---

## 9. Change 累积

State 内部累积所有 Change，包括：

- Applied（来自 Replace）
- Suggested（来自 Suggest）

最终在 Engine / Pipeline 完成后打包为 `Result.Changes` 与 `Result.Suggestions`。

> **1.2 行为**：`Result.Changes` 包含全部；`Result.Suggestions` 仅含 Applied=false。

### 完整 Change 字段

见 [02-核心领域模型](02-核心领域模型.md) §12。

---

## 10. State 生命周期

```text
NewState
   ↓
   ├─ Processor A.Read State
   ├─ Processor A.Calculate
   ├─ Processor A.state.Replace(...)
   ├─ Processor B.Read State
   ├─ Processor B.Calculate
   ├─ Processor B.state.Suggest(...)
   └─ ...
   ↓
Pipeline 完成后：
   ↓
state.Result() → Result 值
   ↓
State 丢弃
```

---

## 11. 自检清单

- [ ] State 是否仍保留导出字段？
- [ ] Replace 是否检查了 Locked Span 重叠并返回 `ErrConflict`？
- [ ] Suggest 是否跳过了偏移维护？
- [ ] IsLocked 是否为 O(log n)？
- [ ] Span 偏移是否错误使用了 rune 索引？
- [ ] State 是否被多个 goroutine 共享？
- [ ] 是否遗漏了 Original/Profile/Changes 方法的移除（1.2 决策）？
- [ ] Processor 是否正确处理 `Replace` 返回的 error？

---

## 12. 相关文档

- 上游：[02-核心领域模型](02-核心领域模型.md) §9
- Processor 写入：[03-Processor接口与生命周期](03-Processor接口与生命周期.md) §3
- 不可变结果：[02 §18](02-核心领域模型.md#18-结果不可变原则)
- 性能：[14-性能设计与算法优化](14-性能设计与算法优化.md) §5
- 错误：[10-配置校验与错误体系](10-配置校验与错误体系.md) §1
