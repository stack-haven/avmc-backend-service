# evie/tool 服务能力复盘（重构前快照）

> 本文档是 evie/tool 全面重构前的最后一份能力梳理，2026-09-05。
> 后续重构必须以本文档为基线，逐项核对需求与实现。

---

## 1. 服务定位（重构后的认知）

**evie/tool** 是一个 **文本规范化（normalization）服务**，把任意原始文本（典型场景：ASR 输出）转换为相对规范的最终文本。

它由两层组成：

| 层 | 内容 | 关注点 |
|---|---|---|
| **业务层** evie/tool | ASR（funasr/讯飞） + 词库加载（qua/system.json） + 适配 | 数据来源、租户隔离、错误码 |
| **引擎层** pkg/lexnorm | 7 步文本规范化 pipeline | 通用文本处理、不感知业务 |

**关键概念分离**：
- 业务层不知道 lexnorm 的实现细节
- 引擎层不感知 tenant / ASR / qua 等业务概念（命名硬约束）

---

## 2. 输入（Inputs）

| 输入 | 来源 | 形态 |
|---|---|---|
| **音频** | HTTP POST body | MP3/WAV/Opus（base64 或原始字节） |
| **Bearer Token** | Authorization header | `oauth2_access_token:<token>` Redis 键 |
| **租户上下文** | 从 Token 反序列化 | tenantId, userId, deptId |
| **系统静态词库** | `configs/dictionaries/system.json` | 3+ entries + phrase rules |
| **租户动态词库** | qua HTTP API（users + depts） | 73+ entries + 65+ relations（实测） |
| **配置** | `configs/config.yaml` | enhancement / asr / tenant_vocab / system_dict |

**Bearer Token 鉴权流程**：
```
HTTP Middleware
  ├─ 解析 Authorization: Bearer xxx
  ├─ Redis GET oauth2_access_token:xxx
  ├─ 反序列化 AuthInfo{tenantId, accessToken, userId, deptId}
  └─ 注入 ctx（key = biz.AuthContext）
```

---

## 3. 核心处理流程（Pipeline）

```
raw_text (ASR 输出)
    │
    ▼
┌────────────────────────────────────┐
│ pkg/lexnorm.Engine.Normalize       │
│   ① normalize   (全角→半角)        │
│   ② disfluency  (去填充词)         │
│   ③ alias       (别名→标准词)      │
│   ④ deterministic (确定性纠错)     │
│   ⑤ pinyin      (拼音同音)         │
│   ⑥ fuzzy_vocab (词库驱动 fuzzy)   │
│   ⑦ ctxproc     (上下文纠错)       │
└────────────────────────────────────┘
    │
    ▼
normalized_text + Changes[]
```

**关键不变量**：
- `lexnorm.Engine` 是单例（每进程一个）
- `TenantProfileResolver` 按 tenantID 懒加载 Lexicon（首次请求触发 qua sync）
- Pipeline 对相同 raw_text **完全确定性**（funasr 抖动不影响 lexnorm 行为）

---

## 4. 输出（Outputs）

### 4.1 HTTP 响应字段（当前 proto）

```protobuf
message AsrRecognizeResponse {
  string request_id        = 1;
  string session_id        = 2;
  string raw_text          = 3;   // ASR 原始
  string enhanced_text     = 4;   // 规范化后  ← 建议改名 normalized_text
  float  confidence        = 5;
  string duration_ms       = 6;
  bool   is_final          = 7;
  string provider_name     = 8;
  string audio_path        = 9;
  repeated EnhanceChange changes = 10;  // ← 建议改名 NormalizeChange
  Status status            = 11;
  string processing_time_ms = 12;
  ... 各 processor 耗时字段（已废弃，应删除）
  string error_message     = 14;
}
```

### 4.2 真实运行结果（晨会录音，5 次实测）

| 维度 | 数值 |
|---|---|
| 总 changes | 74–75 / 次 |
| **ALIAS 命中** | "金种子 → 金种籽" 13 次/次（系统词库 ALIAS 关系） |
| **FUZZY 命中** | "测试播 → 测试1" 5/5 次（qua 用户 PERSON conf=0.67） |
| **FUZZY 命中** | "周丽群 → 佘丽群" 5/5 次（qua 用户 PERSON conf=0.67） |
| **FILLER 命中** | "啊" 5 次 + "呃" 3 次（disfluency） |
| **CLEAN 命中** | 全角逗号 37 次（normalize） |

---

## 5. 已知问题（重构必须修复的）

| ID | 严重度 | 问题 | 触发条件 |
|---|---|---|---|
| **P1** | 🔴 | **冷启动首请求 cache miss**：lazy sync 异步，首次 ProfileResolver.Resolve 拿到 system snapshot（仅 3 entries），导致 fuzzy/alias 全失效 | 第一次请求；或 sync 间隔 5min 后的首个请求 |
| **P2** | 🟡 | **Config 默认阈值 0.95 与业务预期 0.65 不对齐**：fuzzy_vocab 想让 PERSON 类别 0.65 自动替换，但 lexnorm 全局是 0.95，结果 0.67 全变 suggest | 每一次 fuzzy_vocab 跑 |
| **P3** | 🟡 | **服务定位 vs 命名错位**：proto/rpc/服务/文档都叫"Enhance*"，但产品认知是"Normalize*" | 所有 API 路径 |
| **P4** | 🟡 | **pkg/lexnorm 命名约束被破坏**：`TenantProfileResolver` 类名 + fuzzy_vocab 硬编码 "PERSON" | 引擎层与业务层边界 |

### P1 详细：cache miss 时序

```
T0   request 1 进入
T0   Build → tenantSnaps[tenant]==nil  → 异步触发 lazySyncOnMiss
T0   Build 返回 system fallback（3 entries，PERSON=0）
T1   lex 只 4 entries；fuzzy_vocab 全空命中
T5   qua sync 异步完成 → UpdateTenant 写入 75 entries
T6   request 2 → Build 命中 → 75 entries → fuzzy_vocab 正常
```

**修复方向**：
- A. Build cache miss 时阻塞等待 sync（5s timeout）
- B. 增加 `vocab_prefetch` gRPC method 客户端主动预热
- C. 接受"首请求 degraded"，文档明示并加重试

### P2 详细：双层决策冲突

```
lexnorm.Config.DefaultConfig():
  AutoApplyThreshold = 0.95
  SuggestThreshold   = 0.65

fuzzy_vocab.DefaultFuzzyVocabConfig():
  AutoThreshold: 0.80  (全局 fallback)
  CategoryAuto: { PERSON: 0.65 }

实际跑：
  autoApply := s.Config().AutoApplyThreshold  // 0.95
  autoTh    := thresholdFor(CategoryAuto, 0.95, cat)
  if cat=="PERSON": autoTh=0.65 ✓
  if cat=="" or 其他: autoTh=0.95 ✗（PRODUCT 应是 0.80）
```

**修复方向**：
- A. biz 层 `lexnorm.New(..., WithConfig(lexnorm.Config{AutoApplyThreshold: 0.6, ...}))`，把全局阈值调低，让 processor 自己做最终决策
- B. fuzzy_vocab 内部用 `s.Suggest(...)` 替换 `s.Replace(...)` —— 牺牲自动应用，依赖下游 UI

### P3 详细：建议的全面改名清单

| 现在 | 建议 |
|---|---|
| `EnhancementService` | `NormalizationService` |
| `EnhancementUsecase` | `NormalizationUsecase` |
| `EnhanceText` rpc | `NormalizeText` |
| `EnhanceChange` | `NormalizeChange` |
| `EnhancedText` 字段 | `NormalizedText` 字段 |
| `enhancement.proto` | `normalization.proto` |
| `enhancement_service.go` | `normalization_service.go` |
| `enhancement_test.go` | `normalization_test.go` |
| 文档 11/12 "文本增强" | "文本规范化" |
| `enhancement` 配置 section | `normalization` 配置 section |

### P4 详细：业务概念渗透

```
// 现在
biz/lexnorm_engine.go:
  type TenantProfileResolver struct {...}  // ← "Tenant" 业务词
  func (r *TenantProfileResolver) Resolve(...)  // ← "Tenant"

biz/processor/fuzzy_vocab.go:
  CategoryAuto: {"PERSON": 0.65}  // ← "PERSON" 业务词

// 建议
biz/lexnorm_engine.go:
  type profileResolver struct {...}  // 中性
  func (r *profileResolver) Resolve(...)  

biz/processor/fuzzy_vocab.go:
  CategoryAuto 由配置驱动；biz 层负责把 "PERSON"→"NAME" 等通用词
  或：把 PERSON 当作业务概念下沉到 biz 层配置，不进 processor
```

---

## 6. 缺失能力（当前实现没有的）

| 能力 | 描述 | 优先级 |
|---|---|---|
| **首请求阻塞 sync** | P1 的修复 A 方案 | P0 |
| **vocab_prefetch 接口** | P1 的修复 B 方案 | P1 |
| **流式识别结果增强** | 当前流式只返回 rawText，没有逐帧规范化 | P2 |
| **部分失败重试** | 当前 qua 一失败就 fallback system | P1 |
| **错误码细化** | lexnorm 处理错误统一成 INTERNAL，没区分原因 | P2 |
| **配置 lexnorm.Config** | 当前用 DefaultConfig，没暴露阈值到 config.yaml | P0 |
| **processor 关闭开关** | 业务想临时关闭 fuzzy_vocab，目前必须改代码 | P1 |
| **observability hooks** | lexnorm.Hook 没接到 tracing/metrics | P2 |

---

## 7. 验收 Checklist（重构后必须全部 ✓）

- [ ] ASR 识别成功（funasr / 讯飞 任选）
- [ ] Bearer Token 鉴权通过（401 → TOKEN_MISSING）
- [ ] qua sync 至少 1 次成功（73+ entries）
- [ ] system.json 加载成功（3+ entries）
- [ ] Lexicon 至少 70 entries
- [ ] **fuzzy_vocab 替换命中 ≥ 1**（PERSON conf >= 0.65 → replace）
- [ ] **alias 替换命中 ≥ 1**（系统 ALIAS 关系）
- [ ] **filler 命中 ≥ 1**（disfluency）
- [ ] **clean 命中 ≥ 1**（normalize 全角转半角）
- [ ] 5 次连续请求 fuzzy/alias 命中稳定
- [ ] 错误码按 proto 文档映射
- [ ] 文档与代码命名一致（"规范化"或"增强"二选一，不再混用）

---

## 8. 重构目标

| 目标 | 描述 |
|---|---|
| **G1 命名统一** | 全面改为"Normalize*"语义（per P3 清单） |
| **G2 业务层解耦** | 移除业务词从 processor 与 engine 层（per P4） |
| **G3 阈值可配** | lexnorm.Config 通过 config.yaml 注入（per P2） |
| **G4 首请求稳定** | 修复 cache miss 导致的 fuzzy 失效（per P1） |
| **G5 文档同步** | 11/12/14 文档与新代码一致 |
| **G6 测试覆盖** | 关键命中（alias/fuzzy/clean/filler）必须有单元测试 |
| **G7 单测回归** | race detector 全绿 |

---

## 9. 不在重构范围

- evie/service 的文本增强（继续用 pkg/textenhance 或后续单独迁移）
- pkg/lexnorm 自身的开源化（OPEN_SOURCE_PLAN.md）
- qua 接口变更
- proto breaking change（如需，须走 buf breaking 校验）