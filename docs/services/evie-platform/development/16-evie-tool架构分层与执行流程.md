# evie/tool 架构分层与执行流程（重新梳理版）

> 重构后的认知：语音识别 + 词库加载属于**业务层**，`pkg/lexnorm` 是**引擎层**（纯文本规范化），二者解耦。

---

## 1. 三层架构

```
┌────────────────────────────────────────────────────────────────────┐
│                   evie/tool 服务（业务层 biz+service）                │
│                                                                    │
│   ┌──────────────┐         ┌──────────────────┐                   │
│   │  asr/        │         │  vocab/          │                   │
│   │  funasr      │         │  qua HTTP sync   │                   │
│   │  科大讯飞     │         │  system.json     │                   │
│   │  whisper     │         │  tenant cache    │                   │
│   └──────┬───────┘         └────────┬─────────┘                   │
│          │ raw_text                  │ snapshot (entries+relations)│
│          ▼                           ▼                             │
│   ┌──────────────────────────────────────────────────────────┐    │
│   │  biz.EnhancementUsecase                                  │    │
│   │   - adapter: lexnorm.Change → v1.EnhanceChange         │    │
│   │   - ProviderResolver: per-tenant Lexicon                 │    │
│   └────────────────────────┬─────────────────────────────────┘    │
│                            │                                      │
└────────────────────────────┼──────────────────────────────────────┘
                             │ raw_text + Lexicon
                             ▼
┌────────────────────────────────────────────────────────────────────┐
│              pkg/lexnorm（引擎层，与业务解耦）                       │
│                                                                    │
│   Engine.Normalize(ctx, text, opts) → Result                       │
│                                                                    │
│   Pipeline:                                                        │
│   ① normalize  (全角→半角、空白归一)                                 │
│   ② disfluency (去填充词 啊/呃/嗯)                                   │
│   ③ alias      (别名匹配：金种子→金种籽)                              │
│   ④ deterministic (确定性纠错)                                      │
│   ⑤ pinyin     (拼音同音：黑种是→黑种子)                              │
│   ⑥ fuzzy_vocab (词库驱动 Levenshtein：周丽群→佘丽群)              │
│   ⑦ ctxproc    (上下文纠错：未来)                                    │
│                                                                    │
│   命名约束（核心包禁止）：                                            │
│   ❌ tenant/ASR/OCR/HR/Employee/Customer/Meeting/Agent/Document     │
│   ✅ 业务上下文 → Profile                                           │
│   ✅ 知识来源 → LexiconSource                                      │
└────────────────────────────┬───────────────────────────────────────┘
                             │ normalized_text + Changes
                             ▼
┌────────────────────────────────────────────────────────────────────┐
│                     输出层（消费方）                                  │
│   HTTP 200 → JSON {rawText, enhancedText, changes[]}              │
│   写文件 / 入库 / 下游 NLP / 客户交付                                │
└────────────────────────────────────────────────────────────────────┘
```

---

## 2. 单次请求的执行流程（happy path）

```
┌──────────┐
│ Client   │ POST /evie/tool/v1/asr:recognize
└────┬─────┘ + Bearer token + audioData(base64)
     │
     ▼
┌──────────────────────────────────────┐
│ HTTP Middleware (token_auth)         │
│   1. 解析 Authorization: Bearer ...  │
│   2. 从 Redis 取 oauth2_access_token │
│   3. 反序列化 → AuthInfo{tenantId,   │
│      accessToken, userId, deptId}    │
│   4. 注入 ctx（key=biz.AuthContext）│
└────┬─────────────────────────────────┘
     │ ctx{AuthInfo}
     ▼
┌──────────────────────────────────────┐
│ EnhancementService.AsrRecognize      │
└────┬─────────────────────────────────┘
     │
     ▼
┌──────────────────────────────────────┐
│ ASRUsecase.Recognize                 │
│   ├─ 1. asrProvider.Recognize(audio) │
│   │     → {raw_text}                │
│   │     funasr: mp3→wav→POST 18000  │
│   │     科大讯飞: IAT 流式           │
│   │                                   │
│   ├─ 2. ensureVocabReady(tenantId)   │
│   │     首次：VocabSyncer.lazySync   │
│   │     ├─ qua HTTP users GET        │
│   │     ├─ qua HTTP depts GET        │
│   │     ├─ Normalizer.NormalizeBatch │
│   │     ├─ VocabularyBuilder.        │
│   │     │   UpdateTenant(...)        │
│   │     └─ 缓存入内存                │
│   │                                   │
│   ├─ 3. lexnorm.Engine.Normalize     │
│   │     ├─ ProfileResolver.Resolve   │
│   │     │   → 触发 lazy Build        │
│   │     ├─ buildLexiconFromSnapshot  │
│   │     ├─ Pipeline.Process          │
│   │     │   normalize→disfluency→   │
│   │     │   alias→deterministic→     │
│   │     │   pinyin→fuzzy_vocab→      │
│   │     │   ctxproc                  │
│   │     └─ Result{Original, Text,    │
│   │          Steps, Changes}         │
│   │                                   │
│   └─ 4. convertChanges 适配层        │
│         lexnorm.Change →             │
│         v1.EnhanceChange             │
│         (type=ALIAS/FUZZY/...)       │
└────┬─────────────────────────────────┘
     │
     ▼
┌──────────────────────────────────────┐
│ HTTP 200                             │
│   {                                  │
│     rawText: "...金种子情况...",     │
│     enhancedText: "...金种籽...",    │
│     provider: "funasr",             │
│     changes: [                       │
│       {                              │
│         from: "金种子",              │
│         to: "金种籽",                │
│         action: "replace",           │
│         type: "ALIAS",               │
│         source: "alias",             │
│         confidence: 1.0              │
│       },                             │
│       ...                            │
│     ]                                │
│   }                                  │
└──────────────────────────────────────┘
```

---

## 3. 关键解耦点（为什么 lexnorm 是独立引擎）

| 关注点 | 业务层（evie/tool） | 引擎层（pkg/lexnorm） |
|---|---|---|
| **数据来源** | qua HTTP / system.json | 不感知，由 caller 注入 Lexicon |
| **租户/多租户** | TenantRegistry + lazy sync | 不感知，Profile 是抽象 ID |
| **错误码体系** | Kratos errors (TOKEN_MISSING 等) | 不感知，err.Error() string only |
| **变更事件格式** | v1.EnhanceChange (proto) | lexnorm.Change（中立字段） |
| **可单独使用** | ❌ 需启动整个服务 | ✅ `lexnorm.New(opts...).Normalize(...)` |
| **测试方式** | 集成测试（miniredis + httptest） | 单元测试（纯函数式） |

---

## 4. 之前的认知偏差 vs 新认知

| 维度 | 旧理解（错） | 新理解（对） |
|---|---|---|
| **服务定位** | "ASR + 文本增强工具" | "把原始文本规范化的服务" |
| **核心依赖** | pkg/textenhance 是核心 | pkg/lexnorm 是核心，ASR/qua 都是数据源 |
| **产品名建议** | evie-tool 文本增强服务 | evie-tool 文本规范化服务 / text-normalization |
| **vocab 角色** | "喂给增强管道的词库" | "喂给规范化引擎的字典" |
| **fuzzy 算法位置** | 业务 Processor | 引擎 Processor + 业务可扩展 |

---

## 5. 待你核对的点

请确认：

1. **三层切分** 是否对？业务层 / 引擎层 / 输出层
2. **pkg/lexnorm 的命名约束**（禁止 tenant/ASR 等业务词）是否还需保留？
3. **服务定位** 是否要从"文本增强"改为"文本规范化"？
4. **flow 步骤** 是否漏了什么？比如：
   - 是否需要"流式识别"分支？
   - 是否需要"context_correction"步骤？
5. **输出"词法规范文件"** 的语义，根据新分层应该是：
   - 引擎层处理后的 `normalized_text` + `changes[]`（业务层负责序列化）
   - 还是引擎层的 grammar/rule 定义（lexnorm.Config）？

确认后我们再逐项排查：服务代码、协议、词库加载、引擎调用、输出格式。