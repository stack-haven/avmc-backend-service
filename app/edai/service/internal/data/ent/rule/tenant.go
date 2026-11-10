package rule

import (
	"context"

	"backend-service/app/edai/service/internal/data/ent/gen"
	"backend-service/app/edai/service/internal/data/ent/gen/privacy"
	"backend-service/app/edai/service/internal/data/ent/viewer"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/entql"
)

const tenantIDField = "tenant_id"

type tenantFieldMutation interface {
	Op() gen.Op
	Field(string) (ent.Value, bool)
	SetField(string, ent.Value) error
}

// FilterTenantRule 为所有内嵌 TenantID mixin 的 schema 提供租户隔离：
// 读/改/删按 tenant_id 过滤，创建时自动注入 tenant_id。
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
	return filterTenantQuery(q, tenantID)
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
	return filterTenantMutation(m, tenantID)
}

// 注意：EDAI 新增实体后必须同步扩充这两个 switch，否则该实体的
// 查询/变更会被 privacy.Denyf 拒绝（fail-closed，不会静默放行）。
func filterTenantQuery(q ent.Query, tenantID uint32) error {
	switch q := q.(type) {
	case *gen.UserQuery:
		q.Filter().WhereTenantID(tenantScope(tenantID))
	case *gen.DataSourceQuery:
		q.Filter().WhereTenantID(tenantScope(tenantID))
	case *gen.FileRecordQuery:
		q.Filter().WhereTenantID(tenantScope(tenantID))
	case *gen.ScanTaskQuery:
		q.Filter().WhereTenantID(tenantScope(tenantID))
	default:
		return privacy.Denyf("unexpected tenant query type %T", q)
	}
	return privacy.Skip
}

func filterTenantMutation(m ent.Mutation, tenantID uint32) error {
	switch m := m.(type) {
	case *gen.UserMutation:
		m.WhereP(tenantFieldEQ(tenantID))
	case *gen.DataSourceMutation:
		m.WhereP(tenantFieldEQ(tenantID))
	case *gen.FileRecordMutation:
		m.WhereP(tenantFieldEQ(tenantID))
	case *gen.ScanTaskMutation:
		m.WhereP(tenantFieldEQ(tenantID))
	default:
		return privacy.Denyf("unexpected tenant mutation type %T", m)
	}
	return privacy.Skip
}

func tenantFieldEQ(tenantID uint32) func(*sql.Selector) {
	return func(s *sql.Selector) {
		s.Where(sql.EQ(s.C(tenantIDField), tenantID))
	}
}

// tenantScope 返回租户可见范围：本租户数据 + 平台级全局数据（tenant_id=0）。
func tenantScope(tenantID uint32) entql.Uint32P {
	return entql.Uint32Or(entql.Uint32EQ(tenantID), entql.Uint32EQ(0))
}
