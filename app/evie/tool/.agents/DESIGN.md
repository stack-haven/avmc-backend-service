# evie/tool · 设计原则与产品边界

> 本文件定义“什么属于 evie/tool、什么不属于 evie/tool”，以及服务内部设计边界。
> 当前服务是后端轻量工具，不包含管理后台 UI；若后续出现 UI 需求，再引用根仓库 `.agents/DESIGN.md` 的产品/UI 规则。

---

## 1. 服务定位

`evie/tool` 是 `app/evie/service` 的**无 DB、配置驱动、轻量化变体**。

一句话定义：

> 一个为语音识别结果提供“多 ASR Provider 接入 + 8 层文本增强 + per-tenant 词库”的独立轻量工具服务。

| 维度 | evie/service | evie/tool |
|---|---|---|
| 数据存储 | Ent + MySQL/Postgres | 内存 + 文件 + Redis |
| 词库管理 | UI 增删改查 | 配置驱动 + qua 同步 |
| ASR | 单一服务 | 多 Provider（funasr/xunfei） |
| 文本增强 | 8 层 Pipeline | 8 层 Pipeline |
| 鉴权 | gRPC 委托 | Bearer Token + Provider 抽象 |
| 部署形态 | 微服务 | 独立工具 / Sidecar |

---

## 2. 产品边界

### 2.1 属于 evie/tool

- 音频整段识别（funasr / mock）
- 流式识别（xunfei / 双向流）
- 纯文本增强 `/evie/tool/v1/enhance`
- 最近识别记录与音频回放（内存版）
- qua/文件/HTTP 词库同步 + per-tenant 内存词库
- Bearer Token 认证：redis / jwt / static
- 健康检查

### 2.2 不属于 evie/tool（除非用户显式扩大边界）

- 词库中心 UI、审计、版本管理、数据库持久化
- OAuth2 授权码签发、刷新 token、会话管理
- Casbin 权限策略
- 增量同步（CDC / webhook）
- 音频上传云存储
- 完整词库中心后台

---

## 3. 设计原则

### 3.1 零数据库 / 配置驱动

- 运行时数据放在内存（词库快照、最近 ASR 记录、ring buffer）。
- Redis 只用于认证 token 查询，不作为业务数据库。
- 服务行为由 `configs/*.yaml` + `system.json` + YAML Normalizer 规则驱动。

### 3.2 业务层不感知 transport

- `biz` 只处理用例；HTTP/gRPC 转换在 `service`。
- ASR Provider 通过接口注入；新增 Provider 不需要改 biz 主流程。

### 3.3 Q13 外部系统解耦

- 外部数据统一 `Source → RawEntity → Normalizer → NormalizedEntry → VocabularyBuilder`。
- `RawEntity.Data` 不透明，Source 包不依赖业务字段名。
- Normalizer 规则通过 YAML 配置热改，不硬编码在代码里。

### 3.4 HA 优先

- 词库同步失败不阻塞 ASR/增强。
- 增强失败保留 ASR 原文并返回降级状态。
- lexnorm Pipeline 有 panic/timeout/error 三层保护。
- 首请求 cache miss 有 5s lazy sync 等待 + system/fallback 兜底。

### 3.5 可观测与可解释

- 文本增强结果带 Changes、Steps、Processor、Confidence、Error。
- 输出保留 `OriginalText` 与 `EnhancedText`。
- 变更记录可用于审计与后续回归。

### 3.6 确定性

- 输出顺序、匹配结果、同音/模糊命中不受 Go map 随机序影响。
- 所有 map 遍历影响输出时必须排序。

---

## 4. 关键抽象与包边界

### 4.1 `pkg/credential`

```text
Provider.Authenticate(ctx, token) → *CallerIdentity
```

- 服务内化，不污染平台公共包。
- `FieldMapper` 让字段名可配置。
- sentinel errors：`ErrTokenNotFound / ErrTokenInvalid / ErrProviderUnavailable / ErrInvalidConfig`。
- Provider：redis / jwt / static。

### 4.2 `pkg/source`

```text
Source.Fetch(ctx) → []RawEntity
```

- 服务内化，不污染平台公共包。
- Factory 注册中心按 name 构建 Source。
- Adapter 桥接到 `biz.VocabularySource`。
- 实现：http / file / qua。

### 4.3 `internal/biz`

- 业务用例不写基础设施细节。
- 定义接口由 data 实现（如 `VocabularySource`、`AuthContext`）。
- 词库构建链路：`VocabularyBuilder` + `VocabSyncer` + `TenantProfileResolver`。

### 4.4 `internal/data`

- Redis client、TokenCache、QuaClient、ASR provider 装配、HealthChecker。
- 可依赖 `biz` 接口，不可被 biz 反向依赖。

---

## 5. 8 层 Pipeline 语义

| 业务层名称 | 内部 processor | 作用 |
|---|---|---|
| cleaning | normalize | 清理空白/统一标点 |
| filler | disfluency | 移除“嗯/啊/那个” |
| vocab_matching | alias | 词库精确匹配 |
| alias_resolution | alias | 别名“张总”→“张三” |
| deterministic_replacement | deterministic | 确定性替换 |
| phrase_standardization | deterministic (PHRASE) | 短语标准化 |
| pinyin_correction | pinyin | 同音纠错 |
| fuzzy_matching | fuzzy_vocab | 模糊兜底 |
| context_correction | ctxproc | 上下文扩展位 |

> 说明：业务文档的 8 层与 lexnorm processor 名称存在映射关系；修改业务层描述时不要误以为 lexnorm 没有 phrase_standardization 独立 processor。

---

## 6. 决策摘要（Q1–Q13）

| ID | 决策 |
|---|---|
| Q1 | evie/tool 是 evie/service 的无 DB 配置驱动变体 |
| Q2 | Bearer Token + Redis `oauth2_access_token:<token>` |
| Q3 | 词库内存存储 + 启动预加载 + 周期同步 |
| Q4 | ASR Provider 按配置路由 |
| Q5 | qua 端点：member-extended/page + dept/list |
| Q6 | qua 响应壳：Spring Cloud `{code,msg,data}` |
| Q7 | 讯飞凭证默认可启动联调 |
| Q8 | system.json git 跟踪 + 热加载 |
| Q9 | 租户级不持久化，重启重建 |
| Q10 | 音频本地 upload/audio |
| Q11 | live 永远 200；ready 检查依赖 |
| Q12 | 错误码标准 Kratos Reason |
| Q13 | 外部系统解耦三层（Source/RawEntity/Normalizer） |

---

## 7. 当前技术债 / 注意点

- `README.md` 仍是旧版 M0–M9 文档，需在后续迭代重写为开源版。
- `config.demo.yaml` + FileSource/StaticProvider 尚未完成真实 e2e 接线与验证。
- 部分父目录设计文档仍显示“未完成”，实际代码已交付；同步时以当前代码和 `docs/SERVICE_REQUIREMENTS.md` 为准。
- `internal/conf/conf.proto` 目前未覆盖 demo 配置的 `credential` / `vocabulary` 段。

---

## 8. 自检清单

任何设计变更前回答：

- [ ] 是否违反服务边界（把 DB/UI/审计等拉进 evie/tool）？
- [ ] 是否破坏 `service → biz → data` 方向？
- [ ] 是否在 `pkg/source` / `pkg/credential` 写入业务字段硬编码？
- [ ] 是否引入全局可变状态或继承模拟？
- [ ] 是否保证输出确定性？
- [ ] 是否同步更新 `docs/SERVICE_REQUIREMENTS.md` 与 `.agents/*`？
