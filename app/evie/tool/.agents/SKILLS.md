# evie/tool · 项目开发技能选择

> 本文件为 `app/evie/tool` 开发迭代时选择/加载 skill 的依据。
> 技能路径优先指向 avmc 根 `.agents/skills`；使用前用 `read` 读取对应 `SKILL.md`。

---

## 1. 选择原则

1. **框架技能优先**：当前项目使用 go-kratos，凡是涉及 go-kratos 的 service/biz/data/Wire/proto/config/middleware 开发，第一个必须读取 `kratos-skills/SKILL.md`。
2. **契约技能在 API 改动前启用**：新增/修改 Proto、HTTP/gRPC 端点、配置 schema 前，先读取 `avmc-contract-first-backend/SKILL.md`，并按其分层顺序推进。
3. **按当前项目特征裁剪**：`evie/tool` 是零数据库、内存词库、无前端 UI 的轻量工具；不默认加载 Ent、Vben、前端页面类技能。
4. **问题诊断/审查类技能按任务触发**：不是每次开发都加载，但匹配场景时必须读取。
5. **先读 `.agents/AGENTS.md` 与 `docs/SERVICE_REQUIREMENTS.md`**，再用技能执行；技能不能替代项目边界与文档门禁。

---

## 2. 推荐技能栈

| 使用场景 | 推荐技能 | 何时加载 |
|---|---|---|
| 任何 go-kratos 服务端代码开发 | `kratos-skills` | 必选；进入本服务代码开发前先读 |
| API/Proto/契约改动、分层实现顺序 | `avmc-contract-first-backend` | 涉及 `proto/evie/tool/v1`、生成 API、HTTP/gRPC endpoint、service/biz/data 全链路时 |
| 产品功能需求到交付的端到端规划 | `avmc-feature-delivery` | 任务横跨需求澄清、验收标准、后端实现和功能清单追踪时 |
| Proto 校验规则 / CEL | `protovalidate-skills` | 修改 proto 字段校验、使用 protovalidate 时 |
| 后端 CRUD/Ent 场景 | `entgo-skills` + `avmc-backend-crud` | 仅当未来引入 DB/Ent 时；当前默认不适用 |
| Go 测试代码编写/审查 | `go-testing-code-review` | 新增或修改 `*_test.go`、评审测试质量时 |
| 通用 Code Review | `code-review-checklist` | PR/变更审查时 |
| 项目级交付审查 | `avmc-cross-repo-review` | 涉及根仓库/backend-service/frontend-service 多仓库交付检查时 |
| Git 子仓库提交/指针同步 | `avmc-submodule-sync` | 需要提交 `backend-service` 或同步 avmc 根仓库指针时 |
| Bug/故障定位 | `diagnosing-bugs` / `investigate` | 用户报告异常、测试失败、性能问题时 |
| 模块接口/可测试性设计 | `codebase-design` | 需要设计或改进模块接口、判断边界时 |
| 领域模型/术语统一 | `domain-modeling` | 需要沉淀本服务领域概念、ADR 时 |
| TDD/测试先行 | `tdd` | 用户要求测试先行、red-green-refactor 或补集成测试时 |

---

## 3. 当前项目“默认不加载”的技能

| 技能 | 原因 |
|---|---|
| `avmc-frontend-page` / `vben` / `frontend-design` / `web-design-guidelines` | `evie/tool` 当前无管理后台 UI，前端开发不在本目录范围 |
| `entgo-skills` / `avmc-backend-crud` | 本项目是零数据库、内存 + 文件 + Redis 设计；除非后续明确引入 Ent/DB |
| `design-html` / `design-review` / `design-shotgun` 等 UI 设计类 | 非 UI 项目 |

---

## 4. 推荐使用顺序

### 4.1 普通后端功能开发

```text
kratos-skills
→ docs/SERVICE_REQUIREMENTS.md 中的任务章节
→ .agents/RULES.md
→ 现有相近代码
→ 按需 tdd / go-testing-code-review
```

### 4.2 API/契约新增或修改

```text
kratos-skills
→ avmc-contract-first-backend
→ protovalidate-skills（如涉及 proto 校验）
→ .agents/RULES.md
→ make config / make wire / go test ./...
```

### 4.3 Bug/故障修复

```text
diagnosing-bugs / investigate
→ kratos-skills（troubleshooting/common-issues.md）
→ 现有测试与相关文档
→ 修复后 go test ./... + 回归
```

### 4.4 测试/Review

```text
go-testing-code-review / code-review-checklist
→ .agents/REVIEW.md
→ 按 diff 验证
```

---

## 5. 技能路径速查

项目相关 skills 位于 avmc 根仓库 `.agents/skills`：

```text
/Users/jayden/Development/Code/Object/stack-haven/avmc/.agents/skills/
├── kratos-skills/SKILL.md
├── avmc-contract-first-backend/SKILL.md
├── avmc-feature-delivery/SKILL.md
├── protovalidate-skills/SKILL.md
├── entgo-skills/SKILL.md
├── avmc-backend-crud/SKILL.md
├── avmc-cross-repo-review/SKILL.md
├── avmc-submodule-sync/SKILL.md
├── go-testing-code-review/SKILL.md
├── code-review-checklist/SKILL.md
└── ...
```

Pi 用户级 skills 位于 `~/.agents/skills/`，如 `tdd`、`diagnosing-bugs`、`codebase-design`、`domain-modeling` 等；按场景读取即可。
