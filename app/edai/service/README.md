# EDAI 后端服务

> 状态：**P0 + P1 已落地**（2026-11-10）。认证、数据源、扫描登记、Metadata Search、任务中心可用；
> 规则引擎（P2）/ 预览（P3）/ AI（P4）未开工。
> 决策依据：`docs/architecture/4-2-治理-架构决策记录索引.md` **ADR-013**（相对独立产品：只复用 `pkg`，不依赖 `platform/service` 运行时）。
> 产品与阶段口径：`docs/services/edai/README.md`；开发状态与断点：`docs/architecture/4-6-治理-开发功能清单.md` §4.5。

## 定位

EDAI 的**后端业务服务**。与 `app/evie/service`、`app/platform/service` 平级，但**不依赖中台运行时**：

- 不注入平台 JWT 中间件、不调平台 gRPC、不查平台租户/菜单/配额表；
- 复用 `backend-service/pkg`：`auth`、`audit`、`objectstorage`、`aip/listing`、`entgo/mixin`、`health`、`errors`、`middleware`；
- 前端为独立 Vben 应用 `frontend-service/apps/web-antd-edai`，本地静态路由（无平台菜单 seed）。

## 权限模型

EDAI 不接平台 Casbin（ADR-013），采用「令牌 + 三档角色」，**断言在 `internal/biz/authz.go`，
不在 HTTP 中间件**：将来 MCP 或进程内调用走同一道门，不会因为换了入口就绕过。

| 角色 | 能做什么 | 对应前端 access code |
|---|---|---|
| `admin` | 全部，含删除数据源、发起/取消扫描 | `edai:source:*`、`edai:asset:read`、`edai:task:manage` |
| `operator` | 读写数据源配置、连接测试；**不能删、不能驱动任务** | 无 `delete` / `task:manage` |
| `viewer` | 只读 | 只读码 |

| 接口 | 要求角色 |
|---|---|
| `GET /sources`、`/files`、`/scan-tasks`、`/system/info`、`/auth/me`、`/auth/codes` | 登录即可 |
| `POST/PUT /sources`、`POST /sources/{id}:test-connection` | `admin` 或 `operator` |
| `DELETE /sources/{id}`、`POST /sources/{id}:scan`、`POST /scan-tasks/{id}:cancel` | `admin` |

两个实现约束（都有注释说明原因）：

- **`StartScan` 的断言必须在派生后台协程之前**：`run()` 用 `context.WithoutCancel` 脱离请求
  生命周期（关浏览器不能停扫描），把断言放进协程等于绕过鉴权；
- 403 与 401 必须分开：401 会让前端触发令牌刷新/跳登录，把角色不足误报成 401
  会把 viewer 踢回登录页。`authz_test.go` 按码值断言。

## 快速开始

```bash
cd backend-service/app/edai/service

# 1) 建库并创建默认管理员（首次会打印随机口令，或由 EDAI_ADMIN_PASSWORD 指定）
EDAI_ADMIN_PASSWORD='your-local-password' go run ./cmd/migrate -conf ./configs

# 2) 启动（默认 127.0.0.1:8200）
go run ./cmd/server -conf ./configs

# 3) 登录拿令牌
curl -X POST http://127.0.0.1:8200/edai/v1/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"your-local-password"}'

# 4) 建一个数据源（必须绝对路径）
curl -X POST http://127.0.0.1:8200/edai/v1/sources \
  -H "Authorization: Bearer <accessToken>" -H 'Content-Type: application/json' \
  -d '{"name":"测试目录","type":"local","rootPath":"/tmp"}'

# 5) 连接测试（只读，不写任何登记数据）
curl -X POST http://127.0.0.1:8200/edai/v1/sources/1:test-connection \
  -H "Authorization: Bearer <accessToken>" -H 'Content-Type: application/json' -d '{}'
```

## 操作手册（人工验收按此走）

### 0. 起服务

```bash
cd backend-service/app/edai/service
EDAI_ADMIN_PASSWORD='本地口令' go run ./cmd/migrate -conf ./configs   # 首次建库，会打印管理员口令
go run ./cmd/server -conf ./configs                                    # 127.0.0.1:8200

cd ../../../frontend-service
pnpm -F @vben/web-antd-edai run dev                                    # http://localhost:5678
```

> **报「网络异常」基本都是后端没起或端口不对**：vite 代理写死 `http://127.0.0.1:8200`
> （见 `apps/web-antd-edai/vite.config.ts`），后端换了端口就要用
> `EDAI_BACKEND_URL=http://127.0.0.1:<port> pnpm -F @vben/web-antd-edai run dev` 起前端。
> 自查：`curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8200/health/ready`。

### 1. 登录

浏览器打开 `http://localhost:5678`，用 `admin` + migrate 打印（或指定）的口令登录。

### 2. 数据源管理 → 新增数据源

| 字段 | 说明 |
|---|---|
| 名称 | 唯一（同租户下重名会冲突）；中文可用 |
| 类型 | `local` / `smb`；本地必须是**绝对路径** |
| 根路径 | 扫描起点，必须是运行服务账号能读到的绝对路径，例如 `/Users/you/DesignFiles` |
| 包含规则 | 每行一个 glob（如 `*.psd`），留空=全部包含 |
| 排除规则 | 每行一个 glob（如 `~$*`、`.tmp`、`Thumbs.db`），命中的目录整棵跳过 |
| 备注 | 可空 |

保存后先点**连接测试**：只读探测目录可达与权限，返回顶层目录/文件数，
并把结果写进「连接状态」。**不会登记任何文件**。

### 3. 发起扫描

行内「扫描」按钮 → 确认框（明确写了只读、不动原始文件）→ 提交后立即返回任务号，
进度去**任务中心**看（关掉浏览器不影响执行，这是刻意的）。

API 直调（前端按钮固定发「全量、不算哈希」的扫描）：

```bash
curl -X POST "http://127.0.0.1:8200/edai/v1/sources/1:scan" \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"fullScan":true,"computeHash":true,"subPath":"A-客户甲"}'
```

| 参数 | 语义 |
|---|---|
| `fullScan=true` | 扫完后把**本轮没再出现**的路径标 `missing`。仅当**未限定子路径**时才做（见下一行）；增量扫描一律不做 |
| `computeHash=true` | 逐文件算 SHA256；**内容去重只有开了它才有意义**，代价是大目录明显变慢 |
| `subPath` | 只扫根路径下的子树；越界（`../`）直接 400。**带子路径时后端自动跳过消失判定**（即使传了 `fullScan`）——只看到子树就没资格断言子树之外还在不在 |

调资源占用（改完重启后端生效）：

```bash
EDAI_SCAN_WORKERS=8 EDAI_SCAN_FILES_PER_SECOND=0 go run ./cmd/server -conf ./configs
```

实测 10000 个文件：默认（4 worker + 500/s）≈ 19.4s；`8 worker + 不限速` ≈ 7.2s。
限流速度的目的是**不打满机器、不把远端 SMB 打死**，不是越快越好。

### 4. 任务中心

看 `已发现 / 新增 / 更新 / 标记消失 / 跳过 / 失败` 与状态；有运行中任务时页面每 3 秒自动刷新。
「取消」是**协作式**的：状态先落 `cancel_requested`，worker 在安全点停下再转 `cancelled`，
已登记的文件保留、不会回滚，也不会把「取消导致的在途写库」计成失败。

### 5. 资产文件（Metadata Search）

四个统计卡（总数/存在/已消失/总占用）+ 列表。搜索框对应关系：

| 输入框 | 生成的 AIP-160 片段 | 语义 |
|---|---|---|
| 文件名 | `file_name:"poster"` | **包含**匹配 |
| 扩展名 | `extension=".psd"` | 精确匹配（自动补 `.`） |
| 状态 | `file_status="present"` | 精确匹配 |
| 重复文件 | `is_duplicate=true` / `is_duplicate=false` | 见下 |

「所在目录」列支持 `raw_path:"/2024"` 这类条件（前端未开输入框，API 可用）。
其它可用能力：

```bash
# 组合、布尔、范围、时间
curl -G "http://127.0.0.1:8200/edai/v1/files" -H "Authorization: Bearer $TOKEN" \
  --data-urlencode 'filter=file_name:"海报" AND (extension=".psd" OR file_size > 1048576)' \
  --data-urlencode 'order_by=file_size desc' --data-urlencode 'pageSize=50'

curl -G ".../scan-tasks" --data-urlencode 'filter=failed>0'
curl -G ".../sources"    --data-urlencode 'filter=root_path:"/srv" AND connection_status="reachable"'
```

**写错的字段会直接 400，不会静默返回全量**（`bogus="x"` → `过滤条件无效: undeclared identifier`）。
分页 `page_size` 上限 200，`page_token` 是数字 offset。

### 6. 内容去重怎么看

带 `computeHash=true` 的扫描在收尾时重算重复标记：同 `(租户, 数据源, content_hash)`
出现多次时，**id 最小的那条留作正本**，其余标 `is_duplicate`。

- 「重复文件」列三态：`仅重复` / `非重复` / `-`。**`-` 表示没算过内容哈希 = 不知道**，
  不是「确认没有副本」；因此筛 `is_duplicate=false` 只会返回算过哈希的记录。
- **跨数据源不互相标重复**：两个源里的同一份素材可能是各自授权取得的，
  标成重复会误导「可以删一份」。
- 想全量刷新标记：再跑一次 `computeHash=true` 的扫描即可（幂等）。

### 7. 消失检测

`fullScan` **且未限定子路径**时，扫描结束后把本轮没再出现的路径标 `missing`（记录**不删**，可追溯）。
带 `subPath` 的扫描会自动跳过这一步——观察范围只有子树，据此断言整源消失是错的
（实测过：只扫 `A-客户甲` 会让 `B-客户乙` 全部变 missing）。
删除数据源配置只删配置行，不影响已登记记录与磁盘文件。

### 8. 常见现象判断

| 现象 | 结论 |
|---|---|
| 前端提示网络异常 | 后端未启动 / 端口与代理不一致（见 §0） |
| 扫描很慢 | 大概率是限速或 `computeHash=true`；先只扫一个子目录验证 |
| 重复文件列全是 `-` | 该数据源从没跑过 `computeHash=true` 的扫描 |
| 「已消失」数量异常大 | 只可能来自**整棵根目录**的全量扫描（子路径扫描已不做该判定）；先确认根路径是否填成了上层目录 |
| 取消后还有文件在登记 | 正常：在途任务会写完再停，这是「停在安全点」的定义 |
| 资产列表进来看得见几行、然后越来越短 | 已修（2026-11-10）。统计卡 + Grid 的布局必须保证高度链每一环确定：wrapper `h-full`、Grid `flex-1 min-h-0`，否则 vxe 的 `height:'auto'` 会自引用递归收缩 |


## 配置（`configs/config.yaml`）

| 项 | 默认值 | 说明 |
|---|---|---|
| `server.http.addr` | `127.0.0.1:8200` | **D4**：仅本机。改 `0.0.0.0` 等于对局域网开放，需先补 P6 的 Origin/Host 校验 |
| `server.grpc.addr` | `127.0.0.1:9200` | 同上 |
| `data.database.driver` | `sqlite` | **D5**：SQLite + WAL；也支持 `mysql` |
| `data.database.source` | `file:./data/edai.db?_pragma=journal_mode(WAL)&...` | 库文件在服务目录的 `data/` 下 |
| `server.http.middleware.auth.key` | 开发占位 | **必须用 `EDAI_JWT_KEY` 覆盖**；固定口令等于任何人都能伪造令牌 |

环境变量覆盖（`internal/runtimeconfig`）：`EDAI_HTTP_ADDR`、`EDAI_DB_DRIVER`、`EDAI_DB_SOURCE`、`EDAI_DATA_DIR`、`EDAI_RUN_MODE`、`EDAI_ADMIN_PASSWORD`。

扫描资源限制（`internal/biz/biz.go`，只影响扫描，不进配置文件）：

| 环境变量 | 默认 | 说明 |
|---|---|---|
| `EDAI_SCAN_WORKERS` | `4` | 并发处理文件的工作协程数 |
| `EDAI_SCAN_FILES_PER_SECOND` | `500` | 每秒处理文件数上限；设 `0` 表示不限速（仅建议本机 SSD） |

## 目录结构

```text
app/edai/service/
├── cmd/
│   ├── server/          # HTTP + gRPC 入口（wire 注入）
│   └── migrate/         # 建表 + 首次创建管理员
├── configs/config.yaml
├── data/edai.db         # SQLite（运行时生成，不入库）
└── internal/
    ├── biz/             # 业务逻辑（认证、数据源、路径可达性检查）
    ├── conf/            # 配置类型别名 → api/common/conf
    ├── data/            # Ent 仓库实现 + 认证器装配
    │   └── ent/         # schema / mixins / rule / viewer / 生成代码
    ├── runtimeconfig/   # 配置加载、环境变量覆盖、校验
    ├── server/          # HTTP/gRPC server + 中间件链
    └── service/         # proto 服务实现（薄适配）
```

## 已实现能力（P0/P1 部分）

| 能力 | 状态 | 说明 |
|---|---|---|
| 本地认证 | ✅ | 用户名密码 → JWT（`pkg/auth` 自签）；刷新令牌；登出；权限码 |
| 后端角色断言 | ✅ | `admin`/`operator`/`viewer` 三档，写接口在 biz 拦截（见「权限模型」） |
| 数据源 CRUD | ✅ | 名称/类型/根路径/包含排除规则；路径唯一 |
| 数据源连接测试 | ✅ | 只读检查目录可达与权限，统计顶层目录/文件数 |
| 健康检查 | ✅ | `/health/live`、`/health/ready`（只查库，不依赖 Redis） |
| 系统信息 | ✅ | `/edai/v1/system/info`：版本、运行模式、库类型、数据目录 |
| 租户隔离 | ✅ | 全部实体带 `tenant_id`；V1.0 固定为 1，由中间件注入 |
| 文件扫描 | ✅ | P1；worker 池 + 全局限速，支持协作式取消与子路径限定 |
| 文件检索（AIP-160） | ✅ | 名称/路径/扩展名/状态/大小/时间/重复标记；未声明字段直接 400，不静默忽略 |
| 内容去重 | ✅ | 带哈希扫描收尾重算 `is_duplicate`（同数据源内，id 最小者为正本）；未算哈希不参与、UI 显示 `-` |
| 消失检测 | ✅ | 全量扫描后本轮未见者标 `missing`；路径集合 + keyset 翻页，几万文件不受绑定变量上限影响 |
| 规则引擎 | ⬜ | P2 |
| 预览/处理器 | ⬜ | P3 |
| AI 识别 | ⬜ | P4 |

## 安全要点

1. **默认只监听 loopback**。开放局域网前必须先做 P6（Origin/Host 校验、会话安全属性）。
2. **JWT 密钥必须覆盖**。`configs/config.yaml` 里的是开发占位值。
3. **管理员口令不落硬编码**。首次启动随机生成并打印一次，或用 `EDAI_ADMIN_PASSWORD` 指定。
4. **原始文件只读**。数据源连接测试与后续扫描都不修改、不移动、不重命名客户文件。
5. **删除数据源只删配置**。`DELETE /sources/{id}` 删除的是数据源**配置行**（硬删），
   已登记的 `FileRecord` 与磁盘上的原始文件都不动——历史资产不能因为一次配置删除而消失。
   ⚠️ EDAI 当前四张表用的是 `BaseMixin`（ID/时间戳/状态/tenant_id），
   **没有**接 `SoftDeleteMixin`；`mixins/deleted_at.go` 存在但未被引用，别误以为有软删。
6. **写操作在后端断言角色**。见下方「权限模型」——前端的 access code 只负责隐藏按钮，不是权限屏障。

## 迁移

`cmd/migrate` 执行 Ent `Schema.Create`，**不开启** `WithDropColumn` / `WithDropIndex`：

```go
// 有意不加这两个选项——一旦有真实数据，隐式删除列/索引会造成数据丢失。
// 删除列请显式写脚本执行。参见 4-2 ADR-014 对既有服务该风险的记录。
client.Schema.Create(ctx)
```

## 已知限制

| 限制 | 说明 |
|---|---|
| 过滤语法为 AIP-160 子集 | 支持 `AND`/`OR`/`NOT`/`=`/`!=`/`<`/`<=`/`>`/`>=`/`:`（包含）；不支持自定义函数 |
| 去重只在单个数据源内 | 跨数据源不互相标重复（可能是同一素材的独立授权副本） |
| 无平台菜单/权限组 | 前端静态路由；后端按角色断言（当前仅有 admin） |
| 无 SMB 凭据管理 | SMB 走 UNC 路径，凭据以运行服务账号为准 |
| 单租户 | `tenant_id` 固定 1，未实现多租户切换 |
