# ark-lexnorm · 开发与架构规则

> 所有 coding agent 在本目录修改任何 `.go` / `.md` 前必读。
> 冲突优先级：本文档 > 文档拆分（`docs/`）> 历史归档（1.0 / 1.1）> 临时对话。

---

## 1. 命名与术语硬约束

### 1.1 核心包**禁止**出现的业务词

以下词汇**任何形式**（含变体、大小写、缩写）均不得出现在 ark-lexnorm 仓库源码（`/`）中：

```text
Tenant / tenantID / TenantID
ASR / OCR
HR / Employee / Customer / Meeting
Agent / Document / DocumentID
Org / OrgID / 部门 / 组织架构
产品 / 业务 / 租户
```

如需表达"业务上下文隔离"，统一用 `Profile`；如需表达"知识来源"，统一用 `LexiconSource`。

### 1.2 保留词

| 词 | 用途 |
|---|---|
| `Profile` | 规范化上下文标识 |
| `ProfileID` | Profile 标识符 |
| `Lexicon` | 知识容器 |
| `LexiconSource` | 知识来源抽象 |
| `Processor` | 最小规范能力单元 |
| `Pipeline` | Processor 组合机制（**1.2 起为 interface**） |
| `State` | 单次请求工作区 |
| `Result` | 规范化结果（含 RuntimeInfo） |
| `Runtime` | 一次 Normalize 的不可变快照 |
| `Engine` | Facade |
| `ProfileResolver` | ProfileID → Runtime 解析器 |

---

## 2. 架构不变量（15 条）

**任何 PR 不得违反以下任一不变量。** Reviewer 必须在 review 时逐条核对（详见 [REVIEW.md](REVIEW.md)）。

| # | 不变量 | 验证方法 |
|:--:|---|---|
| 1 | Processor 可以独立运行 | 单测脱离 Engine / Pipeline |
| 2 | Pipeline 本身实现 Processor 接口 | `var _ Processor = (Pipeline)(nil)` 编译期断言 |
| 3 | Engine 不承载业务状态 | grep 检查 Engine 内部 |
| 4 | State 不跨请求共享 | race test 覆盖 |
| 5 | Lexicon Runtime 只读 | 测试覆盖 Build 后不可写 |
| 6 | 文本修改只能通过 State.Replace / Suggest | grep 检查 `strings.ReplaceAll` / 字符串拼接 |
| 7 | Protected Span 必须阻止后续非法覆盖 | 业务场景 C 验证 |
| 8 | 一次请求使用一致 Runtime Snapshot | 业务场景 D 验证 |
| 9 | 相同 Input + 相同 Snapshot → 确定性结果 | Golden Test + 多次运行比对 |
| 10 | Processor 失败不能默认导致原始文本丢失 | 业务场景 E 验证 |
| 11 | Profile 不等于 Tenant | grep 检查 `tenant` 出现 |
| 12 | 核心包不得依赖业务系统 | grep 检查业务词 |
| 13 | 外部 Lexicon 数据必须经过 Builder 才能进入运行时 | Lexicon 构造路径验证 |
| 14 | Lexicon 更新必须原子发布 | Last Known Good 验证 |
| 15 | 旧 Snapshot 必须支持正在执行的请求完成 | 业务场景 D 验证 |

详见 [`docs/11-确定性与匹配冲突消解.md`](../docs/11-确定性与匹配冲突消解.md) 与 [`docs/17-开发实施路线.md`](../docs/17-开发实施路线.md) §3。

---

## 3. 决策日志约束（**D1-D7**）

任何 PR 修改以下 7 项中的任一项，**必须**在 PR 描述中显式说明决策变更：

| ID | 决策 | 不可变更的默认值 |
|:--:|---|---|
| **D1** | LLM 不在 Standard Preset | `preset.Standard()` **不包含** `processor/llm` |
| **D2** | `New(...) (*Engine, error)` | 删除 error 返回 → 阻断 |
| **D3** | Result 保留全部字段 | 删除 Original/Duration/Steps/Err 任一 → 阻断 |
| **D4** | Match 冲突：Longest → Priority → Lex | 修改顺序 → 阻断 |
| D5 | 4 个 sentinel：ErrInvalidConfig / ErrInvalidSpan / ErrConflict / ErrRuntime | 恢复 ErrProcessorNotFound / ErrTextTooLarge → 阻断 |
| D6 | Pipeline 是 interface | 回退到 struct → 阻断 |
| D7 | EventType 是 uint8 枚举 | 改为 string → 阻断 |

> 决策变更路径：必须新建决策（**D8+**），在 1.2 头部「决策日志」追加，并在本文件更新默认值。

---

## 4. 包结构与依赖

### 4.1 包布局

```
github.com/stack-haven/lexnorm/  (本地根目录)
├── *.go                       # 核心 API（Engine / Pipeline / Processor / State ...）
├── lexicon/                   # 知识容器
├── processor/<name>/          # 内置 Processor 实现
├── internal/                  # 内部工具（interval / text / pool ...）
├── hooks/                     # 可选 Hook 实现（slog / metrics）
├── examples/                  # Example 函数（Go Example 规范）
├── testdata/                  # 测试数据
└── docs/                      # 文档
```

### 4.2 依赖约束

```bash
# 必须通过
go list -deps ./... | grep -v "^github.com/stack-haven/lexnorm" | grep -v "^internal/"
```

**只允许依赖标准库。** 任何第三方依赖（包括 `golang.org/x/*`）必须：

1. 在 PR 描述中说明必要性
2. 推到独立 submodule（如 `hooks/otel`）
3. 主模块 `go.mod` 不出现该依赖

### 4.3 导入路径

```go
import "github.com/stack-haven/lexnorm"                    // 核心包
import "github.com/stack-haven/lexnorm/lexicon"            // Lexicon 包
import "github.com/stack-haven/lexnorm/processor/alias"    // 内置 Processor
```

`internal/` 子包**不对外暴露**，仅核心包内部使用。

---

## 5. 公共 API 冻结规则（v1.0）

### 5.1 永久冻结（v1.0 起不变）

```go
// Processor 最小接口
type Processor interface {
    Name() string
    Process(context.Context, *State) error
}

// 错误 sentinel
var ErrInvalidConfig error
var ErrInvalidSpan   error
var ErrConflict      error
var ErrRuntime       error

// 引擎入口
func New(opts ...Option) (*Engine, error)  // D2
func (e *Engine) Normalize(ctx context.Context, text string, opts ...CallOption) (Result, error)

// 核心值类型
type Span struct { Start, End int }            // 半开区间，UTF-8 字节偏移
type Change struct { ... }
type Result struct { Original, Text, ... }      // D3: 保留 Original
```

### 5.2 可扩展 API

```go
// Optional Interface（自愿实现）
type Versioner interface { Version() string }

// 注册机制（独立于 Engine）
type Registry struct { ... }
type Descriptor struct { ... }
```

### 5.3 内部 API

- `internal/` 全部视为内部实现
- 任何 `internal/...` 导出符号必须**有充分理由**
- 重命名 / 移动 / 删除不需要走 deprecation

---

## 6. Go 编码规范

### 6.1 通用约束

- **Go 版本**：最低 1.22（与 `ark-go-version-pinning` 一致）
- **格式化**：`gofmt` + `goimports`
- **静态检查**：`go vet` 必须通过
- **Lint**：可选 `golangci-lint`，配置见 `backend-service/.golangci.yml`（如未来加入）

### 6.2 命名

| 类别 | 规则 |
|---|---|
| 包名 | 全小写单词，不复数（`lexicon` 而非 `lexicons`） |
| 接口 | 优先名词（`Processor`、`Resolver`）或 `-er` 后缀（`Reader`） |
| 结构体 | 名词（`Engine`、`Runtime`） |
| 方法 | 动词（`Process`、`Resolve`、`Replace`） |
| 错误 | `ErrXxx`（sentinel）/ `XxxError`（类型化） |
| 常量 | `CamelCase`（不使用 `SCREAMING_SNAKE_CASE`，除非枚举语义匹配） |
| 缩略词 | 大小写一致：`ID` 而非 `Id`，`URL` 而非 `Url` |

### 6.3 错误处理

```go
// ✅ 错误透传
if err != nil {
    return nil, fmt.Errorf("lexnorm: %w", err)
}

// ✅ Sentinel error
errors.Is(err, lexnorm.ErrInvalidConfig)

// ✅ Type assertion
var pe *lexnorm.ProcessorError
errors.As(err, &pe)

// ❌ 字符串匹配
if strings.Contains(err.Error(), "invalid") { ... }

// ❌ Panic 表示正常错误
if err != nil { panic(err) }   // 仅在 init / 不可恢复场景
```

### 6.4 并发

```go
// ✅ 同步原语
sync.Mutex / sync.RWMutex
sync.Map（仅在大量读写场景）
atomic.Pointer[T]（用于 Snapshot 切换）

// ✅ Goroutine 退出保证
go func() {
    defer wg.Done()
    ...
}()

// ❌ 全局可变状态
var globalCache = make(map[string]string)

// ❌ 共享 State
state := &State{}
go func() { state.Replace(...) }()    // 违反不变量 4
```

### 6.5 性能

- **避免不必要的堆分配**：热点路径使用 `sync.Pool`（见 `internal/pool`）
- **避免反射**：核心路径不使用 `reflect`
- **避免类型断言**：接口尽量收窄
- **Benchmark 必须有数据**：见 `docs/14-性能设计与算法优化.md` §1.2

---

## 7. 测试规范

### 7.1 强制覆盖

| 测试 | 命令 | 必须 |
|---|---|:--:|
| 单元测试 | `go test ./...` | ✅ |
| Race Test | `go test -race ./...` | ✅ |
| Fuzz | `go test -fuzz=FuzzXxx` | ✅（接受字符串的公开 API） |
| Benchmark | `go test -bench=. -benchmem ./...` | ✅（核心 Processor + Engine） |
| Coverage | `go test -coverprofile=cover.out ./...` | ≥ 80%（核心包 ≥ 90%） |

### 7.2 命名

```
TestXxx_Method_State      // 单元
TestXxx_Concurrent        // 并发
FuzzXxx_Method            // Fuzz
BenchmarkXxx_Method       // Benchmark
```

### 7.3 测试数据

- 使用 `testdata/` 而非 inline 字符串
- 大数据集使用 `testdata/fixtures/<name>.txt`
- Fuzz corpus 放在 `testdata/fuzz/<name>/`

### 7.4 业务场景验收（M12 必须全部通过）

| 场景 | 验证目标 |
|---|---|
| A：ASR | 多 Change 文本改写 |
| B：会议 | 多 Change 文本改写 |
| C：保护 | Protected Span 阻止覆盖（不变量 7） |
| D：热更新 | 请求一致性（不变量 8/14/15） |
| E：故障 | 降级 + 原文保留（不变量 10） |
| F：多 Profile | 隔离 + Runtime 锁定 |

详见 [`docs/13-应用场景与Pipeline模板.md`](../docs/13-应用场景与Pipeline模板.md) §2。

---

## 8. 文档规范

### 8.1 文档类型

| 类型 | 位置 | 维护者 |
|---|---|---|
| 权威规范 | `../docs/ark-lexnorm-架构设计与开发规范1.x.md` | Tech Lead |
| 拆解文档 | `docs/0X-*.md` | Tech Lead + 贡献者 |
| Agent 规则 | `.agents/*.md` | Tech Lead |
| 公共 README | `README.md` / `README.zh-CN.md` | Tech Lead |
| 变更日志 | `CHANGELOG.md` | Release Manager |
| GoDoc | `*.go` 注释 | 贡献者 |

### 8.2 文档维护触发条件

| 触发 | 必须更新 |
|---|---|
| 修改公共 API | `1.2` 对应节 + `docs/` 对应文档 + `CHANGELOG.md` |
| 修改决策（D1-D7） | 1.2 头部决策日志 + `.agents/RULES.md` §3 + 创建新决策 |
| 修改不变量 | 1.2 §42 + `docs/11` 或 `17` + 本文件 §2 |
| 新增 Processor | `docs/12-内置Processor规范.md` |
| 新增错误 sentinel | `docs/10-配置校验与错误体系.md` §1 |

### 8.3 Markdown 风格

- 使用中文，技术标识 / 路径 / 命令 / API 名保留英文
- 标题层级：`##` 起，禁用 `#` 一级（除 README）
- 代码块必须标注语言（` ```go ` / ` ```bash `）
- 列表必须用 `-` 而非 `*`

---

## 9. Git 提交规范

### 9.1 Commit 格式

```
<scope>(<type>): <subject>

<body>

<footer>
```

**Type**:

- `feat` 新功能
- `fix` 修复
- `refactor` 重构
- `docs` 文档
- `test` 测试
- `chore` 杂项
- `perf` 性能

**Scope**（可选）：

- `engine` / `pipeline` / `processor` / `state` / `result` / `lexicon` / `docs` / `agents`

### 9.2 示例

```
engine(feat): implement New() with error return (D2)

- New(opts ...Option) (*Engine, error)
- Construct-time validation: ErrInvalidConfig on illegal config
- Documented in docs/07-Engine与Profile.md §2

Refs: D2
```

---

## 10. 自检清单

每次提交前：

### 10.1 命名

- [ ] 是否引入 §1.1 禁止的业务词？
- [ ] 公共导出符号是否命名清晰？
- [ ] 接口命名是否一致（`-er` / 名词）？

### 10.2 架构

- [ ] 是否破坏 §2 任一不变量？
- [ ] 是否违反 §3 任一决策？
- [ ] 是否引入第三方依赖？
- [ ] 是否引入 `tenant` / `asr` / `ocr` / `hr` / `employee` 等业务词？

### 10.3 质量

- [ ] `go test ./...` 全绿？
- [ ] `go test -race ./...` 全绿？
- [ ] `go vet ./...` 无警告？
- [ ] 新 API 是否有 Example？
- [ ] 新 API 是否有 Fuzz？
- [ ] 核心 Processor 是否有 Benchmark？

### 10.4 文档

- [ ] 是否更新对应 `docs/0X-*.md`？
- [ ] 是否更新 `1.2` 对应节（如涉及决策/不变量）？
- [ ] 是否更新 `CHANGELOG.md`？
- [ ] 导出符号是否有 GoDoc？

### 10.5 决策

- [ ] 是否修改 D1-D7 任一默认值？（若是，必须新建决策）

---

## 11. 相关文档

- Agent 入口：[AGENTS.md](AGENTS.md)
- 设计原则：[DESIGN.md](DESIGN.md)
- Review 清单：[REVIEW.md](REVIEW.md)
- 权威规范：[../docs/ark-lexnorm-架构设计与开发规范1.2.md](../docs/ark-lexnorm-架构设计与开发规范1.2.md)
- 实施路线：[../docs/17-开发实施路线.md](../docs/17-开发实施路线.md)
- 测试策略：[../docs/15-测试策略与质量工程.md](../docs/15-测试策略与质量工程.md)
