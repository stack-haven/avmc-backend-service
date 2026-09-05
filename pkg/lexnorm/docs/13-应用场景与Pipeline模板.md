# 13 · 应用场景与 Pipeline 模板

> 源节：§41 业务场景验收测试
> 适用阶段：Phase 6
> 受众：业务开发者

---

## 1. 设计前提

`ark-lexnorm` **领域中立**。下述场景展示核心库的典型用法，**不是核心库的定义**。

```text
ASR / OCR / Search / CRM / Document / Other
   ↓
ark-lexnorm
   ↓
规范化文本 + 变更记录 + 完整运行快照
```

> **1.2 关键变化**：Result 现在包含 `RuntimeInfo`，业务方可以获得 ProfileID / ProfileVersion / LexiconVersion / PipelineVersion / ProcessorVersions 完整快照，便于审计与回放。

---

## 2. 1.2 业务场景验收测试（**§41**）

6 个场景必须全部通过，作为 Phase 12 (M13) 的硬验收条件。

### 场景 A：ASR

```yaml
profile: asr-default
input: "小田今天帮我查一下个种子的情况"
lexicon:
  entries:
    - canonical: 田华
      variants: [{ text: 小田 }]
    - canonical: 颗种籽
      variants: [{ text: 个种子 }]
pipeline:
  - clean
  - disfluency
  - alias
  - deterministic
  - pinyin
  - fuzzy
```

**期望**：

```text
Text:
田华今天帮我查一下颗种籽的情况

Changes:
[0,2) 小田 → 田华
[8,11) 个种子 → 颗种籽

Status: Success

Runtime:
  ProfileID: asr-default
  ProfileVersion: v20260904-001
  LexiconVersion: v20260901-003
  PipelineVersion: pipeline-v20260904-002
  ProcessorVersions: { clean: v1, alias: v3, ... }
```

**断言**：
- Text 正确
- Changes 数量 == 2
- Status == Success
- RuntimeInfo 字段全部非空

### 场景 B：会议

```yaml
profile: meeting-document
input: "张总说让老王和小田明天处理市场部的事情"
lexicon:
  entries:
    - canonical: 张强
      variants: [{ text: 张总 }]
    - canonical: 王强
      variants: [{ text: 老王 }]
    - canonical: 田华
      variants: [{ text: 小田 }]
pipeline:
  - clean
  - alias
  - deterministic
```

**期望**：

```text
Text:
张强说让王强和田华明天处理市场部的事情

Changes:
[0,2) 张总 → 张强
[5,7) 老王 → 王强
[8,10) 小田 → 田华
```

**断言**：
- Text 正确
- Changes 数量 == 3
- Status == Success

### 场景 C：保护区（Protected Span）

```yaml
input: "小田今天参加会议"
lexicon:
  entries:
    - canonical: 田华
      variants: [{ text: 小田 }]
pipeline:
  - alias     # 小田 → 田华，Lock [0, 6)
  - fuzzy     # 不得修改 [0, 6)
```

**期望**：

```text
Text: 田华今天参加会议

Changes:
[0,2) 小田 → 田华 (Applied=true)
```

**关键断言**：
- Alias 完成后区间 `[0, 6)` 被 Lock
- Fuzzy 即使有候选匹配 `田华` 也**不得**修改该区间
- Fuzzy 不得在 Result.Changes 中产生 Applied=true 的 Change

### 场景 D：Lexicon 热更新（请求一致性）

```yaml
request_A:
  started_at: T0
  lexicon_version: V1
  input: "小田明天来"
  expected_text: "田华明天来"

publish_V2_at: T1 (T0 < T1 < request_A 结束)

request_B:
  started_at: T2 (T2 > T1)
  lexicon_version: V2  # 包含新词
  input: "新词测试"
  expected_text: "新词规范"
```

**期望**：

```text
request_A:
  Text: "田华明天来"           ← 使用 V1
  Runtime.LexiconVersion: V1

request_B:
  Text: "新词规范"             ← 使用 V2
  Runtime.LexiconVersion: V2
```

**关键断言**：
- 请求 A 即使在执行期间遇到 V2 发布，仍使用 V1
- 请求 B 使用 V2
- 两个请求的 Result.Runtime.LexiconVersion 正确反映各自使用的版本

### 场景 E：Processor 故障降级

```yaml
input: "正常文本，context 失败"
processors:
  - clean
  - alias
  - context  # 模拟失败
error_policy: continue_on_error
```

**期望**：

```text
Text: "正常文本，context 失败"   ← 原文保留（context 失败前已被前面的 Processor 处理过）
Status: Partial
Errors: [context failure]
Duration: 包含 context 步耗时
Steps: [clean OK, alias OK, context FAILED]
```

**关键断言**：
- Status == Partial
- Text 不是空（**不丢失原始文本**）
- Errors 包含 context 失败
- Steps 完整记录

### 场景 F：多 Profile 并发

```yaml
request_A:
  profile_id: profile-a
  input: "A 场景文本"

request_B:
  profile_id: profile-b
  input: "B 场景文本"

concurrent: true
```

**期望**：

```text
request_A:
  Result.Runtime.ProfileID: profile-a
  Result.Runtime.LexiconVersion: lex-a-version
  Result.Runtime.PipelineVersion: pipe-a-version

request_B:
  Result.Runtime.ProfileID: profile-b
  Result.Runtime.LexiconVersion: lex-b-version
  Result.Runtime.PipelineVersion: pipe-b-version
```

**关键断言**：
- 两个请求并发执行，零 race
- 两个请求的 Runtime 互不污染
- 每个请求看到的 Lexicon / Pipeline / Config 是各自 Profile 绑定的

---

## 3. 典型场景与 Preset

### 3.1 ASR 场景

```text
ASR Result
    ↓
ark-lexnorm
    ↓
Clean → Disfluency → Alias → Deterministic → Pinyin → Fuzzy → Context
    ↓
Normalized Text
```

```go
pipeline := lexnorm.NewPipeline(preset.ASR()...)
```

### 3.2 OCR 场景

```text
Normalize → Deterministic → Pinyin → Fuzzy
+ 自研 OCRShapeProcessor（形近字）
```

```go
pipeline := lexnorm.NewPipeline(preset.OCR()...)
pipeline = append(pipeline, myOCRShape.New(cfg))  // 业务侧扩展
```

### 3.3 Search 场景

```text
Normalize → Alias → Deterministic → Pinyin → Fuzzy
```

保守 Apply，多 Suggest。

### 3.4 文档审计场景

```text
全部 Processor 启用
+ Decision Policy: Suggest Only
```

不自动改写，全部 Suggest 供人审核。

### 3.5 客服工单

```text
Clean → Alias → Deterministic
```

保守 Apply + Suggest。

---

## 4. 场景速查表

| 场景 | 必启用 | 推荐启用 | Decision Policy |
|---|---|---|---|
| **ASR** | Disfluency / Pinyin / Fuzzy | Clean / Alias / Deterministic / Context | Apply + Suggest |
| **OCR** | Fuzzy | Clean / Alias / Deterministic / Pinyin | Apply + Suggest |
| **Search** | Alias / Fuzzy | Deterministic / Pinyin | 保守 Apply + Suggest |
| **Doc 审计** | 全启用 | — | **仅 Suggest** |
| **客服工单** | Alias / Deterministic | Clean / Fuzzy | Apply + Suggest |
| **日志规范化** | Clean / Alias | Deterministic | Apply 为主 |

---

## 5. ASR 场景实现详解

### 典型配置

```yaml
profile: asr
lexicon: org-people-v3
pipeline:
  - clean
  - disfluency { tokens: ["呃","额","嗯","啊","那个","然后"] }
  - alias { auto_threshold: 0.95 }
  - deterministic
  - pinyin { auto_threshold: 0.85 }
  - fuzzy { auto_threshold: 0.80, suggest_threshold: 0.65 }
  - context
```

### Result 用法

```go
res, _ := eng.Normalize(ctx, "呃，佘丽群明天开会", lexnorm.WithProfileName("asr"))
fmt.Println(res.Text)             // "周丽群明天开会"
fmt.Println(res.Changes)          // 全部 Change
fmt.Println(res.Suggestions)      // 仅 Applied=false
fmt.Println(res.Status)           // StatusSuccess / StatusPartial / ...
fmt.Println(res.Runtime.LexiconVersion)  // 用于审计
```

### 反例（核心库不得出现）

```go
// ❌ 核心库代码
func IsASRText(text string) bool { ... }   // 业务耦合
type ASRResult struct { ... }               // 业务类型
```

---

## 6. 会议文档场景实现详解

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

### Adapter 实现

```go
type MeetingParticipantSource struct {
    api ParticipantAPI
}

func (s *MeetingParticipantSource) Version() string {
    return s.api.LexiconVersion()
}

func (s *MeetingParticipantSource) Entries(yield func(lexicon.Entry) bool) {
    for _, p := range s.api.Participants() {
        if !yield(lexicon.Entry{
            ID:   lexicon.EntryID(p.ID),
            Text: p.CanonicalName,
            Variants: []lexicon.Variant{
                {Text: p.Nickname, Kind: lexicon.VariantAlias, Confidence: 1.0, Source: "hr"},
                {Text: p.SpeechName, Kind: lexicon.VariantHomophone, Confidence: 0.85, Source: "asr-stat"},
            },
        }) {
            return
        }
    }
}
```

### 业务层利用 Result

```text
Result.Text       → 写入会议文档
Result.Changes    → 文档追溯
Result.Runtime    → 人名纠错审核
                  → 规范化质量分析
                  → 问题复现
```

---

## 7. 自研 Processor 接入示例

### 示例：搜索场景的「同义词扩展」

```go
package synonym

import (
    "context"
    "github.com/stack-haven/lexnorm"
)

type SynonymExpander struct{}

func (SynonymExpander) Name() string { return "synonym-expand" }
func (SynonymExpander) Version() string { return "v1" }
func (SynonymExpander) Certainty() lexnorm.Certainty {
    return lexnorm.CertaintyMedium
}
func (SynonymExpander) Process(ctx context.Context, s *lexnorm.State) error {
    text := s.Text()
    for _, syn := range lookupSynonyms(text) {
        s.Suggest(lexnorm.Span{0, len(text)}, syn, lexnorm.ChangeMeta{
            Kind:       lexnorm.ChangeKindSynonym,
            Source:     "synonym-db",
            Confidence: 0.80,
            Reason:     "synonym expansion",
        })
    }
    return nil
}

var Descriptor = lexnorm.Descriptor{
    Name:      "synonym-expand",
    Certainty: lexnorm.CertaintyMedium,
    New: func(cfg json.RawMessage) (lexnorm.Processor, error) {
        return SynonymExpander{}, nil
    },
}
```

---

## 8. 自检清单

- [ ] 业务 Processor 是否违反 §2.6 零业务依赖？（如取名 `ASRProcessor`）
- [ ] 自研 Processor 是否注册了 Descriptor？
- [ ] 场景配置是否与 Processor 能力匹配？
- [ ] Decision Policy 是否随场景变化？
- [ ] Suggest 输出是否被调用方正确读取？
- [ ] 6 个验收场景（A~F）是否全部通过？
- [ ] Result.Runtime 是否被业务方用于审计？

---

## 9. 相关文档

- Processor 规范：[12-内置Processor规范](12-内置Processor规范.md)
- Registry：[09-Registry与动态装配](09-Registry与动态装配.md)
- 决策分级：[02 §16](02-核心领域模型.md#16-decision)
- 故障矩阵：[07-Engine与Profile](07-Engine与Profile.md) §14
- 测试矩阵：[15-测试策略与质量工程](15-测试策略与质量工程.md)
