# evie/tool — 独立轻量语音识别增强工具

> evie/tool 是 `app/evie/service` 的**轻量化、零数据库、配置驱动**变体：
> 多 Provider ASR + 8 层文本增强 + per-tenant 词库 + Bearer Token 认证。
> 已通过内部 `app/evie/tool` 工作区持续迭代；本文档是当前真实能力的唯一入口。

| 项 | 现状 |
|---|---|
| 服务成熟度 | v1 生产就绪（多租户隔离、token 过期、路径安全、配置校验、可观测性） |
| 数据库 | 无（内存 + 文件 + Redis 仅用于 token 缓存） |
| 默认端口 | HTTP `8110` / gRPC `9110` |
| License | Apache-2.0 |

---

## 1. 能力一览

- **多 Provider ASR**
  - `funasr`（整段 / 本地部署）
  - `xunfei`（流式 / 讯飞云）
  - `mock`（集成测试 / CI）
- **8 层文本增强 Pipeline**（基于 `pkg/lexnorm`）：cleaning → filler → vocab_matching → alias_resolution → deterministic_replacement → phrase_standardization → pinyin_correction → fuzzy_matching → context_correction
- **per-tenant 词库**：qua / HTTP / File 数据源；启动预加载 + 请求级 lazy 同步 + 可选后台周期同步（仅当 `tenants.json` 配置 `sync_token`）。
- **Bearer Token 认证**：默认 Redis 读 `oauth2_access_token:<token>`，过期拒绝。
- **音频本地落盘**：`upload/audio/<tenant>/<session>.<ext>`，租户隔离 + 路径白名单。
- **健康检查**：`/health/live`、`/health/ready`、`/metrics`（Prometheus 文本）。
- **可观测性**：进程级指标（HTTP / ASR / 词库同步）+ 启动期配置校验 + 结构化日志。

明确**不做**：数据库、管理后台 UI、完整词库中心、OAuth 签发、CDC/webhook、云存储。

---

## 2. 目录结构

```text
app/evie/tool/
├── .agents/                # Agent 规则、审计、优化计划、决策日志（本机）
├── cmd/server/             # main + wire + wire_gen
├── configs/
│   ├── config.yaml        # 生产/联调配置
│   ├── config.example.yaml# 公开模板（无凭据）
│   ├── tenants.example.json# 租户注册表示例
│   └── dictionaries/
│       ├── system.json    # 系统静态词条
│       └── demo_vocab.json# Demo 用文件词源
├── internal/
│   ├── conf/              # conf.proto + 校验
│   ├── biz/               # ASR / Enhancement / 词库 / 同步
│   ├── data/              # Redis / qua / ASR providers / Health
│   ├── server/            # HTTP/gRPC transport + 中间件
│   ├── service/           # transport ↔ biz
│   └── metrics/           # Prometheus 文本指标
├── pkg/
│   ├── credential/        # 认证 Provider 抽象（redis / jwt / static）
│   └── source/            # 词库 Source 抽象（http / file / qua）
├── testdata/              # 真实录音 / 调试工具
├── upload/audio/          # 运行时音频（git 忽略）
├── Makefile
├── Dockerfile
└── README.md
```

---

## 3. 快速开始

### 3.1 前置依赖

- Go 1.25+
- 本地 Redis（qua 共享，存放 `oauth2_access_token:<token>`）
- 可访问的 qua HTTP 服务（或自建 mock）
- 可选：funasr / 讯飞凭证

### 3.2 准备配置

```bash
# 1. 复制模板
cp configs/config.example.yaml configs/config.yaml
cp configs/tenants.example.json configs/tenants.json

# 2. 填入真实 Redis / qua / 讯飞地址或环境变量
#    （生产建议使用环境变量 + secret 注入，不要把凭据提交到仓库）
```

### 3.3 启动

```bash
cd backend-service/app/evie/tool

# 生成配置 + wire（仅在依赖变更时需要）
make config
make wire

# 前台启动
make run

# 验证健康检查
curl -s http://127.0.0.1:8110/health/live
curl -s http://127.0.0.1:8110/health/ready | jq .

# 拉取指标
curl -s http://127.0.0.1:8110/metrics
```

### 3.4 端到端调用

```bash
TOKEN=<oauth2-access-token>
curl -X POST http://127.0.0.1:8110/evie/tool/v1/asr:recognize \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
        "format": {"encoding": "wav", "sampleRate": 16000, "bitDepth": 16},
        "audioData": "<base64-encoded-wav>",
        "enableEnhancement": true
      }'
```

### 3.5 Demo 模式（零外部依赖）

为了快速体验文本增强能力，项目内置 **demo 模式**：不连 Redis / qua / ASR Provider，
使用 `conf.Credential.Static` 做本地 Bearer 校验、`conf.Vocabulary.File` 做本地词库加载。

```bash
make demo    # 等价于：EVIE_TOOL_DEMO=1 go run ./cmd/server -conf ./configs/config.demo.yaml
```

启动后可使用 `demo-token` 或 `admin-token` 调用 `/evie/tool/v1/enhance`：

```bash
curl -s -X POST http://127.0.0.1:8110/evie/tool/v1/enhance \
  -H "Authorization: Bearer demo-token" \
  -H "Content-Type: application/json" \
  -d '{"text":"金种子是新产品"}'
# => {"originalText":"金种子是新产品","enhancedText":"金种籽是新产品",...}
```

> demo 模式不会注册 ASRService；ASR 端点返回 404。生产环境请使用 `configs/config.yaml` 并设置真实 Redis / qua 凭证。

---

## 4. HTTP / gRPC 端点

| 路径 | 方法 | 鉴权 | 说明 |
|---|---|---|---|
| `/health/live` | GET | ❌ | 进程存活（永远 200） |
| `/health/ready` | GET | ❌ | 依赖就绪（Redis + qua + ASR + 词库同步模式） |
| `/metrics` | GET | ❌ | Prometheus 文本格式指标 |
| `/evie/tool/v1/enhance` | POST | ✅ | 纯文本增强（同步） |
| `/evie/tool/v1/asr:recognize` | POST | ✅ | 整段识别 |
| `/evie/tool/v1/asr/records` | GET | ✅ | 本租户最近识别记录（分页） |
| `/evie/tool/v1/asr/records/{id}` | GET | ✅ | 单条记录详情（仅本租户） |
| `/evie/tool/v1/asr/records/{id}/audio` | GET | ✅ | 下载原始音频（仅本租户） |
| `evie.tool.v1.ASRService/StreamRecognize` | gRPC | ✅ | 流式识别（双向流） |

> 所有业务端点都要求 `Authorization: Bearer <token>`；token 过期会被拒绝。

---

## 5. 配置参考

`configs/config.example.yaml` 为完整模板；下面列出关键开关。

| 字段 | 含义 | 默认 |
|---|---|---|
| `server.http.addr` | HTTP 监听地址 | `0.0.0.0:8110` |
| `server.grpc.addr` | gRPC 监听地址 | `0.0.0.0:9110` |
| `data.redis.addr` | qua 共享 Redis | `127.0.0.1:6379` |
| `data.redis.token_key_prefix` | Token key 前缀 | `oauth2_access_token:` |
| `qua.base_url` | qua HTTP 服务 | — |
| `qua.endpoints.list_users` | 用户列表端点 | `/admin-api/qua/member-extended/page` |
| `qua.endpoints.list_depts` | 部门列表端点 | `/admin-api/system/dept/list` |
| `asr.default_batch_provider` | 整段识别首选 | `funasr` |
| `asr.default_stream_provider` | 流式识别首选 | `xunfei` |
| `asr.upload.audio_dir` | 音频落盘根目录 | `./upload/audio` |
| `enhancement.pipeline` | 启用的 processor 列表 | `cleaning / filler / …` |
| `system_dict.path` | 系统词条文件 | `./configs/dictionaries/system.json` |
| `system_dict.hot_reload` | fsnotify 热加载 | `true` |
| `tenant_registry.path` | 租户注册表 | `./configs/tenants.json` |
| `vocab_rules` | 外部词源 → 词条 字段映射 | 详见 `config.example.yaml` |

### 5.1 凭据注入

生产部署**不要**把真实密码、API Key 写进 `configs/config.yaml`。推荐：

- 借助部署系统的环境变量注入（或 Kratos `env` source 覆盖）。
- 在 `config.example.yaml` 中保留占位符 `${REDIS_PASSWORD}`、`${XUNFEI_API_KEY}` 等。

### 5.2 词库同步模式

- 启动时读取 `tenant_registry.path`（JSON 数组）。
- 对每条 `{id, sync_token}`：
  - 有 `sync_token` → 后台周期同步（每 `tenant_vocab.sync_interval`）。
  - 无 `sync_token` → 仅请求级 lazy 同步（首次访问某 tenant 触发 5s 超时同步）。
- `/health/ready` 的 `details.vocab_sync_mode` 输出 `background` / `lazy_only`，便于运维确认实际行为。

---

## 6. 安全模型

| 关注点 | 当前实现 |
|---|---|
| Token 过期 | Redis/Static Provider 检查 `ExpiresAt`，过期返回 401 |
| 租户数据隔离 | ASR records/audio 按 `tenantID` 过滤；跨租户访问返回 404 |
| 文件路径安全 | `session_id` / `tenant_id` 走白名单 `^[A-Za-z0-9_-]{1,64}$`；读音频前 `IsPathWithin` 校验 |
| 凭据泄露 | `config.yaml` 已移除真实密码 / 讯飞密钥；公开模板以 `${VAR}` 占位 |
| 请求体大小 | proto `buf.validate` + Kratos `ProtoValidate` 中间件强制 10MB 音频上限 |
| 进程外传输 | HTTP/gRPC 对外需配 TLS（按部署要求） |

---

## 7. 可观测性

`/metrics` 暴露（Prometheus 0.0.4 文本）：

| 指标 | 标签 | 类型 |
|---|---|---|
| `evie_http_requests_total` | `method`, `path`, `status` | counter |
| `evie_http_request_duration_seconds` | `method`, `path` | histogram |
| `evie_asr_requests_total` | `provider`, `kind`, `status` | counter |
| `evie_asr_request_duration_seconds` | `provider`, `kind` | histogram |
| `evie_vocab_sync_total` | `mode`, `status` | counter |

`/health/ready` 输出的 `details`：

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

---

## 8. 验证命令

```bash
make test           # 全部单元/集成测试
make test-race      # 开启 race detector
make bench          # 性能基线（EnhanceText / ASR / FuzzyVocab）
make lint           # gofmt + go vet
make config         # 重新生成 internal/conf/conf.pb.go
make wire           # 重新生成 cmd/server/wire_gen.go
```

---

## 9. 开发与发布

- 平台子仓库：`backend-service`（独立 git 子仓）。
- 本目录使用 Conventional Commits：`feat / fix / refactor / docs / test / chore`。
- CI：`.github/workflows/ci.yaml`（gofmt / go vet / go test -race / bench / coverage）。

---

## 10. 相关文档

- 完整服务需求与决策：`docs/SERVICE_REQUIREMENTS.md`
- 代码审计与未完成项：`.agents/AUDIT.md`
- 优化增强计划：`.agents/OPTIMIZATION_PLAN.md`
- 关键架构决策：`.agents/DECISIONS.md`
- Agent 入口：`.agents/AGENTS.md`

---

## 11. 许可证

Apache-2.0。
