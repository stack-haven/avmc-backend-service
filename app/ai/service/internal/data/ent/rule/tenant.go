//go:build !ent_bootstrap
// +build !ent_bootstrap

// Package rule 实现 ai 服务的 Ent 隐私策略。
//
// 设计约束（与 platform/evie/edai 一致）：
//   - 未认证租户上下文 → 拒绝，绝不隐式放行；
//   - create 时由规则注入 tenant_id，调用方无需也不应显式传入错误值；
//   - update/delete 追加租户谓词，跨租户不可达；
//   - 未经登记的类型走 default 拒绝（fail-closed），而不是默认放行。
package rule

import (
	"context"

	"entgo.io/ent"
	"entgo.io/ent/privacy"

	"backend-service/app/ai/service/internal/data/ent/gen"
	"backend-service/pkg/entgo/viewer"
)

const tenantIDField = "tenant_id"

// tenantFieldMutation 是所有生成的 Mutation 都满足的最小接口，
// 用它读写 tenant_id 可以避免对每个实体重复实现注入逻辑。
type tenantFieldMutation interface {
	Op() gen.Op
	Field(string) (ent.Value, bool)
	SetField(string, ent.Value) error
}

// FilterTenantRule 对嵌入 TenantID mixin 的实体施加租户隔离。
func FilterTenantRule() privacy.QueryMutationRule {
	return tenantRule{}
}

type tenantRule struct{}

func (tenantRule) EvalQuery(ctx context.Context, q ent.Query) error {
	if viewer.IsSystem(ctx) {
		return privacy.Allow
	}
	tenantID, ok := viewer.TenantID(ctx)
	if !ok {
		return privacy.Denyf("missing tenant context")
	}
	return gen.FilterTenantQuery(q, tenantID)
}

func (tenantRule) EvalMutation(ctx context.Context, m ent.Mutation) error {
	if viewer.IsSystem(ctx) {
		return privacy.Allow
	}
	tenantID, ok := viewer.TenantID(ctx)
	if !ok {
		return privacy.Denyf("missing tenant context")
	}
	tm, ok := m.(tenantFieldMutation)
	if !ok {
		return privacy.Denyf("unexpected tenant mutation type %T", m)
	}
	if tm.Op().Is(gen.OpCreate) {
		if value, exists := tm.Field(tenantIDField); exists {
			current, ok := value.(uint32)
			if !ok || current != tenantID {
				return privacy.Denyf("tenant_id mutation mismatch")
			}
			return privacy.Skip
		}
		if err := tm.SetField(tenantIDField, tenantID); err != nil {
			return privacy.Denyf("setting tenant_id: %v", err)
		}
		return privacy.Skip
	}
	return gen.FilterTenantMutation(m, tenantID)
}
