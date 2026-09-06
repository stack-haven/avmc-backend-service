# evie/tool — Agent 项目指南

> 本文件是 Claude Code、Codex 等 coding agent 进入 `app/evie/tool` 的统一入口。
> **修改本项目代码或文档前必须先读本文件。**
> 当前开发范围：只围绕 `app/evie/tool`（即当前目录）维护；不主动修改 `app/evie/service`、`pkg/lexnorm`、平台根文档等其他区域。

---

## 1. 项目定位

`evie/tool` 是从 `app/evie/service` 抽取的**独立轻量语音识别增强工具**：

- 零数据库、配置驱动
- 多 Provider ASR：`funasr`（整段）、`xunfei`（流式）、`mock`（测试/CI）
- 8 层文本增强 Pipeline（基于 `pkg/lexnorm`）
- qua 用户/部门词库自动同步到 per-tenant 内存词库
- Bearer Token 认证：`redis` / `jwt` / `static` Provider 抽象
- 音频本地落盘 `upload/audio/`
- 健康检查 `/health/live` + `/health/ready`

权威需求说明见 [`docs/SERVICE_REQUIREMENTS.md`](../docs/SERVICE_REQUIREMENTS.md)。

---

## 2. 当前目录范围与依赖位置

```text
backend-service/app/evie/tool/     # 当前项目目录（本规则适用范围）
├── .agents/                       # Agent 规则（本目录）
├── cmd/server/                    # main + wire
├── configs/                       # 生产/演示配置
├── docs/                          # 本服务维护文档
├── internal/                      # conf/biz/data/server/service
├── pkg/credential/                # 服务内化认证抽象
├── pkg/source/                    # 服务内化数据源抽象
└── testdata/                      # 回归 / 调试 / 样例数据
```

跨目录依赖（不是日常修改目标）：

- `backend-service/api/evie/tool/v1`：gRPC/HTTP API 生成代码
- `backend-service/proto/evie/tool/v1`：API proto 源
- `backend-service/pkg/asr`、`backend-service/pkg/pinyin`、`backend-service/pkg/lexnorm` 等公共包

> 迭代中如确需改 API/proto 或公共包，先与用户确认再动；默认不在本目录之外新增修改。

---

## 3. 必读入口

### 3.1 实现/修改任务

1. `.agents/AGENTS.md`（本文件）
2. `.agents/SKILLS.md`（技能选择）
3. `.agents/RULES.md`
4. `.agents/DESIGN.md`
5. `.agents/REVIEW.md`
6. `docs/SERVICE_REQUIREMENTS.md`
7. `README.md`
8. `pkg/credential/README.md`
9. `pkg/source/README.md`
10. 相关现有代码和测试

### 3.2 纯文档/状态维护任务

1. `.agents/AGENTS.md`
2. `docs/SERVICE_REQUIREMENTS.md`
3. 需要同步的 README / pkg README

---

## 4. 当前开发状态（以 docs 为准）

最新代码/文档审计：[`.agents/AUDIT.md`](AUDIT.md)。

生产就绪优化计划：[`.agents/OPTIMIZATION_PLAN.md`](OPTIMIZATION_PLAN.md)。

`docs/SERVICE_REQUIREMENTS.md` 是最新完整状态。摘要：

- ✅ M0–M9 全部交付
- ✅ P1–P6 修复 + B1–B4 observability 修复
- ✅ 工具公共化内化（`pkg/credential` + `pkg/source` 服务内化）
- ✅ 11 个测试包全绿（`go test ./...`）
- ⏳ 未完成/待办：
  1. `README.md` 重写为开源版
  2. 原始 M0–M9 设计文档同步
  3. `pkg` 设计文档同步
  4. 架构 `4-6` 断点同步
  5. demo 配置 e2e 验证（`config.demo.yaml` 尚未真实跑通）

> 每完成一个迭代，应同步更新 `docs/SERVICE_REQUIREMENTS.md` 的「当前断点/未完成计划」、`.agents/AUDIT.md`、`.agents/OPTIMIZATION_PLAN.md` 和本文件的「当前开发状态」。

---

## 5. 中断后恢复协议

1. 读本文件，确认项目边界与当前状态
2. 读 `docs/SERVICE_REQUIREMENTS.md`「未完成计划」与「推荐下一动作」
3. 读 `.agents/RULES.md` 和 `.agents/DESIGN.md`
4. 读相关 `internal/`、`pkg/` 代码与测试
5. 确认只改动 `app/evie/tool` 内内容；如需越界先向用户确认
6. 完成后更新状态文档

---

## 6. 验证命令

在 `app/evie/tool` 目录执行：

```bash
go test ./...                # 全量测试（含 pkg）
go test -race ./...          # race 全量
make config                  # 重新生成 internal/conf/conf.pb.go
make wire                    # 重新生成 cmd/server/wire_gen.go
make race                    # 只跑 internal biz/data/server/service race
make health                  # 探测本服务健康状态
```

---

## 7. 关键约束摘要

- 只维护当前 `app/evie/tool` 项目，不顺手改其它服务/包。
- 需求与文档先行；进入开发前先确认 `docs/SERVICE_REQUIREMENTS.md` 中任务与验收边界。
- go-kratos 相关开发第一个读取 `kratos-skills/SKILL.md`；API/契约变更再按 `.agents/SKILLS.md` 选择对应技能。
- 禁止全局可变状态、禁止 Processor 内硬编码资源加载、禁止用继承模拟（Functional Options + 组合）。
- `service → biz → data` 单向依赖，`biz` 接口由 `data` 实现。
- 生成文件（`internal/conf/conf.pb.go`、`cmd/server/wire_gen.go`）不手工修改。
- 所有 map 遍历影响输出时排序，保证确定性。
- 详细规则见 `.agents/RULES.md`。

---

## 8. 相关文档

- 技能选择：[`.agents/SKILLS.md`](SKILLS.md)
- 代码/文档审计：[`.agents/AUDIT.md`](AUDIT.md)
- 生产就绪优化计划：[`.agents/OPTIMIZATION_PLAN.md`](OPTIMIZATION_PLAN.md)
- 拓展方案（v0.x → v1.x）：[`.agents/EXPANSION_PROPOSAL.md`](EXPANSION_PROPOSAL.md)
- 规则：[`.agents/RULES.md`](RULES.md)
- 设计边界：[`.agents/DESIGN.md`](DESIGN.md)
- Review：[`.agents/REVIEW.md`](REVIEW.md)
- 服务需求与状态：[`docs/SERVICE_REQUIREMENTS.md`](../docs/SERVICE_REQUIREMENTS.md)
- 服务 README：[`README.md`](../README.md)
- 认证抽象：[`pkg/credential/README.md`](../pkg/credential/README.md)
- 数据源抽象：[`pkg/source/README.md`](../pkg/source/README.md)
