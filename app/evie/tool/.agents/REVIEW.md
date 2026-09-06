# evie/tool · Code Review 检查清单

> Reviewer 审查 `app/evie/tool` 变更时逐项核对。
> 阻断项任一不通过 → 不合并 / 不进入下一迭代。
> 默认 review 范围：`backend-service/app/evie/tool`。

---

## 一、范围与一致性（R 系列）

- [ ] **R01** — 变更只影响 `app/evie/tool`；确需越界（proto/api/公共包）时在 PR/对话中显式说明
- [ ] **R02** — 与 `docs/SERVICE_REQUIREMENTS.md` 中任务、边界、当前断点一致
- [ ] **R03** — 完成功能后更新 `docs/SERVICE_REQUIREMENTS.md` 与 `.agents/AGENTS.md` 状态
- [ ] **R04** — `README.md` / pkg README 如果涉及行为变化已同步

## 二、架构与依赖（A 系列，阻断项）

- [ ] **A01** — 未出现 `internal/biz → internal/data` 反向依赖
- [ ] **A02** — `internal/service` 不直接调 data 层
- [ ] **A03** — 新依赖已在 Wire ProviderSet 注册
- [ ] **A04** — 跨包 ctx key 放在 `biz` 包
- [ ] **A05** — 没有在 `wire_gen.go` 手写业务闭包
- [ ] **A06** — `pkg/source` / `pkg/credential` 未写死业务字段名
- [ ] **A07** — 没有全局可变状态
- [ ] **A08** — Processor 没有内部硬编码资源加载
- [ ] **A09** — 没有用继承模拟；使用 Functional Options / 组合

## 三、确定性（D 系列，阻断项）

- [ ] **D01** — 影响输出的 map 遍历有排序
- [ ] **D02** — Entries/Variants/Changes 输出顺序可预期
- [ ] **D03** — fuzzy/alias/pinyin 命中逻辑不会重复触发同一 span
- [ ] **D04** — 相同输入 + 相同词库快照结果一致

## 四、HA / 错误处理（E 系列）

- [ ] **E01** — Pipeline 有 recover + ctx timeout + 错误累积/降级
- [ ] **E02** — 词库 sync 失败不阻断主请求
- [ ] **E03** — ASR 增强失败保留 raw text 并标记降级
- [ ] **E04** — 错误聚合使用 `errors.Join`，不用字符串匹配
- [ ] **E05** — 错误不携带 token / 敏感用户文本

## 五、API / 配置（C 系列）

- [ ] **C01** — HTTP 自定义动作使用 `:verb` 路径风格
- [ ] **C02** — 新配置字段同步 `internal/conf/conf.proto` + YAML + 文档
- [ ] **C03** — 生成文件（conf.pb.go / wire_gen.go）未手工修改
- [ ] **C04** — 修改 ASR/enhancement/vocabulary 行为时同步默认配置和回归样例

## 六、测试（T 系列）

- [ ] **T01** — `go test ./...` 全绿
- [ ] **T02** — 涉及并发/热加载/缓存时 `go test -race ./...` 全绿
- [ ] **T03** — 新增 biz / data / service / server 逻辑有对应单测
- [ ] **T04** — 测试数据放 `testdata/`，不污染 `upload/**`
- [ ] **T05** — 真实链路/录音回归结果记录到文档（如适用）

## 七、风格与命名（S 系列）

- [ ] **S01** — Go 文件 snake_case、包名小写简短
- [ ] **S02** — 错误命名 `ErrXxx` / `XxxError`；Kratos reason `UPPER_SNAKE_CASE`
- [ ] **S03** — 缩略词 `ID` / `URL`，不用 `Id` / `Url`
- [ ] **S04** — 构造函数业务依赖在前，`log.Logger` 在后
- [ ] **S05** — Service 结构体 usecase 字段命名为 `uc`
- [ ] **S06** — 导出符号有 GoDoc
- [ ] **S07** — 公共工具先搜索 `backend-service/pkg`，不重复造 helper

## 八、文档（DOC 系列）

- [ ] **DOC01** — 修改架构/API/配置后更新 `docs/SERVICE_REQUIREMENTS.md`
- [ ] **DOC02** — 修改设计边界后更新 `.agents/DESIGN.md`
- [ ] **DOC03** — 修改硬规则后更新 `.agents/RULES.md`
- [ ] **DOC04** — 代码状态变化后更新 `.agents/AGENTS.md` 当前开发状态
- [ ] **DOC05** — 文档路径、命令与当前 Makefile / 代码一致

---

## 阻断项速查

| 系列 | 说明 |
|---|---|
| R | 范围与文档状态 |
| A | 架构依赖（biz→data 反向、Wire、耦合、全局状态） |
| D | 确定性 |
| E | HA / 错误处理 |

---

## 合并条件

1. `go test ./...` 全绿
2. 涉及并发/热加载变更时 race 全绿
3. 无架构阻断项
4. 文档与 Agent 状态已同步
5. 变更范围符合用户指定
