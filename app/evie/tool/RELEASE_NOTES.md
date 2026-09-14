# evie/tool v1.7.0 — Release Notes

**发布日期**: 2026-09-14
**Commit**: fa5eb72 (最新)
**服务**: evie/tool — 独立轻量 ASR + 文本规范化服务
**端口**: HTTP 8110 / gRPC 9110
**变更类型**: Major (新增独立服务) + 多个修复

---

## 🎯 核心能力

- **多 Provider ASR**: funasr / xunfei / mock
- **8 层文本规范化 Pipeline**（基于 `pkg/lexnorm`）:
  cleaning → filler → vocab_matching → alias_resolution →
  deterministic_replacement → phrase_standardization →
  pinyin_correction → fuzzy_matching → context_correction
- **per-tenant 词库** + qua HTTP 数据源
- **lazy sync + TTL**（5min）— 按需加载，无后台定时器
- **Bearer Token 认证**: Redis `oauth2_access_token:<token>`
- **音频本地存储**: `upload/audio/<tenant>/<session>.<ext>`
- **可观测性**: `/health/{live,ready}` + `/metrics` (Prometheus)

---

## 📊 真实录音回归（生产标准）

测试用例: 晨会录音 121 秒真实录音

| 指标 | v1.3 (修复前) | v1.7 (发布) | 提升 |
|---|---|---|---|
| total changes | 73 | 85 | +12 |
| FUZZY replace | 1 | **11** | **+10** |
| FUZZY suggest | 8 | 10 | +2 |
| Precision (replace) | 100% | **100%** | 保持 |
| Recall (用户 13 个案例) | 0/13 (0%) | **9/13 (69.2%)** | **+69.2%** |

---

## ✨ v1.6 系统性减法（690 行 → 221 行）

**核心理念**: 后台 sync 是浪费，删除后台、纯按需 + TTL。

删除:
- TenantRegistry（260 行）
- Warmup / Run / ticker（后台 sync）
- adminToken（跨租户凭据）
- SyncMode 配置
- configs/tenants.json

保留:
- lazy sync（EnsureTenant + AttachLazySync）
- TTL（vocabSnapshotTTL=5min）
- AuthContext（请求路径 token 提取）

---

## ✨ v1.7 系统性减法（再删 ~1604 行）

删除 sync/source 关联死代码:
- `internal/data/health.go`: 移除 SetSyncState/SetSyncMode/lastSync/lastError/mu
- `internal/data/source_registry.go`: 删除（VocabularySourceRegistry 0 引用）
- `internal/data/file_vocab_source.go`: 删除
- `internal/data/demo.go`: 删除
- `internal/data/wire_providers.go`: 删除
- `pkg/credential/{jwt,middleware,adapter}`: 整个包删除
- `pkg/source/file`: 整个包删除

---

## 🐛 Bug 修复（4 个 P-FIX）

### P-FIX v1: 中间件顺序安全修复
- **问题**: 原顺序 `recovery → validate → metrics → auth`，无 token + 无效 body 返回 400 而非 401，泄露验证规则
- **修复**: 改为 `recovery → metrics → auth → validate`（auth 先于 validate）

### P-FIX v2: Wire 依赖链修复（v1.6 减法后丢失）
- **问题**: v1.6 减法删除 `main.go.BeforeStart` 的 syncer 引用，wire 自动优化掉 VocabSyncer，FUZZY 全部 0 changes
- **修复**: `NewLexnormEngine` 显式接受 `lazySync` 函数参数，`NewLazySyncFunc` provider 把 syncer.lazySync 暴露成函数 → wire 自动连接

### P-FIX v3: fuzzy_vocab 桶排序稳定性
- **问题**: 桶按 Text 排序，多解时 `first-encountered wins`，`'田青' → '田华'`（桶错选），应 → `'田清'`
- **修复**: findBestInBucket 内增加 pinyin sig tie-breaker

### P-FIX v4: n=2 字面 dist=1 conf 阶梯
- **修复前**: n=2 dist=1 conf=0.70 (全保守 suggest)
- **修复后**: n=2 dist=1 + sig 相等 conf=0.85（auto-replace） / sig 不等 conf=0.55（保守 suggest）

---

## 🎯 fuzzy_vocab P10 修复（v1.3 链路）

**问题**: n=2 + maxDist=2 = 100% 命中；桶按 Text 排序稳定收敛到"何焓"/"冯渡"等

**修复**:
- n-aware `effectiveMaxDist(n, configMax)`: n≤2/3: 1, n≥4: configMax
- n-aware conf 阶梯: n=2 dist=1→0.70(Suggest), n≥3 dist=1→0.95(Apply), n≥4 dist=2→0.55(Suggest), n≤3 dist=2→0.30(拒)
- n≤2 关闭 pinyin 兜底

**效果**: FUZZY 噪音 320→6 (-98%)，冯渡霸占问题彻底解决

---

## 🛠 Makefile / Wire 重构

- `run` → `serve`（避免 app.mk `run` 用 `./configs` 目录加载触发 panic）
- 删除 evie/tool 本地 `wire` target 重复定义
- `wire.go` 71 → 43 行（与 evie/service 风格对齐，ProviderSet 在各层）

---

## 📚 文档

- `docs/SERVICE_REQUIREMENTS.md`（565 行，14 章）
- `pkg/credential/README.md`
- `pkg/source/README.md`

---

## ✅ 验证状态

- 全部 14 个测试包绿
- `make wire / build / serve` 全通过
- 真实录音（晨会 121s）Precision=100%
- 0 误改、0 冯渡霸占
- 启动极简（仅 [HTTP] listening + [gRPC] listening）

---

## 📦 部署要求

```yaml
# Redis（token 缓存）
data.redis:
  addr: <redis_addr>:<port>
  password: <password>
  db: 9
  token_key_prefix: "oauth2_access_token:"

# qua 接口
qua:
  base_url: <qua_base_url>
  endpoints:
    list_users: /admin-api/qua/member-extended/page
    list_depts: /admin-api/system/dept/list

# vocab_rules（qua 数据源规则）
vocab_rules:
  sources:
    qua:
      entity_mappings:
      - match: {entity_type: user}
        emit:
          standard_text: name
          category: PERSON
          include_when: "status==1 AND ..."
```

---

## 🔗 相关链接

- 服务文档: `app/evie/tool/README.md`
- 服务需求: `app/evie/tool/docs/SERVICE_REQUIREMENTS.md`
- Proto 定义: `proto/evie/tool/v1/{asr,enhancement,error_reason}.proto`
- 引擎核心: `pkg/lexnorm/`
