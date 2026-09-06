# evie/tool · 已实现能力与说明

> 本文件是 `app/evie/tool` 当前**真实可用能力**的唯一事实来源。
> 与 `docs/SERVICE_REQUIREMENTS.md` 的产品边界保持一致：零数据库、配置驱动、无 UI/无完整词库中心。

---

## 1. 协议面

| 端点 | 方法 | 鉴权 | 说明 |
|---|---|---|---|
| `/health/live` | GET | ❌ | 进程存活，永远 200 |
| `/health/ready` | GET | ❌ | 依赖就绪（Redis / qua / ASR / 词库同步模式） |
| `/metrics` | GET | ❌ | Prometheus 0.0.4 文本 |
| `/evie/tool/v1/enhance` | POST | ✅ | 纯文本增强 |
| `/evie/tool/v1/asr:recognize` | POST | ✅ | 整段识别 |
| `/evie/tool/v1/asr/records` | GET | ✅ | 本租户最近识别记录（分页） |
| `/evie/tool/v1/asr/records/{id}` | GET | ✅ | 单条记录详情（仅本租户） |
| `/evie/tool/v1/asr/records/{id}/audio` | GET | ✅ | 下载原始音频（仅本租户） |
| `evie.tool.v1.ASRService/StreamRecognize` | gRPC | ✅ | 流式识别（双向流） |

`/health/ready` 的 `details`：

```json
{
  "redis": true,
  "qua": true,
  "asr": true,
  "asr_providers": ["funasr", "xunfei"],
  "vocab_sync_mode": "background",
  "vocab_last_sync": "2026-09-05T10:11:12+08:00"
}
```

`/metrics` 暴露的指标（5 类）：

- `evie_http_requests_total{method,path,status}`
- `evie_http_request_duration_seconds{method,path}`（histogram）
- `evie_asr_requests_total{provider,kind,status}`
- `evie_asr_request_duration_seconds{provider,kind}`（histogram）
- `evie_vocab_sync_total{mode,status}`

---

## 2. ASR Provider

- `funasr`（整段 / 本地部署）— 通过 HTTP 调用
- `xunfei`（流式 / 讯飞云）— gRPC 双向流
- `mock`（集成测试 / CI）— 进程内 mock
- 路由策略：`conf.Asr.DefaultBatchProvider` 优先；该 provider 未启用时按 enabled 列表降级

---

## 3. 文本增强 Pipeline（8 层 lexnorm）

```
输入文本 → cleaning → filler → vocab_matching → alias_resolution
        → deterministic_replacement → phrase_standardization
        → pinyin_correction → fuzzy_matching → context_correction
```

- 业务 8 层与 lexnorm processor 的映射在 `internal/biz/lexnorm_engine.go` 维护
- 自定义 `fuzzy_vocab` processor：基于词库条目的编辑距离模糊匹配，PERSON 类阈值 0.65
- 拼音派生使用 `backend-service/pkg/pinyin`（非占位）

---

## 4. 词库

- **per-tenant 内存快照**：`recordsByID` + `recordsByTenant` 双索引
- **同步模式**：
  - `background`：`tenants.json` 中存在 `sync_token` 的租户走后台周期同步
  - `lazy_only`：未配置 token 的租户仅在请求路径按需同步
- **数据源**：
  - `qua`（生产）：HTTP + ctx-aware token / tenant header
  - `http`：任意 REST，FieldMapper 配置字段映射
  - `file`（demo / 离线）：本地 JSON
- **Q13 三层解耦**：`Source → RawEntity → Normalizer(YAML 规则) → NormalizedEntry → VocabularyBuilder`
- **Normalizer** 支持 dot-path、include_when 简单条件（`==` / `!=` / 真值）、Priority dot-path / 字面量
- **系统静态词条**：`configs/dictionaries/system.json`，`hot_reload=true` 时 fsnotify 监听

---

## 5. 认证

- **Bearer Token**：`Authorization: Bearer <token>`
- **Provider**：
  - `redis`（生产）：读 `oauth2_access_token:<token>`，支持过期校验
  - `jwt`（自签 HS256 / RS256）：支持 issuer / audience / 过期
  - `static`（demo / 离线）：配置文件 / 代码注入的 (token, identity) 对
- **抽象接口**：`pkg/credential.CallerIdentity` 与 `Provider`；`data.TokenLookup` 统一 HTTP 中间件入口
- **路径安全**：`session_id` / `tenant_id` 走白名单 `^[A-Za-z0-9_-]{1,64}$`；读音频前 `IsPathWithin` 校验

---

## 6. 音频落盘

- 路径：`upload/audio/<tenant>/<session>.<ext>`
- 租户 ID / session ID 双重白名单 + 路径越界检测
- 内存 ring buffer：默认 1000 条 / 全局上限，按 `CreatedAt` 倒序
- ring buffer 索引：`recordsByID`（O(1) GetRecord / GetRecordAudio）+ `recordsByTenant`（O(tenant) ListRecords）

---

## 7. 可观测性

- **结构化日志**：Kratos `log`，含 `module / tenant / session_id` 字段
- **指标**：内置 `internal/metrics`（零依赖 Prometheus 文本）
- **健康检查**：`/health/live` 永远 200；`/health/ready` 检查 Redis ping、qua HEAD、ASR provider capabilities、词库同步模式
- **审计**：词库同步状态 `vocab_last_sync` / `vocab_last_error` 暴露在 `details`

---

## 8. Demo 模式（零外部依赖）

- 触发：`EVIE_TOOL_DEMO=1`
- Token：使用 `conf.Credential.Static` 静态用户表
- 词库：使用 `conf.Vocabulary.File` 本地 JSON
- 启动命令：`make demo`（等价于 `EVIE_TOOL_DEMO=1 go run ./cmd/server -conf ./configs/config.demo.yaml`）
- 已验证：端到端真实录音 MP3 增强、ASR 同步识别（mock provider）、records 列表/详情/音频下载

---

## 9. 配置 & 部署

- 配置文件：`configs/config.yaml`（生产/联调）/ `configs/config.example.yaml`（公开模板，无凭据）/ `configs/config.demo.yaml`（demo 模式）
- 租户注册表：`configs/tenants.json`（`[{id, sync_token?}]`）
- 启动期配置校验：`internal/conf/Validate` 聚合检查 server / redis / qua / asr / pipeline / system_dict / tenant_registry
- 凭据注入：推荐使用环境变量（`${REDIS_PASSWORD}` / `${XUNFEI_API_KEY}` 等）

---

## 10. 质量门禁

- `gofmt -l internal pkg cmd` 0 行
- `go vet ./...` 仅遗留 `lexnorm.Span` 未使用 keyed fields 警告
- `go test ./...` 14 包全绿（含 `TestE2E_RealRecording` 真实 MP3 E2E）
- `go test -race ./...` 全绿
- `go test -bench=. -run=^$ ./internal/biz/ ./internal/biz/processor/` 提供基线：
  - `EnhancementUsecase.EnhanceText` ≈ 15 µs
  - `ASRUsecase.Recognize + ListRecords` ≈ 237 µs
  - `FuzzyVocabProcessor.Process`（1000 词库） ≈ 90 µs（Phase 7.1 优化后，原基线 ~9 ms；等长 Hamming 优化 + entry 预计算 rune 切片）
- CI：`.github/workflows/ci.yaml`（gofmt / vet / test / race / bench）
- Makefile：`make config / wire / test / race / bench / lint / cover / demo`

---

## 11. 已知遗留与下一阶段方向

- `make wire` 自动重新生成 `wire_gen.go`（当前为手动编辑）
- 拓展方向（v0.x → v1.x）见 `.agents/EXPANSION_PROPOSAL.md`
- 小步快跑执行计划见 `.agents/EXECUTION_PLAN.md`
