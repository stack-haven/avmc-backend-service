# ark-lexnorm · Code Review 检查清单

> Reviewer 在审查 PR 时**逐项核对**。任何未勾选 / 不适用项必须显式标注原因。
> 阻断项（**S01-S99 / D1-D7**）任一不通过 → **拒绝合并**。

---

## 一、命名与术语（**S 系列，阻断项**）

- [ ] **S01** — 核心包**未出现**业务词：tenant / ASR / OCR / HR / Employee / Customer / Meeting / Agent / Document / 产品
- [ ] **S02** — 包名小写单词、不复数（`lexicon` 而非 `lexicons`）
- [ ] **S03** — 接口命名一致：`-er` 后缀（`Processor` / `Resolver`）或纯名词
- [ ] **S04** — 错误命名：`Err<Concept>`（sentinel）/ `<Concept>Error`（类型化）
- [ ] **S05** — 缩略词大小写一致：`ID` / `URL` / `UTF8`（不用 `Id` / `Url` / `Utf8`）
- [ ] **S06** — 公共导出符号**有 GoDoc**（中文/英文均可）

---

## 二、架构不变量（**A 系列，阻断项**）

15 条不变量逐项验证：

- [ ] **A01** — Processor **可独立运行**（脱离 Engine / Pipeline），有对应单测
- [ ] **A02** — Pipeline **本身实现 Processor 接口**（`var _ Processor = (Pipeline)(nil)`）
- [ ] **A03** — Engine **不承载业务状态**（grep 验证）
- [ ] **A04** — State **不跨请求共享**（race test 覆盖）
- [ ] **A05** — Lexicon **Runtime 只读**（Build 后不可写）
- [ ] **A06** — 文本修改**只通过** `State.Replace` / `State.Suggest`（grep `strings.ReplaceAll`）
- [ ] **A07** — Protected Span **阻止后续非法覆盖**（业务场景 C 验证）
- [ ] **A08** — 一次请求**使用一致 Runtime Snapshot**（业务场景 D 验证）
- [ ] **A09** — 相同 Input + 相同 Snapshot → **确定性结果**（Golden Test + 多次比对）
- [ ] **A10** — Processor 失败**不导致原始文本丢失**（业务场景 E 验证）
- [ ] **A11** — **Profile ≠ Tenant**（grep `tenant`）
- [ ] **A12** — **核心包不依赖业务系统**（grep 业务词）
- [ ] **A13** — 外部 Lexicon 数据**必须经过 Builder**（构造路径验证）
- [ ] **A14** — Lexicon 更新**原子发布**（Last Known Good 验证）
- [ ] **A15** — 旧 Snapshot **支持正在执行的请求完成**（业务场景 D 验证）

详见 [RULES.md §2](RULES.md)。

---

## 三、决策日志（**D 系列，阻断项**）

PR 描述中必须显式标注 **D1-D7** 是否受影响：

- [ ] **D1** — LLM **不在** Standard Preset
- [ ] **D2** — `New(...) (*Engine, error)` **保留 error 返回**
- [ ] **D3** — Result **保留全部字段**：Original / Duration / Steps / Err
- [ ] **D4** — Match 冲突：**Longest → Priority → Lex**
- [ ] **D5** — Sentinel 4 个：`ErrInvalidConfig` / `ErrInvalidSpan` / `ErrConflict` / `ErrRuntime`
- [ ] **D6** — Pipeline 是 **interface**
- [ ] **D7** — EventType 是 `uint8` 枚举

> 任一决策被修改 / 删除 → **必须**新建决策（D8+），并在 PR 中显式说明。

---

## 四、依赖与构建（**B 系列**）

- [ ] **B01** — `go list -deps ./...` 输出**仅**含 stdlib + ark/lexnorm 自身
- [ ] **B02** — 第三方依赖（如未来引入）推到**独立 submodule**
- [ ] **B03** — `go.mod` 中 Go 版本 ≥ 1.22
- [ ] **B04** — `go build ./...` 通过
- [ ] **B05** — `go vet ./...` 无警告
- [ ] **B06** — `gofmt` / `goimports` 无 diff

---

## 五、错误处理（**E 系列**）

- [ ] **E01** — Sentinel error 用 `errors.Is(err, lexnorm.ErrXxx)`
- [ ] **E02** — 类型化 error 用 `errors.As(err, &target)`
- [ ] **E03** — 错误**不吞掉**（Middleware / Hook 不静默）
- [ ] **E04** — 错误**不携带敏感数据**（用户文本 / Token / Key）
- [ ] **E05** — 错误**不作为正常控制流**（`nil` 表示成功）
- [ ] **E06** — `errors.Join` 用于多错误聚合（`Result.Err` 是聚合视图）
- [ ] **E07** — NaN / ±Inf / 越界 Confidence → `ErrInvalidConfig`（不静默 Clamp）
- [ ] **E08** — State 方法错误：`Replace` 越界 → `ErrInvalidSpan`；命中 Locked → `ErrConflict`

---

## 六、确定性（**DET 系列，阻断项**）

- [ ] **DET01** — 无 `map` 迭代顺序参与输出（输出前必须 `sort.SliceStable`）
- [ ] **DET02** — 无 `goroutine` 调度依赖（`time.Sleep` / `channel` 用于控制流）
- [ ] **DET03** — 无 `math/rand` 用于 tie-breaker
- [ ] **DET04** — 无 `time.Now()` 用于决策分支（仅作 Duration 元数据）
- [ ] **DET05** — Candidate slice 显式排序（输出前）
- [ ] **DET06** — Lexicon.All() 返回**确定顺序**（推荐字典序）
- [ ] **DET07** — Compose 多源冲突 → `ErrConflict`（**不静默覆盖**）
- [ ] **DET08** — Match 冲突按 **Longest → Priority → Lex** 消解

---

## 七、并发安全（**C 系列**）

- [ ] **C01** — `go test -race ./...` 全绿
- [ ] **C02** — Engine / Pipeline / Lexicon / Runtime **可并发共享**（无内部 mutable state）
- [ ] **C03** — State **单 goroutine 独占**（无跨 goroutine 访问）
- [ ] **C04** — Snapshot 切换用 `atomic.Pointer[T]`
- [ ] **C05** — 100+ goroutine 并发 Normalize 测试通过

---

## 八、性能（**P 系列**）

- [ ] **P01** — 核心 Processor 有 Benchmark
- [ ] **P02** — Engine.Normalize 有 Benchmark
- [ ] **P03** — Benchmark 数据记录到 `docs/14-性能设计与算法优化.md`
- [ ] **P04** — 无明显回退（与基线对比）
- [ ] **P05** — 45 字 / 2000 词条 < 200 µs / < 32 KB / < 200 allocs
- [ ] **P06** — 1000 字 / 10000 词条 < 3 ms
- [ ] **P07** — 热点路径使用 `sync.Pool`

---

## 九、API 设计（**API 系列**）

- [ ] **API01** — v1.0 API **未做 breaking change**（已冻结接口 / 签名 / 错误 / 字段）
- [ ] **API02** — 新 API 有 GoDoc + Example
- [ ] **API03** — 新 API 有 Fuzz（接受字符串）
- [ ] **API04** — Optional Interface 通过类型断言检测（不强制）
- [ ] **API05** — Pipeline 是 interface（D6 验证）
- [ ] **API06** — `Result.RuntimeInfo` 完整（Profile / Lexicon / Pipeline / Processor Versions）
- [ ] **API07** — `Change.RuleID` / `EntryID` / `Processor` / `ProcessorVersion` 字段齐全

---

## 十、测试（**T 系列**）

- [ ] **T01** — `go test ./...` 全绿
- [ ] **T02** — `go test -race ./...` 全绿
- [ ] **T03** — 接受字符串的公开 API 有 Fuzz
- [ ] **T04** — 核心 Processor 有 Benchmark
- [ ] **T05** — 核心包覆盖率 ≥ 90%
- [ ] **T06** — 业务场景 A-F（M12 时必须全部通过）
- [ ] **T07** — 故障矩阵 11 行全部覆盖（M12）
- [ ] **T08** — Test 命名规范：`TestXxx_Method_State`
- [ ] **T09** — Fuzz 命名规范：`FuzzXxx_Method`
- [ ] **T10** — Benchmark 命名规范：`BenchmarkXxx_Method`

---

## 十一、文档（**DOC 系列**）

- [ ] **DOC01** — 修改公共 API → 更新对应 `docs/0X-*.md`
- [ ] **DOC02** — 修改决策 → 更新 1.2 头部决策日志 + `RULES.md` §3
- [ ] **DOC03** — 修改不变量 → 更新 1.2 §42 + `RULES.md` §2
- [ ] **DOC04** — 新增 Processor → 更新 `docs/12-内置Processor规范.md`
- [ ] **DOC05** — 新增 sentinel → 更新 `docs/10-配置校验与错误体系.md` §1
- [ ] **DOC06** — 公共导出符号有 GoDoc
- [ ] **DOC07** — 更新 `CHANGELOG.md`
- [ ] **DOC08** — Markdown 风格：中文为主 + 技术标识英文 + ` ```go ` 标注语言

---

## 十二、Git 与交付（**G 系列**）

- [ ] **G01** — Commit 格式：`<scope>(<type>): <subject>`
- [ ] **G02** — 涉及决策时 Commit footer 含 `Refs: D<n>`
- [ ] **G03** — PR 描述含：变更摘要 / 关联决策 / 关联 M 编号 / 测试结果
- [ ] **G04** — 无未使用 import / 变量
- [ ] **G05** — 无 `TODO` / `FIXME` / `XXX`（除非有对应 issue 链接）

---

## 十三、阻断项速查

| 系列 | 阻断项 | 数量 |
|---|---|:--:|
| S | 命名 / 业务词 | 6 |
| A | 架构不变量 | 15 |
| D | 决策日志 | 7 |
| DET | 确定性 | 8 |
| **合计** | | **36** |

其他系列（B / E / C / P / API / T / DOC / G）按需审查，**未通过需修改但不一定阻断**。

---

## 十四、PR 合并条件

**全部**满足才能合并：

1. ✅ CI 全绿（lint / vet / test / race / coverage）
2. ✅ 至少 1 个 Approve
3. ✅ **所有阻断项**勾选
4. ✅ 所有非阻断项**显式标注**通过 / 不适用
5. ✅ Benchmark 无回退（或 PR 描述中说明）
6. ✅ 文档同步更新
7. ✅ PR 描述完整（变更 / 决策 / M / 测试）

---

## 十五、相关文档

- 规则：[RULES.md](RULES.md)
- 设计：[DESIGN.md](DESIGN.md)
- Agent 入口：[AGENTS.md](AGENTS.md)
- 实施路线：[../docs/17-开发实施路线.md](../docs/17-开发实施路线.md)
- 业务场景：[../docs/13-应用场景与Pipeline模板.md](../docs/13-应用场景与Pipeline模板.md)
- 测试策略：[../docs/15-测试策略与质量工程.md](../docs/15-测试策略与质量工程.md)
