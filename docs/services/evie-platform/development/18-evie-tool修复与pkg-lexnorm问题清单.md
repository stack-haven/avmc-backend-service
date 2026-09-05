# evie/tool 修复后回归 & pkg/lexnorm 问题清单

> 本文档记录 evie/tool 已修复的 bug + 重新发现的需要在 **pkg/lexnorm** 单独修复的问题。
> 修复日期：2026-09-05。

---

## A. evie/tool 已修复的 bug（本次提交）

| ID | 问题 | 修复 | 文件 |
|---|---|---|---|
| **P5** | convertRawToVocab 丢失 Priority 字段 | 加 `Priority: int(n.Priority)` | `app/evie/tool/internal/biz/vocab_sync.go` |
| **P1** | 冷启动首请求 cache miss 走 system fallback | Build cache miss 时同步等待 lazySyncOnMiss（5s timeout + ctx cancel） | `app/evie/tool/internal/biz/vocabulary.go` |
| **P2** | lexnorm.Config 默认 AutoApplyThreshold=0.95 拦截 PERSON 0.65 替换 | biz.NewLexnormEngine 设 AutoApplyThreshold=0.5、SuggestThreshold=0.0；让 processor 自主决策 | `app/evie/tool/internal/biz/biz.go` |
| **P6** | fuzzy_vocab 在已被 alias/deterministic 命中过的同区间重复触发 | fuzzy_vocab 改用 `s.Changes()` 检查同 span 已应用，避免重复提议 | `app/evie/tool/internal/biz/processor/fuzzy_vocab.go` |

### 验证结果（5 次真实晨会录音）

```
run  changes    raw md5    enh md5   alias  fuzzyR  fuzzyS  filler
   1       72   9025d7d3   ae62602b    19      2       6       8
   2       72   6a139ae1   d625d012    18      2       7       8
   3       72   ea13c7f7   d1643722    18      2       7       8
   4       72   4ed45bee   5cca131d    18      2       7       8
   5       72   ea0ca0b5   4f499b3a    19      2       6       8

✓ fuzzy replace 5/5 命中：周丽群→佘丽群、测试播→测试1（conf=0.67）
✓ alias 稳定：13-14 次/次 金种子→金种籽
✓ filler 稳定：8 次/次
✓ 72 changes/次 稳定
✓ enh md5 各异（funasr 抖动，非 lexnorm 问题）
```

### 单元测试

12 个 evie/tool 测试包 + 16 个 lexnorm 测试包全绿（含 race detector）。

---

## B. pkg/lexnorm 引擎问题（**待 lexnorm 单独修复**）

下面 4 个问题都在 `backend-service/pkg/lexnorm/` 内，需要单独 PR 修复。
**当前 evie/tool 通过 hack 绕过：convertChanges 按 `Change.Source` 推断 type，而不是 `Change.Processor`。**

### B1. Change.Processor 字段为空字符串（**核心问题**）

**位置**：`pkg/lexnorm/state.go`（Replace / Suggest / Rewrite 三处）

```go
// state.go:243（Replace）
Processor:        "", // filled by Engine when running through Pipeline
ProcessorVersion: "",
```

**问题**：注释承诺"由 Engine 填充"，但 Engine 实际**没有填充**：

- `pkg/lexnorm/engine.go:runProcessors` 跑每个 processor 时只填 `StepTiming.Processor`，没 reverse-fill 到 Change
- `pkg/lexnorm/engine.go:buildResult` 也只是把 `s.Changes()` 拷到 Result.Changelog，**无补全**

**影响**：
- 业务层 `convertChanges` 拿到的 Change.Processor 是空字符串，必须按 `Change.Source` 反推
- audit trail 不完整：无法从 Change 反查是哪个 processor 触发
- 违反 `pkg/lexnorm/processor.go:39` 注释承诺：`Name() 用于 Change.Processor`

**建议**：

方案 A（推荐）：Engine.runProcessors 在 processor 跑完后，遍历 s.changes 把当前 proc.Name() 写到这段 processor 新增的 Change.Processor 字段。

```go
// engine.go runProcessors 里
changesBefore := len(s.changes)
proc.Process(ctx, s)
for i := changesBefore; i < len(s.changes); i++ {
    s.changes[i].Processor = proc.Name()
    if v, ok := proc.(Versioner); ok {
        s.changes[i].ProcessorVersion = v.Version()
    }
}
```

方案 B：在 buildResult 里 reverse-fill（更慢，但要拿到 changes slice 后才有 Processor 信息）。

### B2. fuzzy processor 自身的 conf 字段缺失

**位置**：`pkg/lexnorm/processor/fuzzy/fuzzy.go`

虽然 Change.Meta 有 `Confidence` 字段，但 lexnorm 自家的 fuzzy processor 算 conf 时依赖预登记 Variant 的 Confidence。Variant 没显式设 conf 时默认 1.0。这本身没问题，但应该文档化。

### B3. SuggestThreshold = 0 时的边界行为

**位置**：`pkg/lexnorm/config.go:Validate`

```go
if c.SuggestThreshold > c.AutoApplyThreshold {
    return fmt.Errorf("SuggestThreshold (%v) > AutoApplyThreshold (%v): %w", ...)
}
```

**当前行为**：biz 设 `SuggestThreshold=0.0` 通过 Validate，但语义上 Suggest=0 等于 "任何 Suggest 调用都通过"，失去意义。

**建议**：Config 增加语义化标志，或文档化"0 = 不过滤"。

### B4. StepTiming.ChangeCount 计算时机

**位置**：`pkg/lexnorm/engine.go:runProcessors`

```go
timing := StepTiming{
    Processor:   proc.Name(),
    Duration:    stepDuration,
    ChangeCount: len(s.changes) - changesBefore,
}
```

`ChangeCount` 计的是 proc 跑完后 **新增** 的 change 数量。但如果 proc 内部 Sug + Replace 混用（多个调用），它计的是**累计**变化数，不是 Replace 数量也不是 Suggest 数量。

**建议**：区分 Applied / Suggested 计数。

---

## C. evie/tool 已知的非 P1-P6（**保留**，本轮不动）

| ID | 问题 | 状态 |
|---|---|---|
| P3 | 服务定位 vs 命名错位（Enhancement → Normalization） | 保留，按用户决策"重构取消" |
| P4 | pkg/lexnorm 命名约束被破坏（TenantProfileResolver / PERSON） | 保留，scope外 |

---

## D. 建议修复优先级

| 优先级 | 问题 | 影响范围 |
|---|---|---|
| 🔴 P0 | B1 Change.Processor 字段为空 | 所有调用方 |
| 🟡 P1 | B4 ChangeCount 区分 applied/suggested | observability |
| 🟢 P2 | B2 fuzzy conf 文档化 | 文档 |
| 🟢 P3 | B3 SuggestThreshold=0 语义 | 配置边界 |

请单独 PR 修复 pkg/lexnorm 这 4 个问题。evie/tool 侧已通过 hack 兼容运行。