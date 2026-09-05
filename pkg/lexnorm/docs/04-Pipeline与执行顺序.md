# 04 · Pipeline 与执行顺序

> 源节：§4.2 Pipeline · §21 Preset · §27 Match 冲突规则（局部）
> 适用阶段：Phase 2, 6
> 受众：核心开发者 + 业务开发者

---

## 1. Pipeline 定义

Pipeline 是 **Composite Processor**。

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

**Pipeline 不应该知道 Processor 的具体业务含义。**

---

## 2. Pipeline 接口（**1.2 升级为 interface**）

```go
type Pipeline interface {
    Processor
    Processors() []Processor
}

// 默认实现（包内）
type pipeline struct {
    processors []Processor
}

func NewPipeline(processors ...Processor) Pipeline
```

### 1.0 → 1.2 关键变化

| 版本 | Pipeline 类型 | 影响 |
|---|---|---|
| 1.0 | `struct`（具体类型） | 用户不能自定义 Pipeline；只能使用 `lexnorm.NewPipeline(...)` |
| **1.2** | **`interface`** | **用户可实现自定义 Pipeline**（条件分支 Pipeline、并行 Pipeline、动态裁剪 Pipeline 等） |

### 关键约束

- Pipeline **必须**实现 `Processor` 接口（架构不变量 2）
- `Processors()` 只读快照，外部不能修改 Pipeline 持有的 processor 列表
- Pipeline 实现可放在任何位置（用户仓库、第三方包），不依赖 Engine / Registry

### 自定义 Pipeline 示例

```go
// 条件分支 Pipeline：根据文本长度选择不同分支
type ConditionalPipeline struct {
    shortBranch Pipeline
    longBranch  Pipeline
    threshold   int
}

func (p *ConditionalPipeline) Name() string {
    return "conditional"
}

func (p *ConditionalPipeline) Processors() []Processor {
    return []Processor{p.shortBranch, p.longBranch}  // 装饰性
}

func (p *ConditionalPipeline) Process(ctx context.Context, s *lexnorm.State) error {
    if len(s.Text()) < p.threshold {
        return p.shortBranch.Process(ctx, s)
    }
    return p.longBranch.Process(ctx, s)
}

// 接入 Engine
eng, _ := lexnorm.New(
    lexnorm.WithPipeline(&ConditionalPipeline{
        shortBranch: lexnorm.NewPipeline(clean.New(), alias.New(lex)),
        longBranch:  lexnorm.NewPipeline(clean.New(), alias.New(lex), fuzzy.New(cfg)),
        threshold:   100,
    }),
)
```

---

## 3. 自定义 Pipeline

用户**完全控制**流程。

### 任意组合

```go
pipeline := lexnorm.NewPipeline(
    clean.New(),
    alias.New(lexicon),
    fuzzy.New(config),
)
```

### 只使用一个 Processor

```go
pipeline := lexnorm.NewPipeline(
    alias.New(lexicon),
)
```

### 甚至单步

```go
pipeline := lexnorm.NewPipeline(
    fuzzy.New(config),
)
```

**不要求必须运行完整 Standard Preset。**

---

## 4. Order 与 Certainty（**§27**）

### Order 是显式的

```go
NewPipeline(A, B, C)   // 必然 A → B → C
```

### Certainty 辅助 Preset 设计

Certainty 不参与运行时排序，仅在 Standard Preset 设计时参考。

### 原则

> **显式 Pipeline Order > 默认推荐 Order > Certainty 推导**

**禁止**：

- Pipeline 根据 Certainty 隐式重排用户指定的顺序
- Engine 在运行期按 Certainty 自动调整 Pipeline

> 详见 [02-核心领域模型](02-核心领域模型.md) §15。

---

## 5. Match 冲突下的执行顺序（**§22，1.2 补回**）

当多个候选在同一文本位置命中时，Processor 必须按以下规则消解：

### 默认：Longest Match First

```text
Text:    "张三丰张"
Lexicon: [
    {Canonical: "张三丰", Text: "张三丰"},
    {Canonical: "张三",   Text: "张三"},
]

→ 命中 [0, 6) → "张三丰"  ← Longest 优先
```

### 相同长度：Higher Priority

```text
Lexicon: [
    {Canonical: "周丽群", Priority: 1.0, Text: "周丽群"},
    {Canonical: "周丽群", Priority: 0.8, Text: "周丽群"},
]

→ 优先 Priority=1.0 的版本
```

### 仍相同：Stable Lexicographical Ordering

```text
Lexicon: [
    {Canonical: "周丽群", Priority: 1.0, Text: "周丽群", Order: "1"},
    {Canonical: "周丽群", Priority: 1.0, Text: "周丽群", Order: "2"},
]

→ 优先 Order="1"（Stable Lexicographical）
```

> **完整规则见 [11-确定性与匹配冲突消解](11-确定性与匹配冲突消解.md)**。

---

## 6. Preset（§21）

Preset 是**标准 Pipeline 模板**。

**Preset 不是 Engine 的硬编码流程。**

### 标准用法

```go
pipeline := lexnorm.NewPipeline(
    preset.Default()...,
    // 或 preset.HighAccuracy()... / preset.Fast()...
)
```

### 约束

- Preset **不得**修改 Engine 状态
- Preset **不得**注入全局副作用
- Preset **不得**在内部隐式注册 Middleware / Hook
- 业务开发者可以**完全不用 Preset**

### 包含的 Preset

| Preset | 说明 |
|---|---|
| `preset.Default()` | 7 步默认流程（不含 LLM，**D1 决议**） |
| `preset.HighAccuracy()` | 高精度场景：Pinyin + Fuzzy + Context 全部启用 |
| `preset.Fast()` | 快速场景：仅 Clean + Alias + Deterministic |
| `preset.ASR()` | ASR 场景特化（含 Disfluency） |
| `preset.OCR()` | OCR 场景特化 |

---

## 7. Standard Pipeline（**D1 决议修订**）

### 1.2 标准流程（**不含 LLM**）

```text
1. Normalize
2. Disfluency
3. Alias
4. Deterministic
5. Pinyin
6. Fuzzy
7. Context
```

### LLM

```text
Optional Processor
```

**不作为核心依赖**，**不在 Standard Preset 内**。

> **D1 决议**：1.1 把 LLM 列入"推荐默认顺序"第 8 位会造成误导，1.2 修正为可选扩展。1.0 的明确表述被采纳。

### 用户可对每一步进行

- 启用
- 禁用（不列出该 processor 即可）
- 替换（同名自定义实现）
- 插入
- 删除
- 自定义

---

## 8. 典型 Standard 流程

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

- 启用
- 禁用
- 替换
- 插入
- 删除
- 自定义

---

## 9. ErrorPolicy

```go
type ErrorPolicy uint8

const (
    ContinueOnError ErrorPolicy = iota   // 默认
    FailFast
)
```

### ContinueOnError（默认）

```text
Processor Error
    ↓
记录错误
    ↓
继续后续 Processor
    ↓
Result.Status = Partial
```

### FailFast

```text
Processor Error
    ↓
立即返回
    ↓
Result.Status = Canceled 或 Failed
```

### API 形态

```go
pipeline := lexnorm.NewPipeline(
    processors...,
    lexnorm.WithErrorPolicy(lexnorm.FailFast),
)
```

### 与 Result 的关系

| ErrorPolicy | 单步 error 时 Result.Status | Result.Errors |
|---|---|---|
| `ContinueOnError` | `Partial` | error 追加到 `Errors` |
| `FailFast` | `Canceled` 或 `Failed` | error 写入 `Errors` 后停止 |

---

## 10. Pipeline 测试规范

Pipeline 测试必须验证：

| 验证项 | 含义 |
|---|---|
| Processor Order | 实际执行顺序与声明一致 |
| Error Policy | ContinueOnError / FailFast 行为符合预期 |
| Protection | Lock 的区间不被下游 Processor 修改 |
| Changes | Change 列表的 Span / From / To 正确 |
| Result | Text / Status / Duration / Steps 正确 |
| Match Conflict | 同一位置多个候选按 Longest → Priority → Lex 消解 |

### 关键不变量

- **Pipeline Processors() 顺序 = 实际执行顺序**
- **同一 Pipeline 多次 Process 不共享 State**
- **ContinueOnError 模式下，单步错误不阻断后续步骤**

---

## 11. 自检清单

- [ ] Pipeline 是否按 1.2 interface 实现（而非 1.0 struct）？
- [ ] Pipeline 是否实现了 Processor 接口？
- [ ] Preset 是否避免了修改 Engine 状态？
- [ ] 是否依赖 Certainty 隐式重排用户顺序？
- [ ] ErrorPolicy 默认是否为 ContinueOnError？
- [ ] Pipeline Processors() 是否返回只读快照？
- [ ] 测试是否覆盖了 ContinueOnError 与 FailFast 两种路径？
- [ ] 测试是否覆盖了 Match 冲突的三层规则？
- [ ] LLM 是否被误列入 Standard Preset（D1 决议）？

---

## 12. 相关文档

- 上游：[03-Processor接口与生命周期](03-Processor接口与生命周期.md)
- 下游：[05-State与保护区机制](05-State与保护区机制.md)
- Match 冲突完整规则：[11-确定性与匹配冲突消解](11-确定性与匹配冲突消解.md)
- 场景模板：[13-应用场景与Pipeline模板](13-应用场景与Pipeline模板.md)
- 错误体系：[10-配置校验与错误体系](10-配置校验与错误体系.md)
