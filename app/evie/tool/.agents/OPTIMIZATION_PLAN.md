# evie/tool · 生产就绪优化增强计划

> 目标：在**不偏离现有产品定位**（独立轻量、零数据库、配置驱动、无 UI/无完整词库中心）的前提下，把当前 5 分左右的生产就绪度提升到可对外发布的开源级质量标准。
> 依据：`.agents/AUDIT.md` 的评分与问题清单。
> 执行原则：需求/边界优先，架构决策先行，安全与正确性优先于新功能，文档与代码同步。

---

## 1. 目标画像

### 1.1 保持的产品边界

- 独立轻量语音识别增强工具
- 零数据库：内存 + 文件 + Redis
- 配置驱动：YAML 配置 + system.json + Normalizer 规则
- 多 Provider ASR：funasr / xunfei / mock
- 8 层文本增强 Pipeline
- Bearer Token 认证：redis / jwt / static
- qua/HTTP/File 词库同步
- 健康检查、本地音频落盘

### 1.2 明确不做

- 不引入数据库 / ORM / 持久化业务数据
- 不做管理后台 UI / 完整词库中心
- 不做 OAuth2 授权码签发 / 刷新 token / 会话管理
- 不做 Casbin 权限策略
- 不做 CDC / webhook 增量同步
- 不做云存储 / 对象存储音频上传
- 不改变对外 HTTP/gRPC 路径与消息语义（除非为安全/正确性做兼容修复并升级版本）

### 1.3 生产就绪标准

- 安全默认开启：租户隔离、token 过期、校验、路径安全、无硬编码凭据
- 所有后台任务有明确生命周期，无 goroutine 泄漏/死锁
- 健康检查反映真实依赖状态
- 文档与代码一致，demo 可一键跑通
- `go test -race ./...`、`go vet`、lint 全绿
- 关键路径有 benchmark 基线和性能回归保护
- 对外发布满足开源仓库基础：README、LICENSE、CHANGELOG、示例、CI

---

## 2. 目标评分

| 维度 | 当前 | 目标 |
|---|---:|---:|
| 正确性 | 6 | 9 |
| 稳定性 | 5 | 8.5 |
| 安全性 | 4 | 9 |
| 效率 | 6 | 8 |
| 文档/产品化 | 3 | 9 |
| 整体生产就绪度 | 5 | 9 |

---

## 3. Phase 0：架构与需求校准（进入开发前必须先完成）

**目标**：消除“文档宣称”和“代码实际能力”之间的根本分歧。

1. 建立“当前真实能力矩阵”
   - 按 `docs/SERVICE_REQUIREMENTS.md` 逐项列出：已实现 / 部分实现 / 未实现。
   - 用 `.agents/AUDIT.md` 作为起点，更新到 docs。

2. 必须澄清的架构决策

   | 决策点 | 当前问题 | 建议方向 |
   |---|---|---|
   | 后台词库同步身份来源 | 后台 ctx 无 AuthInfo，Warmup/runOnce 实际跳过 | 支持“同步身份 Provider”：单租户/静态 token、服务账号 token；不可用则明确退化为“仅请求级 lazy sync”，文档不得宣称定时同步 |
   | tenant registry 来源 | 不读文件，无法启动预热 | 实现 `tenants.json` 加载；若改为动态发现，必须说明身份来源 |
   | 词库同步的租户语义 | `RawEntity.Source` 被当成 tenant | 明确 SyncTenant 的 tenantID 来自 registry/auth，不来自 RawEntity.Source |
   | ASR 记录权限粒度 | 全局 ring buffer | 按 proto 注释实现“本租户可见”；是否限 user 由需求确认 |
   | 后台任务是否必须存在 | 若无 service account 则无法周期拉取 | 若产品接受 lazy-only，则修订文档；若必须周期同步，则补同步身份配置 |

3. 输出物
   - 更新 `docs/SERVICE_REQUIREMENTS.md` 的“技术决策”和“未完成计划”。
   - 在 `.agents/` 记录决策日志。

---

## 4. Phase 1：安全加固（P0，最高优先）

对应评分 Security 4 → 9。

### 4.1 租户数据隔离

- `ListRecords / GetRecord / GetRecordAudio` 全部接收 tenantID。
- biz 层按 `TenantID` 过滤记录；跨租户访问返回 `NotFound`，避免泄露存在性。
- proto 若允许按用户查看，则进一步增加 `UserID` 维度；默认至少做到租户级。
- 增加测试：
  - tenant A 看不到 tenant B 的 record。
  - tenant A 无法下载 tenant B 的 audio。
  - 无认证访问 records 返回 401。

### 4.2 Token 过期校验

- RedisProvider、StaticProvider 在解析出 `ExpiresAt` 后执行过期判断。
- 过期 token 返回 `ErrTokenInvalid`，并映射到现有 401。
- 增加配置化的 `Leeway`，保持与 JWT 一致。
- 增加测试：未过期通过、已过期拒绝、零值不拒绝。

### 4.3 请求校验链路

- HTTP/gRPC server 接入 Kratos `validate.Validator()` 或 protovalidate middleware。
- 确保 `RecognizeRequest.audio_data.max_len=10MB`、`EnhanceTextRequest.text.min_len`、`GetRecordRequest.id.min_len` 等规则真正生效。
- 增加 HTTP/gRPC 校验失败测试。

### 4.4 文件路径安全

- session_id：后端统一生成；客户端传入值必须通过白名单校验（`[A-Za-z0-9_-]`），非法则拒绝或替换。
- tenant_id：写入路径前做安全规范化，禁止 `..` / 绝对路径 / 分隔符穿越。
- 读取 audio 前校验 `filepath.Rel(audioDir, resolvedPath)` 不越界。
- 增加路径穿越单测：`../../evil`、绝对路径、超长路径。

### 4.5 凭据与配置安全

- 移除 `configs/config.yaml` 中硬编码的 Redis 密码 / 讯飞 AppSecret。
- 改为环境变量或 secret 注入：`${XUNFEI_API_SECRET}` 等。
- 提供 `configs/config.example.yaml` / `.env.example`，并把真实敏感配置加入 gitignore 或占位。
- CI 扫描确认无真实 secret。

### 4.6 基础传输安全

- HTTP/gRPC 对外默认说明应使用 TLS / 内网部署；README 写明生产部署要求。
- 可选增加基础限流中间件，防止大音频/流式滥用。

---

## 5. Phase 2：核心正确性与稳定性（P1）

对应评分 Correctness 6→9、Stability 5→8.5。

### 5.1 重构词库同步链路

目标：让“预加载 + 周期同步 + 请求 lazy sync”三者语义清晰且可用。

1. `TenantRegistry`
   - 实现从 `configs/tenants.json` 加载。
   - 支持静态租户列表；运行时 `Ensure` 仅追加，不丢失文件来源。
2. 同步身份 Provider
   - 新增可配置的同步凭据来源：`static` / `redis` / 环境变量，供后台 Warmup/Run 使用。
   - 若未配置，则禁止声称“定时同步”，health/details 显示 `sync_mode=lazy_only`。
3. `VocabSyncer`
   - `runOnce` 不再按 `RawEntity.Source` 分组。
   - 遍历 registry tenant，使用对应 tenant 上下文调 `SyncTenant`。
   - Warmup 与 Run 共用同一套 tenant 迭代逻辑。
   - `HealthChecker.SetSyncState` 在成功/失败时都记录真实时间与错误。
4. `VocabularyBuilder`
   - 记录每个 tenant 的真实 `last_sync_at`，运维接口可读。
   - fallback 语义保留：sync 失败不清空旧快照。

### 5.2 补全拼音派生

- 实现 `derivePinyin` / `derivePinyinInitial`，复用 `backend-service/pkg/pinyin`。
- 对 qua 同步用户/部门条目、system.json 条目统一派生。
- 确认 lexnorm bridge 能把这些 Pinyin 信息传给 `lexicon.Entry.Meta`，供 pinyin/fuzzy 使用。

### 5.3 Stream 生命周期修复

- `StreamRecognize` 所有提前返回路径必须关闭 `resultCh`，避免 service sender goroutine 永久等待。
- 在启动 sender goroutine 前完成 stream provider 可用性检查。
- 增加 context cancel 清理：
  - provider goroutine 退出；
  - providerOut 不再写入；
  - audioCh / resultCh 正确关闭；
  - 不等待永不发送的 goroutine。
- 增加测试：
  - stream provider 为 nil 时快速返回错误；
  - 客户端 cancel 后 handler 不 hang；
  - 正常流式结束能收到 final 帧并关闭。

### 5.4 错误码与错误处理

- 统一 biz/data/service 错误语义：
  - 未认证 → 401
  - ASR Provider 不可用 → `ASR_PROVIDER_UNAVAILABLE`
  - qua 错误 → 现有 v1 错误
  - 业务校验失败 → 400
- 避免 service 层把内部 error 直接转成 `Internal`。
- `pkg/source/http` 的 partial failure 改为 `errors.Join`，避免字符串拼接导致无法 `errors.Is/As`。

### 5.5 健康检查真实化

- 为 ASR Provider 定义/补齐健康探测接口。
  - funasr 本地服务可探测 HTTP 可达性；
  - 讯飞若无法简单探测，则 readiness 应区分 `configured` / `unknown`，不能直接 ok。
- `HealthChecker.Ready` 在依赖不可达时返回 503。
- 补充测试：provider 远端 down 时 ready 失败。

### 5.6 配置路径统一

- 所有相对路径（audio_dir、system_dict、tenant_registry）统一按“工作区根/服务根”解析。
- 修复 cwd 在 `backend-service/` 和 `app/evie/tool/` 下不一致的问题。
- 修复 README 端口、Makefile target 等文档偏差。

---

## 6. Phase 3：可观测性与运维能力（P2）

对应稳定性/生产运维提升。

1. 结构化日志
   - 统一 module/tenant/session_id/request_id 字段。
   - 敏感信息（token、apiSecret）禁止入日志。
2. Metrics
   - 增加 HTTP/gRPC 请求计数、耗时、错误率。
   - 增加 ASR provider 调用耗时/失败率、词库同步状态、snapshot 数量。
3. 健康详情
   - `/health/ready` 返回结构化 details：
     - redis 状态
     - qua 状态
     - asr provider 状态
     - vocab sync mode / last sync / last error
4. Graceful Shutdown
   - 确保 stream goroutine、syncer ticker、provider 连接在退出时关闭。
5. 配置启动校验
   - 启动时校验 provider 组合是否合法（如 stream provider 缺失但暴露 stream API）。
   - 校验配置字段类型、路径、端口、必需项，失败快速退出。

---

## 7. Phase 4：性能与质量工程（P2）

对应 Efficiency 6→8。

1. Benchmark 基线
   - `EnhanceText`：空词库 / 系统词库 / 千级租户词库。
   - `fuzzy_vocab`：典型文本长度 + 典型词条规模。
   - ASR audio save / ring buffer。
   - 输出到 `docs/benchmarks.md` 或 Go benchmark 文件。
2. 词库/匹配优化
   - 对 fuzzy_vocab 候选索引增加“按长度桶内有序 + 剪枝/退出阈值”等可控优化。
   - 但必须先有 benchmark，再做优化；不得牺牲确定性。
3. 内存记录结构优化
   - ring buffer 按 tenant 分片，或增加 tenant → records index，避免全量遍历。
   - 列表分页基于稳定排序快照，避免每次 O(n log n)。
4. Source HTTP
   - 限制响应体读取大小，防止上游异常大响应拖垮服务。
   - 使用 context timeout，避免泄漏。
5. 并发热点
   - 若 profiling 显示锁竞争，再把 tenant snapshot map 升级为 `atomic.Pointer` 等。
   - 保持“先正确、再性能”。

---

## 8. Phase 5：开源产品化与文档收口（P3）

对应文档/产品化 3→9。

1. README 重写
   - 项目定位、架构图、快速开始、demo、生产部署、配置参考、API 列表、安全说明、FAQ。
2. 可运行 Demo
   - 真正支持 `make demo`：StaticProvider + FileSource + mock ASR，零外部依赖启动。
   - 提供示例请求/响应和自动验证脚本。
3. Makefile / CI
   - 补齐 `proto` / `config` / `wire` / `demo` / `test` / `race` / `bench` / `lint` / `check`。
   - CI：gofmt、go vet、go test -race、coverage、buf lint/breaking。
4. API 文档与示例
   - OpenAPI 生成、curl/gRPC 示例、错误码文档。
   - 保持 HTTP 路由用冒号自定义动作的约定。
5. 发布规范
   - 版本号、CHANGELOG、LICENSE。
   - 每个阶段完成时同步 `docs/SERVICE_REQUIREMENTS.md` 与 `.agents/*`。

---

## 9. 执行顺序与里程碑

| 阶段 | 内容 | 预计产出 | 退出条件 |
|---|---|---|---|
| P0 | 架构与需求校准 | 决策记录、真实能力矩阵 | 同步模型/权限模型/定时同步身份已确认 |
| P1 | 安全加固 | 安全修复 + 安全测试 | 无跨租户访问、token 过期拒绝、validate 生效、无硬编码 secret、路径穿越测试通过 |
| P2 | 核心正确性/稳定性 | 词库同步重构、拼音、stream 修复、health 真实化 | 后台同步可测、stream 无 hang、ready 可信 |
| P3 | 可观测性 | metrics/logs/health details | 运维可定位问题，无敏感日志 |
| P4 | 性能与质量 | benchmark、结构优化、CI 强化 | 基线可对比，无确定性回归 |
| P5 | 开源产品化 | README、demo、Makefile、CI、发布包 | 新 clone 后一条命令可跑通 demo/test |

---

## 10. 禁止事项（防偏离）

- 禁止在 Phase 1–5 中引入数据库、管理后台 UI、完整词库中心功能。
- 禁止扩大服务边界去承接 evie/service 的审计/版本管理/UI。
- 禁止为了“生产化”引入重量级分布式组件（消息队列、配置中心、注册中心、APM 全家桶）。
- 禁止把当前工具改造成通用 ASR 平台或 Agent 框架。
- 禁止未经决策修改对外 API 路径和消息语义。
- 每个阶段必须保留轻量、零数据库、配置驱动的产品特征。

---

## 11. 文档维护

每个 Phase 完成后的更新点：

- `docs/SERVICE_REQUIREMENTS.md`：能力矩阵、技术决策、未完成计划、当前断点
- `.agents/AUDIT.md`：刷新不一致清单与评分
- `.agents/OPTIMIZATION_PLAN.md`：勾选完成阶段
- `README.md` / pkg README：用户可见说明

## 12. 迭代进度（随迭代刷新）

| 阶段/决策 | 状态 | 摘要 |
|---|---|---|
| P0 决策校准 | ✅ | `.agents/DECISIONS.md` 已收录 D1–D10 |
| P1 安全加固 | ✅ | 租户隔离、token 过期、validate、路径、凭据 |
| P2 词库同步模型 D1/D2 | ✅ | `TenantRegistry` 读文件 + 同步 token + `SyncMode` 上报 |
| P2 Stream 生命周期 | ✅ | `resultChClosed` 保障关闭 |
| P2 拼音派生 | ✅ | 接入 `pkg/pinyin` |
| P2 健康模式可观测 | ✅ | `/health/ready` 含 `vocab_sync_mode` |
| P2 ASR 记录 D4 隔离 | ✅ | 双索引 + `TenantRecordCount` + 单测覆盖 |
| P3 启动配置校验 / metrics | ✅ | conf.Validate + 零依赖 Prometheus 文本指标 `/metrics` + HTTP/ASR/Sync 维度 |
| P4 benchmark / 性能 | ✅ | EnhanceText / ASR Append+List / FuzzyVocab 基线已落地；`make bench` 可重跑；`make lint` 增加 `go vet` 门禁 |
| P5 README / demo / CI | ✅ | README 重写、`.github/workflows/ci.yaml`、`make proto/wire/test/cover/lint/bench` + `make demo`（EVIE_TOOL_DEMO=1 静态 token + 文件词源，端到端验证通过） |
