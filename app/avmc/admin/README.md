# AVMC Admin Service

Admin 是当前后端主要开发模块。服务使用 Go + Kratos，HTTP 监听 `8000`，gRPC 监听 `9000`。

## Local Run

```bash
cd app/avmc/admin
go run ./cmd/server -conf ./configs
```

本地默认配置不会自动执行数据库迁移。首次启动或 schema 变更后，先显式执行迁移：

```bash
cd app/avmc/admin
go run ./cmd/migrate -conf ./configs
```

从旧版随机 `domain_id` 数据模型升级时，必须先把既有 Admin 租户数据归入明确的数据域。该操作会先检查域内唯一约束冲突，并在单事务中完成归域；发现冲突时不会修改任何数据：

```bash
cd app/avmc/admin
go run ./cmd/migrate -conf ./configs -legacy-domain 1
```

迁移命令会删除已被 schema 替代的旧索引，用域内复合唯一索引替换原有全局唯一索引。生产执行前必须完成数据库备份。

初始化或同步超级管理员 Casbin 策略：

```bash
cd app/avmc/admin
go run ./cmd/policy -conf ./configs -domain 1 -role super_admin -users 1
```

为本地空数据库写入可重复执行的 Admin mock 数据：

```bash
cd app/avmc/admin
go run ./cmd/mock -conf ./configs
```

## Production Baseline

生产环境必须通过环境变量覆盖敏感配置，不要提交真实密钥或 DSN。推荐以 `config.prod.example.yaml` 为模板。不要把生产样例放进 `configs/` 目录，因为 `-conf ./configs` 会加载该目录下的配置文件。

Required overrides:

```bash
export AVMC_ADMIN_ENV=production
export AVMC_ADMIN_JWT_KEY='replace-with-strong-secret'
export AVMC_ADMIN_DB_SOURCE='user:password@tcp(mysql:3306)/avmc_system?charset=utf8mb4&parseTime=true&loc=Asia%2FShanghai'
export AVMC_ADMIN_REDIS_ADDR='redis:6379'
export AVMC_ADMIN_REDIS_PASSWORD='replace-with-redis-password'
export AVMC_ADMIN_CORS_ORIGINS='https://admin.example.com'
export AVMC_ADMIN_ENABLE_SWAGGER=false
export AVMC_ADMIN_DB_DEBUG=false
export AVMC_ADMIN_DB_MIGRATE=false
```

Production startup rejects unsafe defaults:

- JWT key cannot be empty or `some_api_key`.
- CORS origins cannot include `*`.
- Swagger must be disabled.
- Database debug and runtime migration must be disabled.

## Operational Order

1. Deploy config and required environment variables.
2. Run `go run ./cmd/migrate -conf ./configs` as a controlled release step.
3. Run `go run ./cmd/policy -conf ./configs -domain <domain> -role super_admin -users <user_ids>`.
4. Start `go run ./cmd/server -conf ./configs`.

## Quality Gates

```bash
GOCACHE=/private/tmp/avmc-go-cache go test -timeout 60s ./...
```

For local smoke testing:

```bash
curl -I http://127.0.0.1:8000/docs/openapi.yaml
```
