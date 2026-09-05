# evie/tool — 独立轻量语音识别增强工具

> **状态**：✅ **M0–M9 全部交付**（proto + conf / Bearer auth / 词库同步 / 8 层增强 / ASR + 音频落盘 / 文档收口）
> **设计文档**：[docs/services/evie-platform/development/11-evie-tool独立轻量语音识别增强工具开发计划.md](../../../docs/services/evie-platform/development/11-evie-tool独立轻量语音识别增强工具开发计划.md)
> **错误码**：[14-evie-tool错误码文档.md](../../../docs/services/evie-platform/development/14-evie-tool错误码文档.md)
> **设计模式**：[12-evie-tool文本增强M6设计模式方案.md](../../../docs/services/evie-platform/development/12-evie-tool文本增强M6设计模式方案.md)

---

## 一、定位

evie/tool 是从 `app/evie/service` 抽取的**轻量化、零数据库、配置驱动**变体：

- **多 Provider ASR**：funasr（整段）+ xunfei（流式），按 conf 路由
- **8 层文本增强**：cleaning → filler → vocab_matching → alias_resolution → deterministic_replacement → phrase_standardization → pinyin_correction → fuzzy_matching → context_correction → llm_reserved
- **qua 用户/部门自动拼音化**：定时同步到 per-tenant 词库
- **Bearer Token 校验**：Redis 查 `oauth2_access_token:<token>`（共用 qua 的 Redis db）
- **音频本地落盘**：`upload/audio/<tenant>/<session>.<ext>`
- **健康检查**：`/health/live` + `/health/ready`

---

## 二、目录速览

```
app/evie/tool/
├── cmd/server/                 # main + wire + wire_gen
├── configs/                    # config.yaml + system.json（系统静态词条）
├── internal/
│   ├── conf/                   # 生成（conf.proto → conf.pb.go）
│   ├── biz/                    # 业务用例
│   │   ├── asr.go              # ASR usecase（同步 + 流式 + ring buffer + 音频落盘）
│   │   ├── enhancement.go      # 增强 usecase
│   │   ├── vocabulary.go       # per-tenant 词库构建器
│   │   ├── vocab_sync.go       # 词库后台同步器
│   │   └── vocab_normalizer.go # Q13 Normalizer（YAML 规则）
│   ├── data/                   # Redis / qua client / system_dict / health
│   │   ├── health.go           # M9 健康检查器
│   │   ├── providers.go        # ASR providers 装配
│   │   ├── qua_client.go       # opaque fetcher（Q13）
│   │   └── token_cache.go      # Bearer token 缓存
│   ├── server/                 # HTTP/gRPC transport + 健康检查 endpoint
│   └── service/                # transport ↔ biz
└── upload/audio/               # 运行时音频（git 忽略）
```

---

## 三、快速开始

```bash
cd backend-service/app/evie/tool

# 1. 生成 proto / 配置 / wire（每次依赖变更后跑）
make proto && make config && make wire

# 2. 启动（前台）
make run

# 3. 验证
curl -s http://127.0.0.1:8100/health/live
curl -s http://127.0.0.1:9100/evie/tool/v1/asr:recognize \
  -H "Authorization: Bearer <token>" -X POST -d '{"format":{"encoding":"wav"},"audioData":"aGVsbG8="}'

# 4. 测试
make test-race
```

---

## 五、HTTP 端点

| 路径 | 方法 | 鉴权 | 说明 |
|------|------|------|------|
| `/health/live` | GET | ❌ | 进程存活（永远 200） |
| `/health/ready` | GET | ❌ | 依赖就绪（Redis + qua + ASR providers） |
| `/evie/tool/v1/enhance` | POST | ✅ | 文本增强（同步） |
| `/evie/tool/v1/asr:recognize` | POST | ✅ | 整段识别 |
| `/evie/tool/v1/asr/stream` | gRPC | ✅ | 流式识别（双向流） |
| `/evie/tool/v1/asr/records` | GET | ✅ | 列出最近记录（分页） |
| `/evie/tool/v1/asr/records/:id` | GET | ✅ | 单条记录详情 |
| `/evie/tool/v1/asr/records/:id/audio` | GET | ✅ | 下载音频文件 |

---

## 六、配置要点

详见 `configs/config.yaml`。最常用开关：

| 字段 | 含义 | 默认 |
|---|---|---|
| `data.redis.addr` | qua 系统的 Redis 地址 | 127.0.0.1:6379 |
| `data.redis.token_key_prefix` | qua token 的 Redis key 前缀 | `oauth2_access_token:` |
| `qua.base_url` | qua HTTP 服务 | `http://127.0.0.1:48080` |
| `asr.default_batch_provider` | 整段识别首选 | funasr |
| `asr.default_stream_provider` | 流式识别首选 | xunfei |
| `asr.upload.audio_dir` | 音频落盘目录 | `upload/audio` |
| `asr.upload.retention_days` | 音频保留天数 | 7 |
| `tenant_vocab.sync_interval` | 租户词库同步周期 | 5m |
| `system_dict.hot_reload` | system.json 热加载开关 | true |
| `enhancement.pipeline` | 启用的 processor 列表（按顺序） | 见 config |

---

## 七、M0–M9 进度

| M | 状态 | 交付 |
|---|:---:|------|
| M0 | ✅ | proto / conf / wire / 骨架启动 |
| M1 | ✅ | pkg/asr Provider 装配（funasr + xunfei） |
| M2 | ✅ | Bearer Token 中间件 + ctx 注入 |
| M3 | ✅ | Q13 外部解耦：fetcher + adapter + Normalizer + wire |
| M4 | ✅ | 系统静态词条加载 + 热加载 |
| M5 | ✅ | 租户注册表 + 预加载 + 周期同步 |
| M6 | ✅ | 8 层文本增强 Pipeline（含 Snapshot/Status/Observer/Policy） |
| M7 | ✅ | **整段 + 流式识别 + 本地音频落盘 + ring buffer** |
| M8 | ⇄ | （与 M7 合并实现） |
| M9 | ✅ | **健康检查 endpoint + 错误码文档 + README 收口 + 13 个新测试** |

---

## 八、测试覆盖（69 cases）

| 层 | 测试数 | 文件 |
|---|---:|---|
| biz | 35 | `internal/biz/*_test.go` |
| data | 15 | `internal/data/*_test.go`（含 5 个 health 测试） |
| server | 16 | `internal/server/*_test.go`（含 5 个 E2E） |
| service | 10 | `internal/service/*_test.go` |

```bash
make test-race   # 全跑
```

---

## 九、依赖

- `github.com/redis/go-redis/v9`
- `github.com/fsnotify/fsnotify`（系统词条热加载）
- `github.com/mozillazg/go-pinyin`
- `github.com/go-kratos/kratos/v2`
- `pkg/asr`、`pkg/asr/audio`、`pkg/pinyin`、`pkg/auth/authn`、`pkg/health`（本仓）

**无 Ent / MySQL / SQLite / Casbin 依赖。**