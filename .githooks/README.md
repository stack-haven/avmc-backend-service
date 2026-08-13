# Git Hooks（backend-service 子仓库自维护）

本目录存放 backend-service 的 git hooks，随子仓库版本管理。

## 激活方式

`core.hooksPath` 是本地配置，不随仓库提交，clone 后执行一次即可：

```bash
make setup-hooks
```

等价于：

```bash
git config core.hooksPath .githooks
```

## 已有 hooks

| 文件 | 触发时机 | 行为 |
|------|----------|------|
| `pre-commit` | 每次提交前 | 若 `deploy/sql/` 24 小时内无备份，打印提醒（非阻塞） |
| `commit-msg` | 提交信息生成后 | 校验提交信息符合后端 Conventional Commits 规范（阻塞，不合规拒绝提交） |

## 提交信息规范（commit-msg 强制）

格式：

```
<type>(<scope>): <subject>
```

- **type**（必填）：`feat` / `fix` / `perf` / `refactor` / `style` / `docs` / `test` / `build` / `ci` / `chore` / `revert`
- **scope**（可选，填写必须合法）：严格按 `backend-service` 结构映射
- **subject**（必填）：简短描述，header 总长 ≤ 108 字符

scope 与目录映射：

| scope | 目录 |
|-------|------|
| `platform` | `app/platform/admin`（平台基础服务：认证/用户/角色/菜单/权限/租户等） |
| `evie` | `app/evie/service` |
| `version` | `app/version/service`（冻结中） |
| `ai` | `app/ai/service` |
| `proto` | `proto/`、`api/`（API 契约与生成代码） |
| `pkg` | `pkg/`（共享包） |
| `make` | `Makefile`、`app.mk`、`scripts/`、`.golangci.yml`（构建与工具链） |
| `deps` | `go.mod`、`go.sum`（依赖） |
| `ci` | `.github/`（CI/CD） |

示例：

```bash
feat(platform): 新增用户管理 CRUD 接口
fix(platform): 修复 JWT token 过期未刷新问题
feat(proto): 新增角色权限 RPC
chore(deps): 升级 entgo 到最新版
```

> merge / revert / squash 提交由 git 自动生成，自动跳过校验。

## 数据库备份

提交后端代码时，pre-commit hook 会提醒备份当前环境 MySQL 数据库。执行：

```bash
bash scripts/db-backup.sh          # 备份所有服务库（结构 + 数据）
bash scripts/db-backup.sh --dry-run  # 预览将备份哪些库
```

脚本自动扫描 `app/**/configs/config.yaml` 的 `data.database.source`（Go DSN），
循环导出到 `deploy/sql/<库名>_<YYYYMMDDHHMM>.sql`。

- `deploy/sql/*.sql` 已加入 `.gitignore`，备份是本地产物，不入库。
- 无需本机 mysql 客户端，优先走 `docker exec` 进 MySQL 容器。
- 服务与库对应关系：platform-admin→`platform_system`、evie→`platform_evie`、
  ai→`platform_ai`、version→`test`（库不存在自动跳过）。
