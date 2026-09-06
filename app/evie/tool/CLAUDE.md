# evie/tool · Claude Code 入口

> Claude Code 进入本目录时建议先读此文件。
> 详细项目规则见 `.agents/`。

## 必读顺序

1. `.agents/AGENTS.md` — 项目定位、当前状态、恢复协议
2. `.agents/SKILLS.md` — 项目开发技能选择
3. `.agents/RULES.md` — 开发与架构硬规则
4. `.agents/DESIGN.md` — 服务边界与设计原则
5. `.agents/REVIEW.md` — Code Review 检查清单
6. `docs/SERVICE_REQUIREMENTS.md` — 完整服务需求与当前断点
7. `pkg/credential/README.md` + `pkg/source/README.md` — 服务内化抽象说明

## 核心约束

- 只维护当前 `app/evie/tool` 项目，不主动修改其它服务/公共包。
- 需求与文档先行；开发前查看 `docs/SERVICE_REQUIREMENTS.md` 的“未完成计划”。
- `service → biz → data` 单向依赖。
- 禁止全局可变状态、Processor 内部硬编码资源加载、继承模拟。
- 生成文件不手工修改；文档/状态变更后同步 Agent 文件。

## 验证

```bash
go test ./...
go test -race ./...
```
