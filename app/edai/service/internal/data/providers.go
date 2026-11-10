package data

import (
	"backend-service/app/edai/service/internal/biz"
	"backend-service/app/edai/service/internal/conf"

	"github.com/google/wire"
)

// ProviderSet 数据层依赖注入集合。
var ProviderSet = wire.NewSet(
	NewEntClient,
	NewData,
	NewFixedTenantResolver,
	NewUserRepo,
	NewDataSourceRepo,
	NewFileRepo,
	NewScanTaskRepo,
	NewHealthChecker,
	// 具体仓库 → biz 接口绑定
	wire.Bind(new(biz.UserRepo), new(*userRepo)),
	wire.Bind(new(biz.DataSourceRepo), new(*dataSourceRepo)),
	wire.Bind(new(biz.FileRepo), new(*fileRepo)),
	wire.Bind(new(biz.ScanTaskRepo), new(*scanTaskRepo)),
)

// NewFixedTenantResolver 提供 V1.0 的单租户解析器。
//
// tenant_id 固定为 1：4-2 ADR-013 决策 3 明确「表带 tenant_id，但 Local-first
// 部署固定单租户」，客户端无法指定其他值。
func NewFixedTenantResolver(_ *conf.Data) TenantResolver {
	return FixedTenantResolver{Tenant: 1}
}
