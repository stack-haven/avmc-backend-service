# evie/tool · 代码与文档一致性审计

> 审计方式：以 `docs/SERVICE_REQUIREMENTS.md`、`README.md` 为基准，反向核对当前代码。
> 结论不是“文档完美”，而是“主干可运行、但文档对完成度的描述高于实际代码状态”。

---

## 1. 总体结论

- ✅ 分层主链路存在：`proto → api(生成) → service → biz → data → wire` 可追踪。
- ✅ 核心 happy path 有测试覆盖：enhance、ASR recognize、records、health。
- ❌ 文档宣称“M0–M9 全部交付 + v1 生产就绪”与实际代码不完全一致。
- ❌ 代码中仍存在明显占位/未完成：TenantRegistry 不读文件、拼音派生返回空串、demo 配置未接线、后台定时同步实际不会跑。
- ❌ 安全/多租户隔离存在阻断级问题，当前不宜视为生产就绪。

---

## 2. 文档与代码一致性核对

### 2.1 基本一致的部分

| 项目 | 结果 |
|---|---|
| 服务分层 service → biz → data | ✅ 基本一致 |
| Wire 生成、ProviderSet 注册 | ✅ 可编译可测试 |
| HTTP 路由 `/evie/tool/v1/enhance`、`/asr:recognize`、records | ✅ 与 proto 一致 |
| gRPC 双向流方法存在 | ✅ `ASRService.StreamRecognize` |
| /health/live、/health/ready 注册 | ✅ 有实现与测试 |
| Bearer Token Redis 默认实现 | ✅ 生产 wiring 走 Redis token cache |
| pkg/credential、pkg/source 服务内化 | ✅ 包路径与文档一致 |
| Q13 RawEntity 不透明、Normalizer 规则化 | ✅ 设计基本一致 |

### 2.2 明显不一致/未完成

| # | 文档说法 | 代码实际情况 |
|---|---|---|
| 1 | README 快速开始使用 8100/9100 | 实际配置和 Makefile 使用 8110/9110 |
| 2 | 启动命令含 `make proto` | 当前 Makefile 不存在 `proto` target |
| 3 | demo 配置可 e2e（StaticProvider + FileSource） | main/wire 只接 Redis/qua；`conf.proto` 没有 `credential`/`vocabulary` 配置段；`make demo` 只 echo |
| 4 | 启动预加载 + 周期同步租户词库 | 后台 ctx 无 AuthInfo，Warmup/runOnce 会跳过；实际主要靠请求 lazy sync |
| 5 | tenant_registry 从文件加载租户 | `NewTenantRegistry` 注释明确“不读文件”，tenants map 为空 |
| 6 | qua 用户/部门自动拼音化后注入词库 | `derivePinyin/derivePinyinInitial` 返回空字符串，占位未实现 |
| 7 | `/health/ready` 检查 ASR providers | 只取注册 provider 并调 `Capabilities()`，不探测远端可达性 |
| 8 | ListRecords/GetRecordAudio 等记录属本租户 | 实现未按 tenant/user 过滤，任意已认证租户可访问全局 ring buffer 中其它租户记录 |
| 9 | proto 有 10MB 音频、text min_len 等校验 | server 未挂 `validate` 中间件，生成的 pb.validate 未被调用 |
| 10 | Bearer Token 校验含过期语义 | Redis/Static Provider 不检查 `ExpiresAt`；仅 JWT 检查 |
| 11 | 系统静态词条自动派生拼音/首字母 | `buildSystemSnapshot` 未填充 `Pinyin/PinyinInitial` |
| 12 | README/文档多处声称 P1–P6/B1–B4 已完成 | 代码仍留 M5/M9 占位注释、简化实现，不能按“全部收口”理解 |

---

## 3. 完整实现路径清晰性

### 3.1 清晰的路径

```text
HTTP/gRPC
→ internal/service      proto 转换、取 AuthInfo
→ internal/biz          ASR/Enhancement/Vocab usecase
→ internal/data         Redis/Qua/ASR providers
→ pkg/credential        Bearer Provider 抽象
→ pkg/source            词库数据源抽象
```

代码分层命名清楚，新增业务模块时容易定位。

### 3.2 不清晰/断链点

1. **租户词库同步语义混乱**
   - `RawEntity.Source` 被当作“来源名”，不是“租户”。
   - `VocabSyncer.runOnce` 按 `Source` 分组注册 tenant，会把 `"qua"` 当成租户。
   - 真正 per-tenant 同步只能靠请求 lazy sync 携带 AuthInfo 后按 tenantID 写入。
   - 文档里“定时全租户同步”的实现路径不成立。

2. **demo 配置路径未打通**
   - `config.demo.yaml` 有 `credential.static` 和 `vocabulary.file`，但配置模型与 wire 均未消费。

3. **Stream 错误路径有死锁风险**
   - service 启动 result sender goroutine 后再调 usecase；若 usecase 因缺少 stream provider 提前返回，`resultCh` 不关闭，`sendDone` 永久阻塞。

4. **校验链路未打通**
   - proto 校验规则存在，但 HTTP/gRPC middleware 未加入 validator。

5. **可观测状态不完整**
   - HealthChecker 的 ASR 检查是“能力列表”而非“健康探测”。

---

## 4. 已完成功能评分

| 维度 | 分数（0–10） | 依据 |
|---|---|---|
| 正确性 | 6 | 主链路 happy path 可用；但租户同步、拼音、demo 配置、校验等存在未完成/偏差 |
| 稳定性 | 5 | race 测试通过，具备 recover/timeout/fallback；但 stream 缺 provider 可能死锁、后台同步实际停摆、health 可能假阳性 |
| 安全性 | 4 | 记录/音频无租户隔离、Redis/Static token 不校验过期、validate 未启用、session_id/tenant_id 路径未净化、config 中硬编码凭据 |
| 效率 | 6 | 内存 ring buffer 和 lexnorm 分层合理；但缺 benchmark，fuzzy_vocab 在长文本/大词库下存在无基准的暴力扫描风险，后台同步重复 fetch |
| **整体生产就绪度** | **5** | 可作为开发/联调版本，不能按文档宣称的 v1 生产就绪处理 |

---

## 5. 建议后续优先级

1. 修复租户级 ASR records/audio 隔离。
2. Redis/Static credential 增加过期校验；server 接入 validate middleware。
3. 明确词库同步模型：后台无 AuthInfo 应使用 service account 或取消“定时同步”宣称，至少修正 Warmup/runOnce。
4. 补全拼音派生或从文档/配置移除相关承诺。
5. 完成 demo 模式 wiring 或从文档移除。
6. 修复 stream 提前返回时关闭 resultCh/sendDone 的问题。
7. 清理 README/启动命令/端口等文档偏差。

---

> 本文件由代码审计生成，后续迭代后应刷新“2.2 不一致清单”和“评分”表。

## 6. 修复进度（随迭代刷新）

| 项 | 状态 | 关键改动 | 验证 |
|---|---|---|---|
| ASR records/audio 租户隔离 | ✅ | recordsByID + recordsByTenant 双索引；service 传 tenantID | 单测 `TestASRUsecase_TenantIsolation` |
| Token 过期校验（redis/static） | ✅ | 解析 ExpiresAt 后判 `time.Now().After` | 手工 `go test ./...`；token 过期返回 401 |
| validate 中间件 | ✅ | HTTP/gRPC 接入 `kvalidate.ProtoValidate()` | `go build` |
| 文件路径安全 | ✅ | `SanitizeTenantID/SessionID` + `IsPathWithin` | `TestSanitizeSessionID / TestIsPathWithin` + 单测 `TestASRUsecase_InvalidSessionID` |
| config 凭据清理 | ✅ | 真实密码/讯飞密钥改占位 + `configs/config.example.yaml` | `grep` 验证 |
| Stream 关闭安全 | ✅ | `resultChClosed` + defer | 全量测试 |
| 拼音派生补全 | ✅ | `pkg/pinyin` 替代占位空串 | 单测 + 现有用例 |
| 词库同步模型 D1/D2 | ✅ | `TenantRegistry` 读 `tenants.json`；`runOnce/Warmup` 按 token 逐租户；`SyncMode` 上报 | 现有用例不依赖 | 
| 同步模式 health | ✅ | `HealthChecker.SetSyncMode` + `Details.vocab_sync_mode` | health_test 既存 | 
| 跨租户分片 D4 | ✅ | 双索引 + `TenantRecordCount` | `TestASRUsecase_TenantIsolation` |
| 启动配置校验 / metrics / benchmark | ✅ / ✅ | config.Validate + Prometheus 文本指标 / `/metrics`；bench 基线（EnhanceText ≈15µs，ASR Append+List ≈237µs，fuzzy ≈9ms/1000词库）已落地，`make bench` 可重跑 | `go test -bench` |
| README 文档收口 | ✅ | 重写为开源产品视角，补充端点/配置/安全/指标/开发入口 | `README.md` |
| Makefile 补齐 | ✅ | 补充 `proto` / `wire` / `test` / `cover` 目标，保留 `bench` / `lint` | `Makefile` |
| CI 入口 | ✅ | `.github/workflows/ci.yaml`（gofmt / vet / test / race / bench） | 需在仓库根启用 |
| Demo 零依赖 wiring | ✅ | `conf.Credential` / `conf.Vocabulary` schema + `data.NewStaticTokenCache` / `data.NewFileVocabularySource` + `server.TokenLookup` 抽象 + `cmd/server/runDemoApp`（`EVIE_TOOL_DEMO=1` 触发，跳过 Redis/qua） + `make demo` 启动脚本。`curl /evie/tool/v1/enhance` 验证 demo-token 可完成文本增强 | `make demo` 端到端测试 |
