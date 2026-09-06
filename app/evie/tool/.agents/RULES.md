# evie/tool · 开发与架构规则

> 所有 coding agent 在本目录修改任何 `.go` / `.proto` / `.md` / 配置前必读。
> 冲突优先级：`.agents/RULES.md` > `docs/SERVICE_REQUIREMENTS.md` > `README.md` > 临时对话。

---

## 1. 范围硬规则

- 默认只修改当前 `backend-service/app/evie/tool` 目录内的文件。
- 开发前先按 `.agents/SKILLS.md` 选择并加载匹配 skill；go-kratos 相关开发必须从 `kratos-skills/SKILL.md` 开始。
- 不修改 `app/evie/service`、`pkg/lexnorm`、`backend-service/pkg/*` 等公共区域，除非用户明确指定。
- 需要修改 `backend-service/proto/evie/tool/v1` 或 `backend-service/api/evie/tool/v1` 时，先说明影响并等待确认。
- `docs/SERVICE_REQUIREMENTS.md` 是本项目手维护的权威状态文档；每个迭代结束需同步更新「当前断点」。

---

## 2. 分层依赖方向（强制）

```
internal/service → internal/biz → internal/data
                       ↑ 接口/抽象        ↑ 实现
```

- `biz` 不 import `data`。
- `data` 可以 import `biz` 的接口与 ctx 抽象，并实现这些接口。
- `service` 不直接调用 `data`。
- 跨包 ctx key（如 AuthContext）放在 `biz` 包；`data` 通过 `biz.WithAuth/AuthFrom` 注入。
- 新增 provider / usecase / repo 依赖时同步更新 Wire ProviderSet。
- 禁止在 `wire_gen.go` 中手写闭包；把业务方法放在 usecase 上，由构造器内部注册（如 `AttachLazySync`）。

### 当前项目包职责

| 包 | 职责 |
|---|---|
| `internal/conf` | 配置定义与生成代码（`conf.proto` → `conf.pb.go`） |
| `internal/biz` | 业务用例：ASR、增强、词库、同步、Normalizer |
| `internal/data` | 基础设施：Redis、qua client、ASR provider、健康检查、TokenAuth |
| `internal/server` | HTTP/gRPC 路由与中间件装配 |
| `internal/service` | transport 层 proto ↔ biz 转换 |
| `pkg/credential` | 服务内化认证抽象（redis/jwt/static/middleware） |
| `pkg/source` | 服务内化数据源抽象（http/file/qua） |

---

## 3. 三大禁令

1. **禁止全局可变状态**。状态必须显式注入到结构体或函数。
2. **禁止 Processor 内部硬编码资源加载**。字典 / 配置 / logger 通过 Options/构造参数注入。
3. **禁止继承模拟**。使用 Functional Options + 组合。

---

## 4. 确定性规则

- 所有影响输出/测试断言的 `map` 遍历必须先排序（sorted keys / `sort.Slice` / `sort.Strings`），避免不同 Go 版本 map 随机序。
- Lexicon、Entries、Relations、Candidates、Changes 输出顺序必须可预期。
- 相同输入 + 相同词库快照 → 相同结果。

---

## 5. 文本增强 Pipeline 规则

8 层顺序（本项目实际 lexnorm pipeline 与业务术语映射）：

```text
cleaning / normalize
filler / disfluency
vocab_matching + alias_resolution / alias
deterministic_replacement / deterministic
pinyin_correction / pinyin
fuzzy_matching / fuzzy_vocab
context_correction / ctxproc
```

- 层与层通过 `lexnorm` Engine 组合，不在 biz 中手写替换逻辑。
- 修改 processor 顺序/阈值必须同步 `configs/config.yaml`、`docs/SERVICE_REQUIREMENTS.md`。
- `Change.Processor` / `ProcessorVersion` / `Confidence` / `SuggestThreshold` 语义不要破坏。
- fuzzy 命中同一区间不应重复触发；使用 `hasChangeAtSpan` 等辅助方法保证幂等。

---

## 6. 外部系统解耦（Q13）

```
Source（http/file/qua） → opaque RawEntity → Normalizer（YAML 规则） → NormalizedEntry → VocabularyBuilder
```

- `pkg/source` 不依赖任何业务字段名；`RawEntity.Data` 是不透明 `map[string]any`。
- `pkg/credential` 不依赖业务结构；所有 JSON/JWT claim 字段名通过 `FieldMapper` 配置。
- 新外部数据源不得在 `pkg/source` 中写死业务字段，必须走 YAML Normalizer 映射。
- qua 相关协议只在 `pkg/source/qua` / `internal/data` 薄包装层出现。

---

## 7. HA / 失败语义

- Pipeline 三层保护：`recover` + ctx timeout + error accumulation，失败不能丢失原文。
- 词库 sync 失败不得阻断主请求；用 system/fallback/empty snapshot 降级。
- ASR 增强失败保留 ASR raw text，返回 `DEGRADED` 状态。
- 错误聚合优先 `errors.Join`；不使用 `err.Error()` 字符串匹配。

---

## 8. 命名与代码风格

- Go 包名小写、简短；文件 snake_case。
- 错误：sentinel `ErrXxx`；类型化 `XxxError`；Kratos reason 用 `UPPER_SNAKE_CASE`。
- 缩略词：`ID` / `URL`，不写 `Id` / `Url`。
- 构造函数顺序：业务依赖在前，`log.Logger` 在后。
- Service 结构体 usecase 字段统一用 `uc`。
- 导出符号写 GoDoc；业务注释可用中文。
- 公共工具优先搜索 `backend-service/pkg` 下已有实现，不重复制造转换 helper。

---

## 9. 配置与生成文件

- `internal/conf/conf.proto` 是配置 schema 的事实来源；`conf.pb.go` 由 `make config` 生成。
- `cmd/server/wire_gen.go` 由 `make wire` 生成。
- **禁止手工编辑生成文件**。
- `configs/config.yaml` 是生产/联调配置；`configs/config.demo.yaml` 是演示配置。
- 当前 `conf.proto` 尚未覆盖 `credential` / `vocabulary` 等 demo 段；因此 demo 配置尚未真正接入 main 的 Bootstrap。遇到 demo 相关任务先确认是否已完成该接线。
- 修改配置 schema 后同步 `configs/*.yaml` 与文档。

---

## 10. API 风格

- HTTP 自定义动作路径：`/evie/tool/v1/asr:recognize`（冒号，不用斜杠）。
- HTTP 路径：`/health/live`、`/health/ready` 不鉴权；业务端点走 Bearer Token。
- gRPC 流式参数放 metadata，不放 body。
- 分页/列表记录为 in-memory ring buffer，不落 DB。

---

## 11. 测试要求

- 每个 biz / data / service / server 核心逻辑有 `_test.go`。
- 关键包至少 `go test ./...` 全绿；并发/热加载逻辑跑 `go test -race ./...`。
- 修改 ASR/词库/增强链路后跑真实录音回归（见 `docs/SERVICE_REQUIREMENTS.md` §13.3）。
- 测试数据放 `testdata/`，不在代码中写死长音频/大 JSON。
- Test 命名：`TestXxx_Method_State`。

---

## 12. Git 提交

- 当前 git 根是 `backend-service`，但本项目是其中的 `app/evie/tool` 子目录。
- 提交信息使用 Conventional Commits：`scope(type): subject`，示例 `evie(tool): fix ...`。
- 涉及架构/状态变更时，在提交说明或 PR 描述中注明更新了 `docs/SERVICE_REQUIREMENTS.md`。

---

## 13. 自检清单

每次改动前/后检查：

- [ ] 只改了 `app/evie/tool` 范围内文件？
- [ ] 没有反向依赖（biz → data / service → data）？
- [ ] 没有全局可变状态、Processor 硬编码资源、继承模拟？
- [ ] map 输出已排序？
- [ ] 生成文件没有手工修改？
- [ ] `go test ./...` 全绿？
- [ ] 涉及文档/状态变化已同步 `docs/SERVICE_REQUIREMENTS.md` 和本 Agent 文件？
