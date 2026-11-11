package data

import (
	"context"
	"fmt"

	"backend-service/app/edai/service/internal/data/ent/gen"
	entviewer "backend-service/app/edai/service/internal/data/ent/viewer"
	"backend-service/pkg/conf"
	"backend-service/pkg/entgo/mixin"
	pkgHealth "backend-service/pkg/health"

	"github.com/go-kratos/kratos/v2/log"
)

// TenantResolver 提供 EDAI 的租户上下文。
//
// V1.0 是 Local-first 单机部署（4-2 ADR-013 决策 3）：tenant_id 由服务端固定注入，
// 不接受客户端传入。保留接口是为了未来 LAN / 多客户化时不改调用方。
type TenantResolver interface {
	// TenantID 返回当前请求应使用的租户ID。
	TenantID(ctx context.Context) uint32
}

// FixedTenantResolver 固定返回单一租户ID（V1.0 默认实现）。
type FixedTenantResolver struct {
	Tenant uint32
}

func (r FixedTenantResolver) TenantID(context.Context) uint32 {
	if r.Tenant == 0 {
		return 1
	}
	return r.Tenant
}

// 编译期确认 pkg mixin 可用（EDAI 全部实体复用该包的 ID/时间戳/状态字段）。
var _ = mixin.ID{}

// Data 聚合数据层依赖。
type Data struct {
	db     *gen.Client
	log    *log.Helper
	tenant TenantResolver
}

// NewData 构造数据层。
func NewData(db *gen.Client, tenant TenantResolver, logger log.Logger) (*Data, func(), error) {
	d := &Data{db: db, tenant: tenant, log: log.NewHelper(log.With(logger, "module", "edai/data"))}
	cleanup := func() {
		if err := db.Close(); err != nil {
			d.log.Errorf("closing ent client: %v", err)
		}
	}
	return d, cleanup, nil
}

// Ent 返回 ent 客户端。
func (d *Data) Ent() *gen.Client { return d.db }

// TenantContext 把固定租户注入 context，供 Ent 隐私策略使用。
func (d *Data) TenantContext(ctx context.Context) context.Context {
	tenantID := d.tenant.TenantID(ctx)
	if tenantID == 0 {
		return ctx
	}
	return entviewer.NewTenantContext(ctx, tenantID)
}

// entSystemContext 返回绕过租户隐私过滤的上下文，仅用于迁移/种子。
// 注意：业务代码禁止调用，否则会绕过租户隔离。
func entSystemContext(ctx context.Context) context.Context {
	return entviewer.NewSystemContext(ctx)
}

// 迁移与种子入口在 ent_client.go 中；这里只暴露运行期依赖。
var _ = RunSchemaMigration
var _ = conf.Data{}

// ptrIfNotEmpty 返回非空字符串的指针；空字符串返回 nil。
//
// 用于 ent（指针字段，如 *string）到 proto（optional string，*string）的转换：
// convert.EmptyToNil[T] 返回 *T，对 *string 会变成 **string，不适用于此处。
func ptrIfNotEmpty(v *string) *string {
	if v == nil || *v == "" {
		return nil
	}
	return v
}

// healthChecker 检查数据库可达性。EDAI 不依赖 Redis（V1.0 单机），故只查库。
type healthChecker struct {
	data *Data
}

// NewHealthChecker 创建健康检查器。
func NewHealthChecker(data *Data) pkgHealth.Checker {
	return &healthChecker{data: data}
}

func (c *healthChecker) Ready(ctx context.Context) error {
	if c == nil || c.data == nil || c.data.db == nil {
		return fmt.Errorf("database: unavailable")
	}
	ctx = entSystemContext(ctx)
	if _, err := c.data.db.User.Query().Limit(1).Count(ctx); err != nil {
		return fmt.Errorf("database: %w", err)
	}
	return nil
}
