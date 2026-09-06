# evie/tool · 拓展方案（v0.x → v1.x 演进路线图）

> 目标：在**不偏离现有产品定位**（零数据库、配置驱动、无 UI/无完整词库中心）的前提下，把当前已经具备多 Provider ASR + 8 层增强 + per-tenant 词库 + Bearer 认证的“轻量工具”，演进为对**开发者友好、生产可观测、协议可组合、运行可治理**的开源产品。
> 本文件只做方案与权衡分析，**不**直接进入实现；每个 Phase 开始前需要业务方在对应决策点签字确认。

---

## 1. 现有能力快照（基线）

| 维度 | 现状 |
|---|---|
| 协议 | HTTP（Kratos 路由 + Google HTTP annotations）、gRPC（ASRService.StreamRecognize 双向流） |
| ASR Provider | funasr（整段）、xunfei（流式）、mock（测试） |
| 增强 Pipeline | 8 层 lexnorm：cleaning / filler / alias / deterministic / pinyin / fuzzy / ctx |
| 词库 | per-tenant 内存快照；Q13 三层解耦（Source → RawEntity → Normalizer） |
| 词库来源 | Qua（HTTP）、HTTP（任意 REST）、File（本地 JSON） |
| 认证 | Bearer Token：Redis（生产） / JWT（自签） / Static（demo） |
| 音频 | 本地 `upload/audio/<tenant>/<session>.<ext>`；ring buffer 1000 条 |
| 同步模式 | `background`（配 sync_token）/ `lazy_only`（默认） |
| 可观测性 | `/health/live` `/health/ready` `/metrics`（Prometheus 文本，0 依赖） |
| Demo | `EVIE_TOOL_DEMO=1` 零外部依赖启动；`make demo` |
| 质量门禁 | gofmt / go vet / `go test` / `go test -race` / bench 基线 |
| CI | `.github/workflows/ci.yaml`（gofmt / vet / test / race / bench） |
| 测试覆盖 | 14 包，500+ 用例（unit + E2E + race + bench） |

---

## 2. 扩展边界（绝对不能做）

> 与 `docs/SERVICE_REQUIREMENTS.md` 一致，以下方向**任何阶段都不能加入**：

- 引入业务数据库（MySQL/Postgres/SQLite）
- 引入管理后台 UI / 完整词库中心
- 引入 OAuth2 授权码签发 / 刷新 token / 会话管理
- 引入 Casbin / 复杂权限策略
- 引入 CDC / webhook 增量同步
- 引入云存储 / 对象存储上传
- 改造为通用 ASR 平台或 Agent 框架
- 单仓内置 evie/service 的词库审计 / 版本管理
- 引入重量级分布式组件（消息队列 / 注册中心 / 配置中心 / APM 全家桶）

允许加入的只是**横向能力**（协议、治理、安全、可移植性），不引入新的核心业务复杂度。

---

## 3. 扩展维度（候选）

按影响面和成本从低到高排序，每项标注“与产品边界是否冲突”。

### A. 开发者生态（低风险，高 ROI）

| 项 | 说明 | 边界合规 |
|---|---|---|
| **A1：稳定 Go SDK 形态** | 把 `internal/biz`、`pkg/credential`、`pkg/source` 的核心 API 抽取为 `pkg/sdk`（SemVer），提供 `go get` 入口；现有服务内部也切换到 SDK，验证可用性。 | ✅ |
| **A2：插件式 Processor** | 把 `cleaning / filler / alias / deterministic / pinyin / fuzzy / ctx` 抽象为 `lexnorm.Processor` 接口的注册表，第三方可注入 Go 包（编译期 plugin 而非 .so），通过 `config.yaml` 启用。 | ✅ |
| **A3：示例与教程仓库** | 单独 `examples/`：1) 嵌入 SDK 写一个简化的“会议文本润色器”；2) 自定义 Processor；3) 自定义 Source。 | ✅ |
| **A4：CLI 工具** | `cmd/evie-toolctl` 子命令：`validate-config` / `dump-vocab` / `tail-metrics`（本地拉 `/metrics` 渲染人类可读摘要）。 | ✅ |
| **A5：API 文档站** | 从 proto 自动生成 OpenAPI + 静态站点（mkdocs / docusaurus），包含“快速开始 / 配置参考 / 端点手册 / 性能基线 / 故障排查”五大块。 | ✅ |

### B. 可观测性 & 运维（低风险）

| 项 | 说明 | 边界合规 |
|---|---|---|
| **B1：结构化访问日志** | 把当前 Kratos log 升级为 JSON 输出（保持默认文本可选），字段：`ts / level / service / version / tenant / request_id / latency_ms / status / path / method`。 | ✅ |
| **B2：OpenTelemetry 集成** | 接入 `go.opentelemetry.io/otel`，traces + metrics；span 串联 HTTP → middleware → ASR provider → lexnorm，便于排障；保留 `/metrics` Prometheus 文本作为兜底。 | ✅ |
| **B3：SLO 指标** | 暴露 `evie_request_duration_seconds{quantile="0.95"}`（用 HDR histogram 或 t-digest），加上 error budget burn rate 指标。 | ✅ |
| **B4：pprof 端点** | 仅在 `EVIE_TOOL_PPROF=1` 时挂载 `net/http/pprof`；避免默认暴露。 | ✅ |
| **B5：审计日志** | 业务关键事件（record 创建 / 词库同步 / 健康状态变更）输出审计日志；可由 Loki/ELK 收集。 | ✅ |

### C. 弹性 & 质量（中风险）

| 项 | 说明 | 边界合规 |
|---|---|---|
| **C1：ASR Provider 熔断 + 失败回退** | 引入 `pkg/aegis` 熔断器：当主 provider 连续失败 N 次后切到备选 provider；恢复探测。 | ✅ |
| **C2：请求级超时分层** | 把 `conf.Asr` 增加 `timeouts.{recognize,stream}` 与 `enhance.timeout`；与 `lexnorm` 的 ctx timeout 联动。 | ✅ |
| **C3：幂等键** | `Idempotency-Key` header 支持；24h 内的相同 key 走 ring buffer 缓存的旧结果，避免重复 ASR 计费。 | ✅ |
| **C4：批量增强** | 新增 `POST /evie/tool/v1/enhance/batch`，单请求最多 N 条，底层并发 + 单条超时；返回结构保留 partial_ok。 | ✅ |
| **C5：fuzzy_vocab 启发式剪枝** | 在 1000 词库下把 9ms 优化到 < 2ms：长度桶内按编辑距离下界提前退出；带 benchmark 回归。 | ✅ |
| **C6：QA 评估 CLI** | `evie-toolctl eval` 读取一组 (audio, expected_text) fixture，输出命中率 / WER / p95 延迟；可接入 CI。 | ✅ |

### D. 协议 & 集成面（低-中风险）

| 项 | 说明 | 边界合规 |
|---|---|---|
| **D1：WebSocket 流式 ASR** | 新增 `GET /evie/tool/v1/asr/stream/ws`（RFC 6455），作为 gRPC stream 的 HTTP 友好替代；客户端 SDK 提供 JS / Python 样例。 | ✅ |
| **D2：SSE 推送增强进度** | 对长文本 / 大词库场景，`POST /enhance/stream` 返回 `text/event-stream`，每层输出增量。 | ✅ |
| **D3：v2 API 路由并行** | 路径 `/evie/tool/v2/...` 与 v1 共存；不再为兼容历史破坏 v1；proto 标注 deprecation。 | ✅ |
| **D4：Source/HTTP 协议补全** | `pkg/source/http` 支持 `Retry-After` 透传、`If-None-Match` 缓存、HTTP/2；对接企业 SSO 风格鉴权。 | ✅ |
| **D5：Processor 组合 DSL** | `config.yaml` 允许 `pipeline.steps: [{name, type, opts}, ...]`，以及条件 `when: result.confidence < 0.8`，参考 lexnorm 的 Profile 思路。 | ✅ |

### E. 安全 & 合规（低-中风险）

| 项 | 说明 | 边界合规 |
|---|---|---|
| **E1：mTLS** | HTTP/gRPC 服务端启用 TLS（依赖部署侧证书，仓库不落证书）；提供 `EVIE_TOOL_TLS_CERT/KEY` env。 | ✅ |
| **E2：OIDC 接入** | 增加 `oidc` Provider：用 discovery + JWKS 验签；与现有 JWT Provider 并存。 | ✅ |
| **E3：租户级限流** | Redis token bucket，每 tenant 每分钟 N 次（默认 60，可调）；超限返回 429。 | ✅ |
| **E4：审计与数据脱敏** | 识别敏感 token / 个人信息，写入审计时 hash；日志默认脱敏 `Authorization` header。 | ✅ |
| **E5：Secret 轮转** | 配置热加载后，`Credential.Provider` 支持多源顺序 fallback；通过 reload 触发重新加载。 | ✅ |
| **E6：依赖漏洞扫描** | CI 增加 `govulncheck` / `trivy`；在 PR 中阻断 high/critical。 | ✅ |

### F. 部署 & 打包（低风险）

| 项 | 说明 | 边界合规 |
|---|---|---|
| **F1：多架构镜像** | Docker manifest 支持 `linux/amd64` + `linux/arm64`；`scratch` 或 `distroless` 基底，最小层。 | ✅ |
| **F2：Helm Chart** | `deploy/helm/evie-tool`：`Deployment` + `Service` + `HPA`（基于 CPU/自定义指标）+ `PodDisruptionBudget` + `ServiceMonitor`（Prometheus Operator）。 | ✅ |
| **F3：kustomize 覆盖** | 提供 base + overlays（dev / staging / prod），覆盖 configmap / secret。 | ✅ |
| **F4：Systemd 单元** | 单机部署：`evie-tool.service` + `evie-tool.socket`（systemd 激活），含 restart / resource limits。 | ✅ |
| **F5：Go modules SemVer 化** | 把 `backend-service/app/evie/tool` 抽到独立 go module（`github.com/stack-haven/evie-tool`），与 `pkg/lexnorm` 同级；单仓 / 多仓皆可。 | ✅ |

### G. 国际化 & 多语言（低-中风险）

| 项 | 说明 | 边界合规 |
|---|---|---|
| **G1：英文 / 数字 normalization** | 增强 `cleaning` 增加英文/数字 token 化；新增 `english_normalize` processor。 | ✅ |
| **G2：多语言 pinyin/romaji** | 把 `pkg/pinyin` 抽象为 `Romanizer` interface，提供日文 romaji、韩文等实现；按 `meta.lang` 选择。 | ✅ |
| **G3：双语对齐词典** | `vocab_rules.sources.dual` 支持中-英对齐条目；`alias` 跨语言。 | ✅ |
| **G4：本地化错误信息** | i18n（go-i18n）按 `Accept-Language` 返回错误 message。 | ✅ |

### H. 调度 & 治理（低-中风险）

| 项 | 说明 | 边界合规 |
|---|---|---|
| **H1：Config 热加载** | fsnotify 监听 `config.yaml` / `vocab_rules` / `tenants.json`；变更通过 `validation` 后原子替换；不中断正在处理的请求。 | ✅ |
| **H2：运行时 Feature Flag** | 简易内存 flag（无外部服务），通过 `config.yaml` + `EVIE_TOOL_FLAG_X` env 覆盖；可被 reload 触发。 | ✅ |
| **H3：租户配额** | per-tenant：最大 records 200、最大音频大小 50MB、每分钟 ASR N 次；超限返回 429 + 详细信息。 | ✅ |
| **H4：审计事件总线** | 把 `vocab_sync / enhance_error / asr_provider_change` 事件输出到 `pkg/event` 接口；本地默认 `log`，未来可替换为 Kafka。 | ✅ |

---

## 4. 优先级矩阵（推荐序）

> 我**强烈建议**采用以下 4 个阶段推进（每个 Phase 2-4 周）。任何阶段都允许在用户决策点暂停或调整。

| 阶段 | 包含项 | 价值 | 成本 | 风险 | 依赖 |
|---|---|---|---|---|---|
| **Phase 6 / 治理基线** | A4 / A5 / B1 / B4 / E4 / F5 | 立刻提升开源仓库可读性、运维可定位性、合规基线 | 中 | 低 | 无 |
| **Phase 7 / 稳定性强化** | C1 / C2 / C3 / C5 / E3 / H3 | 解决目前 P0/P1 风险（fuzzy 慢、ASR 单点、租户打爆） | 中-高 | 中 | Phase 6 |
| **Phase 8 / 协议扩展** | A1 / A2 / D1 / D2 / D3 | 让工具被更多场景使用（嵌入、流式、版本兼容） | 中 | 中 | Phase 6 |
| **Phase 9 / 智能/语言扩展** | A3 / C4 / C6 / D4 / D5 / E2 / G1 / G2 | 真正走向“产品”层级（多语言、可评估、可组合） | 高 | 中-高 | Phase 7-8 |

**显式不做（即使收益高）**：

- 引入业务数据库 / 完整词库中心（违背边界）
- 引入 Kafka / Service Mesh / 注册中心（重量级，与 zero-dep 目标冲突）
- 改造成通用 Agent 平台（偏离 ASR 增强定位）
- 引入大模型实时调用（成本/可控性风险；如需要，预留 processor interface 通过 B1/A2 接入第三方实现）

---

## 5. 关键决策点（需要用户签字）

1. **A2 插件式 Processor 的形态**：编译期 `import` vs 运行期 `.so`？
   - 建议：编译期（`pkg/processor/registry` 注入），保证可静态分析、可被 go vet 检查。
2. **C1 熔断回退策略**：回退到备用 provider（`xunfei` → `funasr`）还是降级（直接返回错误 + 缓存最近一次结果）？
   - 建议：仅切到备选 provider，不返回缓存（避免幻觉）。
3. **C3 幂等键的存储**：内存 LRU vs Redis。
   - 建议：内存 LRU（默认 10k entries，TTL 24h）；不引入 DB。
4. **D3 v2 API 与 v1 的兼容窗口**：v1 进入 maintenance 多久？
   - 建议：v1 进入 maintenance 12 个月，v2 出来后 v1 只修阻塞 bug。
5. **F5 模块化路径**：单仓 + `replace` 指令 vs 独立仓库。
   - 建议：先在单仓内用 `app/evie/tool/go.mod` 拆出独立 module（与 `pkg/lexnorm` 同层），CI 同时验证；后续可平滑迁出。
6. **H1 配置热加载触发 reload 的方式**：SIGHUP vs HTTP `/admin/reload` vs fsnotify 自动。
   - 建议：三者并行（默认 fsnotify 自动；SIGHUP 强制；`/admin/reload` 用于控制面），均需鉴权。

---

## 6. 资源估算

| 阶段 | 人力 | 周期 | 主要交付 |
|---|---|---|---|
| Phase 6 | 1 人 | 2-3 周 | 模块化（SDK 占位）、CLI 雏形、结构化日志、pprof 开关、依赖扫描 |
| Phase 7 | 1-2 人 | 3-4 周 | 熔断、幂等、租户配额、fuzzy 优化（<2ms） |
| Phase 8 | 1-2 人 | 3-4 周 | SDK 稳定版、WebSocket/SSE、v2 API 路由 |
| Phase 9 | 2 人 | 4-6 周 | 多语言、QA CLI、英文 normalize、OIDC |

> 期间所有阶段都必须维持 `go test ./... && go test -race ./...` 全绿、`make bench` 基线不退化。

---

## 7. 度量（如何判断扩展成功）

- **采纳度**：`pkg/sdk` 周下载量、`examples` 仓库 star、`docs` 站点访问。
- **可观测性**：P95 延迟 / 错误率 / 熔断触发次数 / p99 audio 路径时延。
- **质量**：QA CLI 命中率 ≥ 99%（与历史 baseline 对比）；race detector / vet / vuln 扫描全绿。
- **运维**：MTTR < 30min（pprof + 审计日志 + 文档）。
- **安全**：漏洞 critical 数 0；secret 扫描 0 hit。

---

## 8. 需要的用户输入

1. **A2 插件形态** 选哪个？
2. **C1 熔断回退** 选哪个？
3. **C3 幂等键存储** 选哪个？
4. **D3 v2 兼容窗口** 多长？
5. **F5 模块化** 立刻拆还是后续？
6. **H1 热加载** 全部三选一是否可接受？
7. 是否同意把 `evie/tool` 升级为**独立 go module**（与 `pkg/lexnorm` 同级）？
8. 是否接受把 `configs/tenants.json` 与 `config.yaml` 全部纳入 git 跟踪模板（移除硬编码凭据已经完成）？

---

> 任何一项被否决，我可以替换为等价低风险项；本文件不是承诺，而是一份**等待你确认的提案**。
