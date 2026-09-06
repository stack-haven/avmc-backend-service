# evie/tool · 架构决策日志（执行计划阶段）

> 来源：`.agents/OPTIMIZATION_PLAN.md` 的 P0 阶段。
> 本文件记录为消除“文档宣称 vs 代码实际”根因分歧而做的关键决策。
> 每条决策列出：背景、选项、结论、影响范围。

---

## D1 · 后台词库同步的身份来源

**背景**：`docs/SERVICE_REQUIREMENTS.md` 宣称启动预加载 + 周期同步租户词库，但后台 ctx 不携带 AuthInfo，`Warmup/runOnce` 在生产配置下必然跳过。

**结论**：提供可配置的同步身份 Provider，**支持可选启用后台同步**。

- `qua.sync` 顶层新增可选配置：
  - `enabled`: bool，默认 `false`
  - `mode`: `static` / `redis`（默认 `redis`）
  - 当 `enabled=false`：服务退化为“仅请求级 lazy sync”，健康详情 `vocab_sync_mode=lazy_only`。
  - 当 `enabled=true`：根据 `mode` 选择 sync token 拉取所有 tenant：
    - `static`：从 `tenants.json` 的 `sync_token` 字段读取（多租户场景每个租户一个 token）。
    - `redis`：按 `qua.sync.token_key_prefix` + token 读取 AccessToken，作为跨租户同步凭据（qua 系统要求业务系统注入 token，简化方案）。
- `tenants.json` 字段：
  - `tenants`: [{ id, sync_token? }]

**影响**：
- 新增 `qua.sync` 配置（conf.proto 增加）。
- 修改 `VocabSyncer.runOnce` 遍历 registry tenant，使用对应 tenant 上下文。
- 健康详情输出 sync_mode。
- 不改变现有请求级 lazy sync 行为。

---

## D2 · 词库同步的租户语义

**背景**：`RawEntity.Source` 被错误地当作 tenant，`runOnce` 把 `"qua"` 注册为 tenant，导致后台同步不可能按真实租户遍历。

**结论**：
- `SyncTenant(ctx, tenantID)` 的 `tenantID` 必须来自 `TenantRegistry` 或 `AuthInfo`，**不来自 `RawEntity.Source`**。
- `runOnce` 仅遍历 `TenantRegistry.List()`；每个 tenant 用对应身份调 `Fetch`，再 Normalize 后 `UpdateTenant(tenantID, ...)`。
- 不再以 `RawEntity.Source` 分组注册 tenant。

**影响**：
- `VocabSyncer.runOnce` 重构；
- `VocabularySource` 接口保持不变（依旧 `Fetch(ctx)` 拉全部），由 `SyncTenant` 控制 tenant 上下文；
- Normalizer rules key 仍按 `source`（如 `qua`），不影响。

---

## D3 · ASR 记录/音频的权限粒度

**背景**：proto 注释 “本租户近 N 条识别记录”，但实现未做租户过滤。

**结论**：
- 最低要求：按 `tenantID` 隔离。
- 暂不按 `userID` 细化（如需可在 proto 增 `user_id` 查询参数）。
- 跨租户访问统一返回 `NotFound`，避免泄露存在性。

**影响**：
- `ASRUsecase` 增加 tenantID/userID 参数；
- `service` 从 auth 注入；
- 测试增加跨租户访问用例。

---

## D4 · ASR Records 存储结构

**背景**：当前 ring buffer 全局，O(n) 扫描 + 复制。

**结论**：保留 ring buffer（FIFO 1000 条），按 `tenantID` 索引查找；list 时只返回该 tenant。

- 数据结构：`recordsByID map[string]*ASRRecord` + `recordsByTenant map[string][]*ASRRecord`（保持插入顺序，>1000 淘汰最旧）。

**影响**：Phase 2 性能优化；本阶段先做租户隔离，索引优化留到 Phase 4。

---

## D5 · Health 真实化

**背景**：`/health/ready` 只看 ASR 注册的 provider 列表，不探测可达性。

**结论**：
- funasr：`HTTP GET <addr>/health`（若实现）或 简单 TCP/HTTP 探测。
- xunfei：不发起复杂握手；判断“配置存在即视为可路由”，但要在 details 里明确 `health=unknown`，避免假阳性。
- redis：保持 `Ping`。
- qua：保持 `HEAD baseURL`。
- 当任一关键依赖不可达时，ready 返回 503。

**影响**：pkg/asr 增加可选 `HealthCheck(ctx) error`；HealthChecker 调度。

---

## D6 · secret 与 config

**结论**：
- `configs/config.yaml` 中的真实凭据（redis password、xunfei app_id/api_key/api_secret）移除或改为占位。
- 引入 `configs/config.example.yaml`（示例模板）。
- Kratos config 支持 `env` source（在生产可叠加），示例文档说明。
- CI 增加 secret 扫描（可选）。

---

## D7 · validate middleware

**结论**：
- HTTP/gRPC 都接入 Kratos `validate.Validator()`（`contrib/middleware/validate`）。
- 让 proto 的 `buf.validate` 规则真正生效（10MB 音频、文本 min_len、session_id 模式等）。

---

## D8 · 路径安全

**结论**：
- `session_id` 只接受 `[A-Za-z0-9_-]{1,64}`；不合规拒绝。
- `tenant_id` 写入路径前规范化，拒绝 `..`、分隔符、绝对路径前缀。
- 读取 audio 前用 `filepath.Rel(audioDir, resolved)` 校验不越界。

---

## D9 · Pinyin 派生

**结论**：
- 实现 `derivePinyin` / `derivePinyinInitial`，使用 `backend-service/pkg/pinyin`。
- 同时为 qua 同步条目和 system.json 条目派生。
- lexnorm bridge 将其写入 `lexicon.Entry.Meta["pinyin"]`，与已有的 pinyin processor 兼容。

---

## D10 · Stream 生命周期

**结论**：
- service 层在启动 sender goroutine 前完成 stream provider 可用性校验；
- usecase 所有提前返回路径必须 `defer close(resultCh)`；
- sender goroutine 在错误或 ctx cancel 时立即退出；
- 避免永不接收的 provider goroutine 阻塞写入。