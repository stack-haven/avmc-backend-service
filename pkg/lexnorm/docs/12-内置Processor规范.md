# 12 · 内置 Processor 规范

> **v1.1 行为同步（2026-09）**：本文各 Processor 小节的行为描述以「v1.1」标注
> 为准，与代码当前实现一致；未标注处为 1.2 基线描述。分类归属（Category）与
> 能力元数据见 `docs/processor.md`。

> 源节：§34 LLM · §35 内置 Processor · §36~§42 各 Processor 详情
> 适用阶段：Phase 5
> 受众：核心开发者 + 学习者（用 ark-lexnorm 解决业务问题的人）

---

## 1. 内置 Processor 清单（§35）

| # | Processor | 包路径 | 确定性 |
|:--:|---|---|:--:|
| # | 处理器 | 包路径 | Category | Certainty | DefaultOrder |
|:--:|---|---|---|---|:--:|
| 1 | Normalize | `processor/normalize` | `normalization` | high | 1 |
| 2 | Disfluency | `processor/disfluency` | `noise` | high | 2 |
| 3 | Alias | `processor/alias` | `canonicalization` | high | 3 |
| 4 | Deterministic | `processor/deterministic` | `deterministic` | high | 4 |
| 5 | Pinyin | `processor/pinyin` | `phonetic` | medium | 5 |
| 6 | Fuzzy | `processor/fuzzy` | `approximate` | medium | 6 |
| 7 | Context | `processor/ctxproc` | `contextual` | low | 7 |
| 8 | LLM | `processor/llm` | `semantic` | low | 8（可选） |

> LLM 作为扩展能力，**不在 Standard Preset 内**。

---

## 2. Normalize Processor（§36）

**包路径**：`processor/normalize`（1.2 重命名：原 `clean/`）

### 职责

负责基础文本归一。

### 典型能力

```text
控制字符
不可见字符
异常空白
重复标点
Unicode 基础归一（NFKC）
```

### 确定性

```text
Deterministic
```

### 配置示例

```yaml
processors:
  - name: clean
    config:
      strip_control: true
      strip_zero_width: true
      collapse_whitespace: true
      collapse_punctuation: true
      max_punct_repeat: 3
      unicode_form: "NFKC"  # NFKC / NFC / NFD / NFKD / none
```

---

## 3. Disfluency Processor（§37）

**包路径**：`processor/disfluency`

### 职责

负责处理不流畅文本成分。

### 典型词（v1.1 两级词表）

```text
无条件删除（无歧义单字叹词）：
呃 嗯 啊 哦 诶

边界守卫删除（多字歧义词：仅在两侧均为边界——标点/空白/首尾——时删除）：
那个 这个 然后 就是说 其实 反正 你知道
```

> **规范依据**：§37 / 规范化指令 §2 Noise——「那个文件给我」中的"那个"
> 不得被无条件删除（它是指示词，直接修饰名词）。该反例已进入回归测试。
>
> **逃生门**：`WithAggressiveFillers()` 一键恢复旧行为（全词表无条件删除）；
> `WithTokens(...)` 显式覆盖词表（调用方自担语义责任）。

### 关键约束

> 规则**必须可配置**。
>
> **不得把具体业务规则硬编码为核心知识**；**不得把所有口语词默认删除**。

### 重复词 / 重复短语（v1.1 新增）

- 连续重复字符（≥3 个相同字）与连续重复短语（2~4 字短语重复 ≥3 次）
  **只产生 Suggest**（折叠建议，默认不改文本）——规避「哈哈哈哈」类合法
  重复被误折叠；`WithRepeatedWordFolding(n)` / `WithRepeatedPhraseFolding(len, n)`
  可调参或关闭。

### 删除与空格

删除填充词时吸附**一个相邻空格**进入删除 span，避免输出双空格。

### 配置示例

```yaml
processors:
  - name: disfluency
    config:
      tokens: ["呃", "额", "嗯", "啊"]   # 显式覆盖 = 无条件删除（历史契约）
      action: remove   # remove / suggest
```

---

## 4. Alias Processor（§38）

**包路径**：`processor/alias`

### 职责

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

### 数据来源

Lexicon（`VariantKind = VariantAlias`）。

### 配置示例

```yaml
processors:
  - name: alias
    config:
      lexicon_ref: "default"
      auto_threshold: 0.95
```

### 实现要点

- 使用 Aho-Corasick 一次性扫描
- 命中后 `state.Replace`
- 自动 Lock 命中区间

---

## 5. Deterministic Processor（§39）

**包路径**：`processor/deterministic`

### 职责

负责明确的确定性映射：

```text
From → To
```

例如：

```text
个种籽 → 颗种籽
```

### 关键约束

> Processor 本身**不理解业务语义**。

### 配置示例

```yaml
processors:
  - name: deterministic
    config:
      rules:
        - from: "个种籽"
          to: "颗种籽"
        - from: "的的"
          to: "的"
```

> 业务规则从代码移到配置（`§60 原则`）。

---

## 6. Pinyin Processor（§40）

**包路径**：`processor/pinyin`

### 职责

负责同音、拼音近似规范。

### 依赖

```text
Lexicon
Pinyin Index
```

### 结果必须能够提供

```text
Confidence
Source
Reason
```

### 实现要点

1. Aho-Corasick 扫描 Lexicon 同音变体
2. 命中后查 Pinyin Index 比对原词的拼音
3. 拼音匹配则 `state.Replace`，否则 `state.Suggest`
4. Confidence 默认根据 Levenshtein 距离计算

### 配置示例

```yaml
processors:
  - name: pinyin
    config:
      auto_threshold: 0.85
      suggest_threshold: 0.70
      pinyin_dict_ref: "default"
```


### v1.1 行为同步（Phonetic 整词路径，D-2 修复）

- **整词路径（新增）**：构建期把所有 `Variant{Homophone}.Text` 加入 Aho-Corasick
  模式集（跳过 variant 为 canonical 子串的登记与跨条目重复），整词命中后按
  置信度阈值 Apply / Suggest / Skip。`Variant{Homophone}.Text` 由此正式被消费
  （缺陷清单 D-2 关闭）。
- **逐字路径收紧**：仅索引**单字 canonical entry**。多字 entry 不可能通过单字
  span 命中——杜绝"1 字 span 换整词"的文本损坏（曾实测复现三重复制）。
- **分类与算法解耦**：Category 为 `phonetic`；拼音算法经由 `PinyinConverter`
  插件注入，中文拼音只是一种实现。分类名/处理器名均不绑定中文。

---

## 7. Fuzzy Processor（§41）

**包路径**：`processor/fuzzy`

### 推荐执行流程

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

### 关键禁止

> **禁止对整个 Lexicon 进行无条件暴力匹配。**

### 实现要点

1. **n-gram 候选剪枝**：提取输入文本的 2-gram / 3-gram，查倒排表
2. **长度过滤**：候选长度差超过阈值直接淘汰
3. **编辑距离**：仅对剪枝后的候选计算 Levenshtein
4. **决策**：根据 `auto_threshold` / `suggest_threshold` 决定 Apply / Suggest / Skip

### 配置示例

```yaml
processors:
  - name: fuzzy
    config:
      auto_threshold: 0.80
      suggest_threshold: 0.65
      ngram_size: 2
      max_edit_distance: 2
      max_length_diff: 1
      categories:
        PERSON: { auto: 0.70, suggest: 0.55 }
        ORG:    { auto: 0.75, suggest: 0.60 }
        DEFAULT: { auto: 0.80, suggest: 0.65 }
```

> 类别阈值支持 per-class 覆盖。

---

## 8. Context Processor（§42）

**包路径**：`processor/ctxproc`（v1.1 实现落地，不再是纯占位）

### v1.1 行为同步

- 消费 `Variant{Contextual}`（v1.1 新增 kind，值追加兼容）：整词 AC 扫描；
- 候选唯一 → **Suggest**；多候选 → 注入 `Scorer` 重排序，唯一最优 → Suggest，
  仍歧义 → Skip；
- **v1 只 Suggest，永不 Apply**（上下文消歧属启发式，按规范"无法确定时
  Suggest 或 Skip"降级）；
- 无 `Scorer` 且多候选时保持词库 ID 序的确定性。
- `ctxproc.New()` 保留为 no-op（向后兼容）；功能性构造为
  `ctxproc.NewWithLexicon(lex)`，Standard/HighAccuracy 预设已启用。

### 职责

根据上下文规则进行规范。

### 输入

```text
Context
Candidate
Rule
```

### 输出

```text
Apply
Suggest
Skip
```

属于**推断型** Processor。

### 实现要点

1. 上下文窗口：左右各 N 字符（或 N token）
2. 规则匹配：正则 / DSL / 简单字符串模式
3. 决策：与 Fuzzy 类似的三档决策

### 配置示例

```yaml
processors:
  - name: context
    config:
      window_left: 4
      window_right: 4
      rules:
        - pattern: "我要(\\w+)了"
          suggest_for: "$1"
        - pattern: "(.{2,})有限公司"
          require_class: ORG
```

---

## 9. LLM Processor（**§34 + D1 决议：可选扩展**）

**包路径**：`processor/llm`

### 职责

LLM 是普通 Processor，**不在 Standard Preset 内**（D1 决议）。

### 核心库**不绑定**具体模型 SDK。

```go
type LLMProcessor struct {
    client Client
}
```

### 使用

```go
pipeline := lexnorm.NewPipeline(
    clean.New(),
    alias.New(lexicon),
    fuzzy.New(config),
    // LLM 不在默认 Preset；业务侧显式追加
    llm.New(client),
)
```

### 核心库不负责

- API Key
- 模型选择
- Token 成本
- Prompt 管理
- 模型供应商
- 模型服务生命周期

### Client 接口（由调用方实现）

```go
type Client interface {
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
}

type CompletionRequest struct {
    Prompt      string
    MaxTokens   int
    Temperature float64
    // 其他模型无关字段
}
```

> 业务侧可以适配 OpenAI / Anthropic / 自研模型，核心库保持零依赖。

### 配置示例

```yaml
processors:
  - name: llm
    config:
      client_ref: "openai-prod"
      prompt_template: "请规范化以下文本，保持原意：\n\n{{.Text}}"
      max_tokens: 2000
      temperature: 0.1
      processor_version: "gpt-4o-2024-08"  # 用于 Result 溯源
```

### 非确定性标识

LLM 必须通过 `Version()` 显式标识模型版本，便于审计。**Result.Changes 中的 ProcessorVersion 字段不可为空**（LLM 强制）。

---

## 10. 内置 Processor 一致性约束

| 维度 | 约束 |
|---|---|
| 包结构 | 每个 processor 子包独立 `processor/<name>/` |
| New 签名 | `New(config ...) (*Type, error)` |
| Process 签名 | `Process(ctx, *State) error` |
| Name 返回 | 与 Descriptor.Name 完全一致 |
| 确定性 | 在 §1 表中标定 |
| 业务词 | **零业务词**（如 PERSON/ORG 只作为 Class 字符串） |
| 测试 | 必须覆盖 Apply / Suggest / Skip 三档 |

---

## 11. 内置 Processor 自检清单

- [ ] 是否每个 processor 都注册到 `DefaultRegistry`？
- [ ] 是否每个 processor 都提供 `New()` 构造函数？
- [ ] 是否每个 processor 都能脱离 Engine 单独运行？
- [ ] 是否每个 processor 都实现了 Descriptor？
- [ ] 是否每个 processor 都经过 Benchmark 验证？
- [ ] 是否每个 processor 都经过 Fuzz（如果接受字符串输入）？
- [ ] 是否每个 processor 都有 doc.go？

---

## 12. 相关文档

- 上游：[03-Processor接口与生命周期](03-Processor接口与生命周期.md)
- 场景组合：[13-应用场景与Pipeline模板](13-应用场景与Pipeline模板.md)
- 算法细节：[14-性能设计与算法优化](14-性能设计与算法优化.md)
- 测试：[15-测试策略与质量工程](15-测试策略与质量工程.md)
