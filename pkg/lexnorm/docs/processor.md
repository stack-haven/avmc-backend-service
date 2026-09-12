# Processor 体系规范（v1.1）

> 本文是 ark-lexnorm Processor 体系的正式规范入口。架构细节见
> [architecture.md](architecture.md)，示例见 [examples.md](examples.md)。

---

## 1. 正式定义

**Processor 是最小执行单元**：在一个确定的规范化上下文中，基于给定的词法知识，
对文本执行一类可组合、可解释、可追溯的规范化操作。

核心接口保持最小化（冻结于 v1.0）：

```go
type Processor interface {
    Name() string
    Process(context.Context, *State) error
}
```

可选能力通过**可选接口**表达，第三方 Processor 不被强制实现任何额外方法：

| 可选接口 | 作用 |
|---|---|
| `Versioner` | 声明实现版本 |
| `CertaintyReporter` | 声明确定性等级 |
| `DescribedProcessor` | 声明完整能力 `Descriptor`（v1.1 新增） |

---

## 2. 八大分类（Category）

```go
type Category string

const (
    CategoryNormalization  Category = "normalization"   // 基础表示统一
    CategoryNoise          Category = "noise"           // 无语义价值文本
    CategoryCanonical      Category = "canonicalization"// 合法表达归一
    CategoryDeterministic  Category = "deterministic"   // 确定性纠错
    CategoryPhonetic       Category = "phonetic"        // 语音/拼音纠错
    CategoryApproximate    Category = "approximate"     // 近似/模糊纠错
    CategoryContextual     Category = "contextual"      // 上下文消歧
    CategorySemantic       Category = "semantic"        // 模型/生成式修正
)
```

分类语义要点：

- **Normalization**：Unicode、全半角、空白、控制字符。不表达语义纠错。
- **Noise**：口头语、语气词、填充词、重复词/短语。**不得无条件删除可能带语义的词**
  （默认策略：单字叹词无条件删除；多字歧义词仅在独立出现（边界包围）时删除）。
- **Canonicalization**：Alias / Nickname / Abbreviation / 称呼 / 变体 → 统一表示。
  原表达并非"错误"。
- **Deterministic**：有明确规则/词条来源的稳定映射。
- **Phonetic**：同音、音近、拼音相似。**分类不绑定中文拼音**——拼音只是其一实现。
- **Approximate**：编辑距离、n-gram、相似度召回。默认优先产生候选（Suggest）。
- **Contextual**：上下文窗口、候选排序、消歧。无法确定时 Suggest 或 Skip。
- **Semantic**：LLM / 本地模型 / 生成式修正。低确定性、高风险、高成本，默认居尾。

业务无关边界：分类名不得引入 Tenant / ASR / HR / Meeting 等业务概念。

---

## 3. Category 与内置实现映射

| 内置实现 | Category | 包 |
|---|---|---|
| NormalizeProcessor | `normalization` | `processor/normalize` |
| DisfluencyProcessor | `noise` | `processor/disfluency` |
| AliasProcessor | `canonicalization` | `processor/alias` |
| DeterministicProcessor | `deterministic` | `processor/deterministic` |
| PinyinProcessor | `phonetic` | `processor/pinyin` |
| FuzzyProcessor | `approximate` | `processor/fuzzy` |
| ContextProcessor | `contextual` | `processor/ctxproc` |
| LLMProcessor | `semantic` | `processor/llm` |

---

## 4. Descriptor 设计（v1.1）

`lexnorm.Descriptor` 同时承担**构造描述**（Registry 用）与**能力描述**（审计/文档/装配用）：

```go
type Descriptor struct {
    // 构造（既有字段，兼容不变）
    Name      string
    Certainty Certainty
    New       func(cfg json.RawMessage) (Processor, error)
    Default   func() any

    // 能力（v1.1 追加，零值 = 未声明）
    Version           string
    Category          Category
    MutatesText       bool
    SupportsSuggest   bool
    SupportsProtected bool
    Deterministic     bool
    Determinism       Determinism // deterministic / probabilistic / generative
    DefaultOrder      int         // 仅用于默认装配与文档展示，绝不重排用户管线
    Description       string
}
```

读取任意 Processor 的能力元数据（带回退）：

```go
d, fullyDeclared := lexnorm.DescriptorOf(proc)
```

维度视图：`Descriptor.Mutation()` 返回 `MutationNone / MutationSuggest /
MutationApply / MutationMixed`。

---

## 5. 独立使用 Processor

Processor 不依赖 Engine 即可运行：

```go
lex, _ := lexicon.NewBuilder().Add(entries...).Build()
state, _ := lexnorm.NewState(ctx, "小田今天帮我查一下个种子的情况", lex, lexnorm.DefaultConfig())

proc := alias.New(lex)
_ = proc.Process(ctx, state)
fmt.Println(state.Text(), state.Changes())
```

每个内置 Processor 都有独立运行测试（`TestXxx_IndependentOfEngine`）。

---

## 6. Pipeline 组合

```go
// 直接组合（内置与自定义平等）
pipeline := lexnorm.NewPipeline(normalize.New(), custom, alias.New(lex))

// 组合助手（返回新 Pipeline，原对象不变）
p2, err := lexnorm.ReplaceProcessor(pipeline, "alias", aliasV2)
p3, err := lexnorm.RemoveProcessor(pipeline, "fuzzy")
p4 := lexnorm.AppendProcessor(pipeline, myProc)
```

**顺序规则**（优先级从高到低）：

```
显式用户 Pipeline Order > Preset Order > DefaultOrder > Certainty
```

用户显式指定后：不自动重排、不因 Certainty/Category 改序、不隐藏插入。

默认 Pipeline（Standard Preset）按八大分类排序：

```
Normalization → Noise → Canonicalization → Deterministic
→ Phonetic → Approximate → Contextual → (Semantic，默认不含，见 D1)
```

---

## 7. Apply / Suggest / Skip

处理器按置信度阈值三级决策（阈值来自 Config，可配置）：

| 条件 | 行为 |
|---|---|
| confidence ≥ `AutoApplyThreshold`（默认 0.95） | `State.Replace`（Applied=true） |
| `SuggestThreshold`（默认 0.65）≤ confidence < AutoApply | `State.Suggest`（Applied=false） |
| confidence < `SuggestThreshold` | Skip（不产生 Change） |

 Approximate / Phonetic 类默认建议阈值以下全部 Suggest，不直接改文本。

---

## 8. Protected Span

- `State.Lock(span)` 保护区间；`Replace` 在锁区/已替换区/包含已替换区间的 span 上
  返回 `ErrConflict`，`Suggest` 不受锁限制。
- 内置处理器通过 State 的冲突检查天然尊重 Protected Span。
- 高确定性处理器产出的修改（已替换区域）不会被低确定性处理器覆盖。

---

## 9. 实现约束（所有 Processor）

```
候选发现 → 候选验证 → Protected Span 检查 → 置信度计算
→ Decision → State.Replace / State.Suggest → Change
```

- 禁止 `strings.ReplaceAll` / `regexp.ReplaceAllString` 作为最终修改入口
  （可用于候选发现）。
- Change 必含：Span / From / To / Action / Kind / Source / Processor /
  ProcessorVersion / Confidence / Applied / Reason；可追溯时含 RuleID / EntryID。
- 词库登记卫生（`lexicon.Builder.Validate()`）：拒绝 variant 为 canonical 子串、
  variant 撞其他条目 canonical、跨条目重复 variant。

---

## 10. 自定义 Processor + Descriptor 示例

最小实现只需 `Processor`；声明能力请追加 `DescribedProcessor`：

```go
type Redactor struct{}

func (Redactor) Name() string { return "redactor" }
func (Redactor) Process(_ context.Context, s *lexnorm.State) error {
    // ... 候选发现后经 State.Replace 写回
    return nil
}

// 可选：能力声明
func (Redactor) Descriptor() lexnorm.Descriptor {
    return lexnorm.Descriptor{
        Name: "redactor", Category: lexnorm.CategoryDeterministic,
        Certainty: lexnorm.CertaintyHigh, MutatesText: true,
        Deterministic: true, Determinism: lexnorm.DeterministicTrue,
    }
}
```

Registry 方式（可选，不是使用前置条件）：

```go
reg := lexnorm.NewRegistry()
reg.Register(alias.Descriptor)
proc, err := reg.Build("alias", nil)
```

完整可运行示例见 [examples.md](examples.md) 与 `example/04-custom-processor`。

---

## 11. LLM / Semantic 接入

- `processor/llm` 是 Semantic 分类占位边界：无 SDK / API Key / 计费耦合。
- 应用侧实现 `Client` 接口（见包文档）后构造自定义 Processor 加入 Pipeline 尾部。
- D1：LLM **不在** Standard Preset。

---

## 12. 测试要求对照

| 要求 | 位置 |
|---|---|
| 分类测试 | `classification_test.go` |
| Descriptor 测试 | `descriptor_capability_test.go` |
| Pipeline 测试（顺序/插入/替换/删除/不重排） | `pipeline_test.go`、`p5_features_test.go` |
| 独立运行测试 | 各 `processor/*` 包 |
| Protected Span 测试 | `state_test.go`、`hardening_test.go` |
| Determinism 测试 | `state_test.go`、`engine_test.go`、golden 语料 |
| Failure 测试（Error/Panic/Cancel/Partial/原文不丢） | `error_policy_test.go`、`noise_phonetic_test.go` |
| Race | `go test -race ./...` |
| 行为基线 | `golden_test.go` + `testdata/golden/` |
