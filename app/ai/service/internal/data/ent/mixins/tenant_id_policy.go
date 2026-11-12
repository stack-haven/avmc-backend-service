//go:build !ent_bootstrap
// +build !ent_bootstrap

package mixins

import (
	"entgo.io/ent"

	"backend-service/app/ai/service/internal/data/ent/rule"
)

// Policy 把租户隔离挂到所有嵌入本 mixin 的实体上。
// 未嵌入的实体不会获得任何过滤，因此新增实体必须显式选择。
func (TenantID) Policy() ent.Policy {
	return rule.FilterTenantRule()
}
