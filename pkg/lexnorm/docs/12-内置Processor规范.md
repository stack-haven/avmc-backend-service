# 12 · 内置 Processor 规范

> 源节：§34 LLM · §35 内置 Processor · §36~§42 各 Processor 详情
> 适用阶段：Phase 5
> 受众：核心开发者 + 学习者（用 ark-lexnorm 解决业务问题的人）

---

## 1. 内置 Processor 清单（§35）

| # | Processor | 包路径 | 确定性 |
|:--:|---|---|:--:|
| 1 | Normalize | `processor/clean` | Deterministic |
| 2 | Disfluency | `processor/disfluency` | Deterministic |
| 3 | Alias | `processor/alias` | Deterministic |
| 4 | Deterministic | `processor/deterministic` | Deterministic |
| 5 | Pinyin | `processor/pinyin` | High |
| 6 | Fuzzy | `processor/fuzzy` | Medium |
| 7 | Context | `processor/context` | Medium |
| 8 | LLM | `processor/llm` | Unknown（可选） |

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

### 典型词

```text
呃
额
嗯
啊
那个
然后
```

### 关键约束

> 规则**必须可配置**。
>
> **不得把具体业务规则硬编码为核心知识。**

### 配置示例

```yaml
processors:
  - name: disfluency
    config:
      tokens: ["呃", "额", "嗯", "啊", "那个", "然后"]
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

**包路径**：`processor/context`

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
