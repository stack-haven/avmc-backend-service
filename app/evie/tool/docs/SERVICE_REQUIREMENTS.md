# evie/tool — 完整服务需求与功能明细

> **服务名称**：evie/tool — 独立轻量语音识别增强工具
> **仓库**：`backend-service`（GitHub: stack-haven/avmc-backend-service）
> **设计文档**：`docs/services/evie-platform/development/11-evie-tool独立轻量语音识别增强工具开发计划.md`
> **本文档路径**：`backend-service/app/evie/tool/docs/SERVICE_REQUIREMENTS.md`
> **当前状态**：✅ **M0–M9 全部交付 + 9.x 收口 + P1–P6 修复 + 工具公共化内化完成**

---

## 一、服务定位

evie/tool 是从 `app/evie/service` 抽取的**轻量化、零数据库、配置驱动**变体，专门服务于需要"语音识别 + 文本规范化"但不需要完整词库中心 UI/审计/版本管理的场景。

### 1.1 在 Ark Tech Platform 中的位置

```
Ark Tech Platform
├── app/evie/service        完整词库中心（DB + UI + 审计 + 版本管理）
└── app/evie/tool           轻量变体（无 DB、配置驱动、内置 8 层增强 + 多 ASR Provider）
```

### 1.2 服务边界（与 evie/service 对比）

| 维度 | evie/service | evie/tool |
|---|---|---|
| 数据存储 | Ent + MySQL/Postgres | 内存 + 文件 + Redis |
| 词库管理 | UI 增删改查 | 配置驱动 + qua 同步 |
| ASR | 单一服务 | 多 Provider（funasr/xunfei） |
| 文本增强 | 8 层 Pipeline | 8 层 Pipeline（同一份 lexnorm 引擎） |
| 鉴权 | gRPC 委托 | Bearer Token + Redis |
| 部署形态 | 微服务 | 独立工具 / Sidecar |

---

## 二、核心功能明细

### 2.1 多 Provider ASR（语音识别）

**功能描述**：按配置路由不同 ASR Provider，同一接口对前端屏蔽差异。

| Provider | 模式 | 适用 |
|---|---|---|
| **funasr** | 整段识别（POST） | 长录音、高精度 |
| **xunfei（讯飞）** | 流式识别（gRPC 双向流） | 实时场景 |
| **mock** | 内置假实现 | 集成测试 / CI |

**接口**：

- `POST /evie/tool/v1/asr:recognize` — 整段识别（funasr）
- `gRPC /evie/tool/v1/asr/stream` — 流式识别（xunfei，双向流）
- `POST /evie/tool/v1/enhance` — 纯文本增强（无 ASR）

**关键能力**：

- 音频本地落盘到 `upload/audio/<tenant>/<session>.<ext>`（git 忽略）
- 流式 ring buffer（64 帧）+ 半开区间语义
- 健康检查 endpoint：`/health/live` + `/health/ready`

### 2.2 8 层文本增强 Pipeline

**功能描述**：对 ASR 输出的原始文本逐层处理，最终输出规范化文本 + 完整变更记录。

**8 层顺序**（pkg/lexnorm 引擎）：

```
输入文本
  ↓
[1] cleaning                  清理（多余空白、标点统一）
  ↓
[2] filler                    填充词移除（"嗯"、"啊"、"那个"）
  ↓
[3] vocab_matching            词库精确匹配
  ↓
[4] alias_resolution          别名解析（"张总" → "张三"）
  ↓
[5] deterministic_replacement 确定性替换
  ↓
[6] phrase_standardization    短语标准化
  ↓
[7] pinyin_correction         拼音纠错（"周丽群" → "佘丽群"）
  ↓
[8] fuzzy_matching            模糊匹配（兜底）
  ↓
[9] context_correction        上下文纠错（未来扩展位）
  ↓
规范化文本 + 完整 Change 列表
```

**每层特性**：

- Functional Options 配置注入
- Observer 模式可观测（counting / logging）
- Snapshot / Status 检查点
- Policy 阈值控制
- Processor 命名空间（每次 Change 反填 `Change.Processor / ProcessorVersion`）

### 2.3 qua 系统词库自动同步

**功能描述**：定时从外部 qua 系统拉取用户/部门 → 拼音化 → 注入 per-tenant 词库。

**数据流**（Q13 决定的"外部系统解耦"）：

```
qua HTTP API
  ↓
pkg/source/qua (薄包装)
  ↓
pkg/source/http (通用 HTTP adapter)
  ↓
[]RawEntity (不透明 payload)
  ↓
biz.VocabularyNormalizer (YAML 规则)
  ↓
[]NormalizedEntry (canonical)
  ↓
VocabularyBuilder
  ↓
per-tenant in-memory lexicon
```

**关键设计**：

- RawEntity.Data 不透明（Source 包不依赖任何业务字段名）
- Normalizer YAML 规则可热改
- 同步策略：启动预加载 + 周期拉取（可配置）+ lazy sync

### 2.4 Bearer Token 认证

**功能描述**：从 `Authorization: Bearer <token>` 提取 token，查 Redis `oauth2_access_token:<token>` 获取 caller 身份。

**credential 包设计**（5 Provider 抽象）：

| Provider | 场景 |
|---|---|
| **redis** | 默认生产配置（共享业务系统 Redis） |
| **jwt** | 自签 JWT（独立部署，HS256/RS256） |
| **static** | 本地开发 / demo / 单元测试 |
| （可扩展） | 自定义后端（OIDC / LDAP / 数据库） |

**核心抽象**：

- `Provider.Authenticate(ctx, token) → *CallerIdentity`
- `CallerIdentity{TenantID, UserID, UserName, DeptID, UserType, Scopes, ExpiresAt, AccessToken}`
- `FieldMapper` JSON 路径映射配置化（支持 dot-path）
- 4 sentinel errors：`ErrTokenNotFound / ErrTokenInvalid / ErrProviderUnavailable / ErrInvalidConfig`

### 2.5 音频本地落盘

**功能描述**：所有 ASR 接收的音频本地保存到 `upload/audio/<tenant>/<session>.<ext>`。

**用途**：

- 审计追溯
- 重跑增强（同一音频不同策略）
- 数据回流（未来可入仓训练）

**约束**：

- `upload/**` 在 `.gitignore` 中
- 不上传到云存储（仅本地）
- 单租户单 session 文件命名

---

## 三、架构设计原则

### 3.1 三层架构

```
业务层 (evie/tool biz+service)  ←→  引擎层 (pkg/lexnorm)  ←→  输出层
```

### 3.2 三大禁令

1. **禁止全局可变状态**
2. **禁止 Processor 内部硬编码资源加载**
3. **禁止继承模拟**（用 Functional Options + 组合）

### 3.3 Q13 决定的"外部系统解耦"

所有外部系统数据走：**Generic Source → opaque RawEntity → Normalizer (YAML) → NormalizedEntry → Builder**

`pkg/source` 不 import 任何业务系统的结构体。

### 3.4 HA 三层保护（Pipeline）

```go
defer recover { ... }           // 1. panic 兜底
ctx timeout (5s default)        // 2. ctx 超时
error accumulation              // 3. 错误累积不中断
```

### 3.5 跨进程一致性（Map 遍历）

所有遍历 `entries / relations map` 必须用 `sortedKeys`（避免不同 Go 版本随机顺序导致的不一致）。

### 3.6 包结构偏好

业务无关功能抽象到服务内 `pkg/` 子包（与 `pkg/asr`, `pkg/lexnorm` 平级）：

- `pkg/credential` — 认证抽象
- `pkg/source` — 外部数据源抽象

> 已从原 `pkg/credential` 公共包改为服务内化，避免污染其他服务。

---

## 四、关键技术决策（Q1–Q13）

| # | 决策点 | 决定 |
|---|---|---|
| Q1 | 服务边界 | evie/tool = evie/service 的无 DB 配置驱动变体 |
| Q2 | 认证方式 | Bearer Token + Redis `oauth2_access_token:<token>` |
| Q3 | 词库存储 | 内存 + 启动预加载 + 周期同步 |
| Q4 | ASR Provider 路由 | 按 conf 路由（funasr 整段 / xunfei 流式） |
| Q5 | qua HTTP 端点 | `/admin-api/qua/member-extended/page?selectAll=true` + `/admin-api/system/dept/list` |
| Q6 | qua 响应壳 | Spring Cloud 风格 `{code, msg, data}` |
| Q7 | 讯飞凭证 | 默认值可直接启动联调 |
| Q8 | 系统静态词条 | `configs/dictionaries/system.json`（git 跟踪），热加载 |
| Q9 | 租户级持久化 | 不持久化，重启重建（启动预加载） |
| Q10 | 音频保留 | 本地 `upload/audio/`（git 忽略） |
| Q11 | 健康检查 | `/health/live` 永远 200；`/health/ready` 检查 Redis + qua + ASR providers |
| Q12 | 错误码 | 标准 Kratos Reason：`ASR_PROVIDER_UNAVAILABLE` 等 |
| Q13 | 外部系统解耦 | **三层解耦**，Source → RawEntity → Normalizer → NormalizedEntry |

---

## 五、8 层文本增强 Pipeline 详解

### 5.1 每层职责

| 层 | 输入 | 输出 | 主要规则 |
|---|---|---|---|
| 1. cleaning | 原始文本 | 清理后文本 | 多余空白、标点统一 |
| 2. filler | 清理后文本 | 去填充词文本 | "嗯"/"啊"/"那个" |
| 3. vocab_matching | 去填充词文本 | 词库匹配标记 | per-tenant 词库精确匹配 |
| 4. alias_resolution | 词库匹配 | 别名解析 | "张总" → "张三" |
| 5. deterministic_replacement | 别名解析 | 确定性替换 | 业务规则字典 |
| 6. phrase_standardization | 确定性替换 | 短语标准化 | "打开空调吧" → "打开空调" |
| 7. pinyin_correction | 短语标准化 | 拼音纠错 | "周丽群" → "佘丽群"（同音近字） |
| 8. fuzzy_matching | 拼音纠错 | 模糊匹配（兜底） | Levenshtein + 拼音双指标 |
| 9. context_correction | 模糊匹配 | 上下文纠错 | 未来扩展位 |

### 5.2 lexnorm 引擎特性

- **Functional Options**：所有 processor `NewXxxProcessor(opts ...Option)`
- **资源注入**：通过 Options 注入字典 / 配置 / logger
- **Observability**：
  - `Change.Processor / ProcessorVersion`（B1 修复：engine 自动反填）
  - `StepTiming.ChangeCount`（B4 文档：总数，不区分 Replace/Remove/Suggest）
  - `Variant.Confidence`（B2 文档：零值 0.0 = 未设置时静默 Skip）
  - `SuggestThreshold`（B3 文档：0 = 合法配置 = 不过滤）
- **HA 三层保护**：recover + ctx timeout + 错误累积
- **半开区间** `[start, end)`：所有 span 用半开区间，避免 off-by-one
- **sortedKeys**：跨进程 map 遍历顺序一致

---

## 六、工具公共化内化（P1–P2）

### 6.1 P1: pkg/credential

**最终结构**（已服务内化到 `app/evie/tool/pkg/credential/`）：

```
credential/
├── credential.go              Provider interface + CallerIdentity + 4 sentinel errors
├── mapping.go                 FieldMapper + MapFromMapper + ParseExpiresAt + LookupPath
├── mapping_test.go            8 sub-tests
├── adapter/biz.go             ctx 桥接（WithAuth / AuthFrom / CopyAuthContext）
├── redis/redis.go             RedisProvider
├── jwt/jwt.go                 JWTProvider (HS256/RS256, 自实现，无外部依赖)
├── jwt/jwt_test.go            10 sub-tests
├── static/static.go           StaticProvider (demo / 测试)
├── static/static_test.go
├── middleware/middleware.go   HTTPMiddleware + GRPCUnaryInterceptor
└── middleware/middleware_test.go
```

### 6.2 P2: pkg/source

**最终结构**（已服务内化到 `app/evie/tool/pkg/source/`）：

```
source/
├── source.go                  Source interface + RawEntity + Factory 注册中心
├── adapter/biz.go             biz.VocabularySource 桥接
├── http/http.go               通用 HTTP adapter (15 项 Config 字段)
├── http/http_test.go          10 sub-tests
├── file/file.go               JSON 文件 adapter (热重载)
├── file/file_test.go          8 sub-tests
└── qua/qua.go                 qua 协议特化 (薄包装 http)
```

### 6.3 公共化 vs 内化决策

- **原计划**：作为平台级公共包（`backend-service/pkg/credential`、`backend-service/pkg/source`）
- **最终决定**：服务内化（`app/evie/tool/pkg/*`）
- **理由**：credential 与 source 业务特性明显（qua 协议、oauth2_access_token），不应作为通用包；放服务内保留 pkg 复用优势但避免污染其他服务
- **撤回依赖**：jwt 从引用 `pkg/auth` 撤回，恢复自实现 HS256/RS256

---

## 七、HTTP/gRPC API 端点

### 7.1 HTTP

| 路径 | 方法 | 鉴权 | 说明 |
|---|---|---|---|
| `/health/live` | GET | ❌ | 进程存活（永远 200） |
| `/health/ready` | GET | ❌ | 依赖就绪（Redis + qua + ASR） |
| `/evie/tool/v1/enhance` | POST | ✅ | 纯文本增强（同步） |
| `/evie/tool/v1/asr:recognize` | POST | ✅ | 整段识别（funasr） |
| `/evie/tool/v1/asr/records` | GET | ✅ | 最近记录列表（分页） |
| `/evie/tool/v1/asr/records/:id` | GET | ✅ | 单条记录详情 |
| `/evie/tool/v1/asr/records/:id/audio` | GET | ✅ | 下载音频文件 |

### 7.2 gRPC

| 路径 | 说明 |
|---|---|
| `/evie/tool/v1/asr/stream` | 流式识别（双向流，session_id / format 通过 metadata） |

### 7.3 proto 命名约定

- HTTP 路由用 `:` 冒号：`/evie/tool/v1/asr:recognize`
- gRPC 流式参数走 metadata 而非 body

---

## 八、配置文件

### 8.1 `configs/config.yaml`（生产）

```yaml
server:
  http:
    addr: 0.0.0.0:8110
  grpc:
    addr: 0.0.0.0:9110

asr:
  providers:
    - name: funasr
      type: funasr
      endpoint: http://funasr:10095
    - name: xunfei
      type: xunfei
      appId: ...
      apiKey: ...
      apiSecret: ...

qua:
  baseURL: http://qua:8080
  userPath: /admin-api/qua/member-extended/page?selectAll=true
  deptPath: /admin-api/system/dept/list
  syncInterval: 5m

credential:
  redis:
    client: { addr: redis:6379 }
    keyPrefix: "oauth2_access_token:"
    fields: { tenantId, userId, userInfo.nickname, ... }
```

### 8.2 `configs/config.demo.yaml`（公共化演示）

```yaml
credential:
  static:
    defaultTenant: demo
    users:
      - { token: "demo-token", userId: "u1", userName: "Demo" }

vocabulary:
  sources:
    - name: file
      config: { path: ./configs/dictionaries/demo_vocab.json }
```

### 8.3 `configs/dictionaries/system.json`（系统静态词条）

- git 跟踪
- 热加载（可选）
- 内容：人名、产品名、专有名词等（黑种籽 / 周丽群 等）

---

## 九、测试覆盖

### 9.1 测试包清单（11 包全绿）

```
app/evie/tool/internal/biz               ✅
app/evie/tool/internal/biz/processor     ✅
app/evie/tool/internal/data              ✅
app/evie/tool/internal/server            ✅
app/evie/tool/internal/service           ✅
app/evie/tool/pkg/credential             ✅
app/evie/tool/pkg/credential/jwt         ✅
app/evie/tool/pkg/credential/middleware  ✅
app/evie/tool/pkg/credential/static      ✅
app/evie/tool/pkg/source/file            ✅
app/evie/tool/pkg/source/http            ✅
```

### 9.2 关键测试

- **fuzzy_vocab**：8 helper + 2 端到端测试（233 行）
- **credential/jwt**：10 sub-tests（HS256/RS256，alg mismatch，issuer/audience，malformed）
- **credential/middleware**：HTTP Bearer 解析 + gRPC metadata + FromContext
- **source/http**：httptest mock + token/tenant header + code error + QueryParams
- **source/file**：JSON 解析 + 热重载 + 缺字段 + 空数组
- **qua_client**：9 测试保兼容

### 9.3 真实链路回归

**3 次晨会录音**（同一 MP3）：

```
run  changes  fuzzyR  alias  filler  周丽群→佘丽群
1   76        2      18     8       1
2   76        2      18     8       1
3   76        2      18     8       1
```

完全稳定。fuzzy 命中（周丽群→佘丽群、测试播→测试1）。

---

## 十、已完成的修复与重构

### 10.1 P1–P6 修复（修复阶段）

| # | 问题 | 修复 |
|---|---|---|
| P1 | 冷启动 cache miss | Build 同步等待 lazy sync |
| P2 | lexnorm.Config 阈值冲突 | biz.NewLexnormEngine 设 0.5/0.0 |
| P3 | 命名错位 Enhance → Normalize | **保留**（用户决定不做） |
| P4 | 业务词渗透 | **保留**（用户决定不做） |
| P5 | convertRawToVocab 丢失 Priority | 加 `Priority: int(n.Priority)` |
| P6 | fuzzy_vocab 重复触发 | 用 `s.Changes()` + `hasChangeAtSpan` helper |

### 10.2 B1–B4 observability 修复

| # | 问题 | 修复 |
|---|---|---|
| B1 | Change.Processor 空 | reverse-fill in engine.runProcessors |
| B2 | fuzzy.Variant.Confidence 文档 | 文档化零值 0.0 = Skip |
| B3 | SuggestThreshold=0 文档 | 文档化 0 = 不过滤 |
| B4 | StepTiming.ChangeCount 文档 | 文档化总数、不区分 type |

### 10.3 代码全方位重构（P1–P5 5 阶段）

| 阶段 | 内容 |
|---|---|
| P1 | 清理死代码（占位符、未用 import） |
| P2 | 拆 StreamRecognize → 5 helper；Build 抽 waitForLazySync |
| P3 | errors.Join 替代手写；bodyPreview 限 200 字节 |
| P4 | 提取常量（lazySyncWaitTimeout / streamBufferSize / defaultSampleRate） |
| P5 | fuzzy_vocab 8 helper + 2 端到端测试 |

---

## 十一、当前提交历史

```
backend-service (codex/ai)
├── a9513a1  P1/P2/P5/P6 + system 黑种籽词条
├── 4de9797  pkg/lexnorm B1-B4
├── 7e416dc  evie/tool 代码全方位重构
├── a37bfc2  pkg/credential 抽象 P1
├── 0ab7d39  credential + source 服务内化
└── 1242d29  credential + source 双语 README

avmc 根仓库 (ark-tech)
└── 965b392  指针更新到 1242d29
```

---

## 十二、未完成计划（当前断点）

| # | 任务 | 状态 | 备注 |
|---|---|---|---|
| 1 | **D4.3** `app/evie/tool/README.md` 重写为开源版 | ❌ 未开始 | 当前仍是 M0–M9 旧版，没提到工具公共化、D4 介绍 |
| 2 | **2.1** `11-...开发计划.md` 设计文档更新 | ❌ 未开始 | M3b/M3c/M3d/M4-M9 全部显示为 🚧/📋，实际代码已全部 ✅ |
| 3 | **2.2** `13-...pkg设计.md` 文档更新 | ❌ 未开始 | 文档状态显示"设计确认中"，实际 pkg/source 已交付 |
| 4 | **2.3** `18-evie-tool修复与pkg-lexnorm问题清单` | ❌ 未完成 | 上一轮总结提到写过该路径，实际文件不存在 |
| 5 | **2.4** `docs/architecture/4-6` 当前断点更新 | ❌ 未开始 | evie/tool 完成度（含 P1-P6修复 / 工具公共化 / 服务内化）需写入断点恢复 |
| 6 | **4.1** demo 配置 e2e 验证 | ❌ 未跑过 | `configs/config.demo.yaml` + `configs/dictionaries/demo_vocab.json` 已加，但**真实启动 + 完整链路验证**还没做（应使用 StaticProvider + FileSource） |
| 7 | P3 / P4 技术债 | ⏸️ 用户明确不做 | 命名错位 Enhance→Normalize、业务词渗透清理 |

### 12.1 推荐下一动作

按用户优先级：

1. **D4.3 主 README 重写** — 这是用户最后明确指示但未完成的任务
2. **2.4 更新 4-6 当前断点** — 同步 git 已提交但清单没更新
3. **4.1 demo 配置 e2e 验证** — 公共化验收的最后一步
4. P3 / P4 保留为后续技术债

---

## 十三、补充说明

### 13.1 不在范围

- OAuth2 授权码流程（只校验，不签发）
- 会话管理 / 刷新 token
- 权限策略（Casbin）— 上层 `pkg/auth` 处理
- 增量同步（CDC / webhook）— 只做全量拉取
- 数据库 — 零数据库设计

### 13.2 服务启动

```bash
cd backend-service/app/evie/tool
make proto && make config && make wire
make run                  # 生产配置
make demo                 # 演示配置（StaticProvider + FileSource）
```

### 13.3 真实录音回归命令

```bash
go run ./testdata/inject_token/                       # 注入 oauth2_access_token
curl -X POST http://127.0.0.1:8110/evie/tool/v1/asr:recognize \
  -H 'Authorization: Bearer <token>' \
  -H 'Content-Type: application/json' \
  -d @request.json
```

---

## 十四、相关文档索引

| 路径 | 说明 |
|---|---|
| `../README.md` | 服务主 README（待重写为开源版） |
| `../../pkg/credential/README.md` | 认证抽象双语 README |
| `../../pkg/source/README.md` | 数据源抽象双语 README |
| `../../pkg/lexnorm/` | lexnorm 引擎包（含自身 README） |
| `../../pkg/asr/` | ASR provider 抽象包 |
| `../../docs/services/evie-platform/development/11-...开发计划.md` | 原始 M0–M9 设计 |
| `../../docs/services/evie-platform/development/12-...M6设计模式方案.md` | 8 层 Pipeline 设计 |
| `../../docs/services/evie-platform/development/13-...pkg设计.md` | 外部 API pkg 设计（待同步） |
| `../../docs/services/evie-platform/development/14-...错误码文档.md` | 错误码定义 |
| `../../docs/architecture/4-6-治理-开发功能清单.md` | 平台开发功能清单（待更新当前断点） |

---

**最后更新**：当前会话结束点
**服务成熟度**：✅ v1 生产就绪（11 测试包全绿 + 真实链路稳定）
**下一步**：D4.3 主 README 重写

---

> 💡 **新对话开始时的快速恢复指引**：
>
> 1. 读 `docs/SERVICE_REQUIREMENTS.md`（本文档）了解完整服务需求
> 2. 读 `README.md` 了解当前状态与快速开始
> 3. 读 `pkg/credential/README.md` + `pkg/source/README.md` 了解工具抽象
> 4. 检查「未完成计划」章节，按优先级继续
> 5. 修改前查 `.agents/RULES.md`（结构与开发规则）+ `.agents/DESIGN.md`（产品 UI 设计规则）