# 建议执行计划（小步快跑）

> 依据：`.agents/EXPANSION_PROPOSAL.md`、`.agents/CAPABILITIES.md`。
> 节奏：每个 step 1-3 天，PR 粒度，**每个 step 必须 `go test ./...` + `go test -race ./...` + `go vet ./...` 全绿** 才合并。
> 决策门：标注 🔑 的 step 在开始前需要你确认 EXPANSION_PROPOSAL 第 5 节的对应决策点。

---

## 0. 立即可做（今天内，0.5 天）

| Step | 内容 | 产出 | 验证 |
|---|---|---|---|
| **0.1** | 把 `fuzzy_vocab_test.go` 中 `lexnorm.Span{...}` 改为 keyed 字段，消除 `go vet` warning | 1 文件 | `go vet ./...` 0 warning |
| **0.2** | `make wire` 重新生成 `wire_gen.go` 并与手编辑版本对比，确保 `Credential`/`Vocabulary` 已纳入 wire providers | `wire_gen.go` 自动重生成 | `make wire` 无 diff 或仅必要 diff |
| **0.3** | `make cover` 本地跑覆盖率，输出 `coverage.html`，记录基线数字 | `coverage.html` | 文件生成成功 |

---

## 1. Phase 6 · 治理基线（1-2 周）

> 目标：仓库可读性 / 运维可定位 / 合规基线。

| Step | 内容 | 关键依赖 | 验证 |
|---|---|---|---|
| **6.1** | **A4 CLI**：`cmd/evie-toolctl` 子命令骨架 + `validate-config`（复用 `internal/conf.Validate`）+ `dump-vocab` | 6.1 | `make cli && ./bin/evie-toolctl validate-config -f configs/config.demo.yaml` 返回 0 |
| **6.2** | **B1 结构化日志**：把 Kratos log 包装为 JSON 输出（默认文本），字段：`ts/level/service/version/tenant/request_id/latency_ms/status/path/method`；通过 `EVIE_TOOL_LOG_FORMAT=json` 切换 | — | demo 启动后日志为 JSON；通过 `jq` 可解析 |
| **6.3** | **B4 pprof**：仅 `EVIE_TOOL_PPROF=1` 时挂载 `net/http/pprof`；提供 `internal/server/pprof.go` 注册 | 6.2 | `curl /debug/pprof/heap` 200（开关开），404（开关关） |
| **6.4** | **E4 日志脱敏**：`Authorization` / `X-API-Key` 等 header 默认 `***`；可通过 `EVIE_TOOL_LOG_REDACT=off` 关闭（仅调试） | 6.2 | 测试中写 `Authorization: Bearer secret`，日志输出为 `Bearer ***` |
| **6.5** | **A5 API 文档站骨架**：mkdocs + `buf generate` OpenAPI 落盘到 `docs/api/`；提供 `make docs` | 6.1 | `make docs` 产出 `docs/api/index.html` |
| **6.6** | **F5 模块化（前置）**：在 `app/evie/tool/go.mod` 创建独立 module（`github.com/stack-haven/evie-tool`），与 `pkg/lexnorm` 同级；`replace` 指令指向本地路径；CI 同时跑根 module + 子 module | 🔑 需你确认 | `go test ./...` 在两个 module 下都通过 |

---

## 2. Phase 7 · 稳定性强化（2-3 周）

> 目标：消除 P0/P1 风险（fuzzy 慢 / ASR 单点 / 租户打爆）。

| Step | 内容 | 关键依赖 | 验证 |
|---|---|---|---|
| **7.1** | **C5 fuzzy 优化**：长度桶内 `LevenshteinDistance` 下界剪枝 + early-exit；保留 baseline benchmark | 6.6 | `BenchmarkFuzzyVocabProcessor_Process` 1000 词库 < 2 ms |
| **7.2** | **C2 超时分层**：`conf.Asr.Timeouts.{Recognize,Stream}` + `Enhancement.Timeout`；`lexnorm` ctx timeout 联动 | — | 单测：provider 延迟 > timeout 时返回 504 |
| **7.3** | **C1 熔断**（🔑 决策 1+2）：`pkg/aegis` 引入（已是依赖）；`ASRUsecase.Recognize` 包裹熔断器；备选 provider 切换；暴露 `evie_asr_circuit_breaker_state` gauge | 7.2 | chaos test：mock provider 50% 错误，验证自动切到备选 |
| **7.4** | **C3 幂等键**（🔑 决策 3）：`Idempotency-Key` header 支持；内存 LRU（默认 10k，TTL 24h）；`evie_idempotency_hits_total` counter | — | 同 key 两次请求，第二次命中缓存（response hash 相同） |
| **7.5** | **H3 租户配额**：每 tenant 每分钟 ASR N 次（默认 60）；超限 429 + 详细 `Retry-After`；`evie_tenant_quota_exhausted_total{tenant}` counter | 7.4 | 测试：A 租户打满后 B 租户仍可正常 |
| **7.6** | **E3 租户限流**（与 7.5 共用 Redis token bucket，但限流与配额分层） | 7.5 | 同上 |

---

## 3. Phase 8 · 协议扩展（2-3 周）

> 目标：让工具被更多场景使用（嵌入 / 流式 / 版本兼容）。

| Step | 内容 | 关键依赖 | 验证 |
|---|---|---|---|
| **8.1** | **A1 Go SDK 形态**（🔑 决策 5）：把 `pkg/credential` / `pkg/source` / `biz.EnhancementUsecase` 暴露为稳定 API；`pkg/sdk/evie` 入口；`SemVer` v0.x → v1.0 | 6.6 | 写一个 `examples/embed` 单独 module 引用 SDK，build 通过 |
| **8.2** | **A2 插件式 Processor**（🔑 决策 1）：`pkg/processor/registry` 编译期注册；示例 `examples/custom-processor` | 8.1 | 注册自定义 processor 后 `conf.enhancement.pipeline` 能引用 |
| **8.3** | **D1 WebSocket 流式 ASR**：`/evie/tool/v1/asr/stream/ws`（RFC 6455），复用现有 `StreamRecognize` 后端；JS / Python 样例 | — | 用 `wscat` 发送 PCM 帧，收到 partial/final 帧 |
| **8.4** | **D2 SSE 增强进度**：`POST /evie/tool/v1/enhance/stream` 返回 `text/event-stream`，每层输出增量（中间过程） | — | `curl -N` 看到多个 `event: step` 帧 |
| **8.5** | **D3 v2 API 路由并行**（🔑 决策 4）：`/evie/tool/v2/enhance` 等新端点；v1 进入 maintenance 标记 | 8.1 | v1 + v2 同时跑，proto 文件标注 deprecated |

---

## 4. Phase 9 · 智能 / 语言扩展（3-4 周）

> 目标：走向产品层级（多语言 / 可评估 / 可组合）。

| Step | 内容 | 关键依赖 | 验证 |
|---|---|---|---|
| **9.1** | **D5 Processor 组合 DSL**：`pipeline.steps: [{name, type, opts, when}]`；`when` 支持 `result.confidence < 0.8` 等简单条件 | 8.2 | 配置 `when` 条件后行为符合预期 |
| **9.2** | **G1 英文 / 数字 normalize**：新增 `english_normalize` processor；`cleaning` 适配英文标点 / 数字 | — | 英文文本也能获得合理 enhanced text |
| **9.3** | **G2 多语言 Romanizer**（🔑）：抽象 `pkg/romanizer` interface；默认 pinyin 实现；提供 romaji 占位 | 9.2 | 接口可替换；单元测试覆盖 |
| **9.4** | **C6 QA 评估 CLI**：`evie-toolctl eval -d ./testdata/eval` 读取 (audio, expected_text) fixtures，输出命中率 / WER / p95 延迟；CI 接入 | 7.1 | 在 CI 中跑 eval，输出基线报告 |
| **9.5** | **E2 OIDC Provider**（🔑 决策 5）：`pkg/credential/oidc` 通过 discovery + JWKS 验签 | 8.1 | 注入 OIDC discovery URL，单测覆盖正常 + 过期 + 错 issuer |
| **9.6** | **A3 示例与教程仓库**：单仓 `examples/`，覆盖 SDK 使用 / 自定义 Processor / 自定义 Source | 8.1-8.3 | 每个 example 独立 `go.mod`，CI 跑通 |

---

## 5. 横向可持续工作（不依赖上述阶段）

| 项 | 周期 | 备注 |
|---|---|---|
| `make wire` 自动化（CI step） | 0.5 天 | 加在 `.github/workflows/ci.yaml` |
| `govulncheck` 接入 CI | 0.5 天 | 阻断 high/critical |
| `trivy` 镜像扫描 | 1 天 | 与 F1 多架构镜像同步 |
| 文档站（mkdocs）上线 | 1-2 天 | 与 6.5 接续 |

---

## 6. 决策清单（开始前必须敲定）

> 详见 `.agents/EXPANSION_PROPOSAL.md` 第 5 节。

1. **A2 插件 Processor 形态**：编译期 import（推荐） / 运行期 .so
2. **C1 熔断回退策略**：切备选 provider（推荐） / 返回缓存 / 二者并存
3. **C3 幂等键存储**：内存 LRU + 24h TTL（推荐） / Redis
4. **D3 v2 兼容窗口**：v1 maintenance 12 个月（推荐）/ 6 个月 / 24 个月
5. **F5 模块化时机**：现在拆（推荐） / 后续
6. **H1 热加载触发**：SIGHUP + HTTP `/admin/reload` + fsnotify 三者并行（推荐）/ 子集
7. **evie/tool 独立 go module**：同意（推荐） / 暂缓
8. **config 模板纳入 git**：同意（推荐） / 仅保留 example 不入仓

---

## 7. 节奏建议

- **每日**：一个 step，1 个 PR，2-3 个 commit
- **每周**：周五汇总本周 step 进度，更新 `.agents/CAPABILITIES.md` 的"已知遗留"段
- **每月**：复盘决策点，决定是否进入下一 Phase
- **每个 Phase 结束**：更新 `.agents/OPTIMIZATION_PLAN.md` 与 `.agents/EXPANSION_PROPOSAL.md` 状态；触发 release tag

---

## 8. 推荐先做的 3 步（本周可交付）

按"价值/成本"最高优先：

1. **0.1** 清理 `go vet` warning（10 分钟，立即可合）
2. **6.1** CLI 骨架 + `validate-config`（半天，仓库专业度立刻提升）
3. **7.1** `fuzzy_vocab` 启发式剪枝（1-2 天，性能基线从 9ms 拉到 <2ms，可在 benchmark 报告里直接体现）

> 这 3 步都不依赖任何决策点（🔑），可以无门槛推进。
