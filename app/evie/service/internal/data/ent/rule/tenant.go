package rule

import (
	"context"

	"backend-service/app/evie/service/internal/data/ent/gen"
	"backend-service/app/evie/service/internal/data/ent/gen/privacy"
	"backend-service/app/evie/service/internal/data/ent/viewer"

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

// FilterTenantRule enforces tenant isolation for every schema that embeds the
// TenantID mixin. It filters read/update/delete mutations and injects tenant_id
// on create mutations.
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

func filterTenantQuery(q ent.Query, tenantID uint32) error {
	switch q.(type) {
	case *gen.DictionaryQuery:
		q.(*gen.DictionaryQuery).Filter().WhereTenantID(tenantScope(tenantID))
	case *gen.DictionaryEntryQuery:
		q.(*gen.DictionaryEntryQuery).Filter().WhereTenantID(tenantScope(tenantID))
	case *gen.DictionaryRelationQuery:
		q.(*gen.DictionaryRelationQuery).Filter().WhereTenantID(tenantScope(tenantID))
	case *gen.DictionaryCategoryQuery:
		q.(*gen.DictionaryCategoryQuery).Filter().WhereTenantID(tenantScope(tenantID))
	case *gen.DictionaryVersionQuery:
		q.(*gen.DictionaryVersionQuery).Filter().WhereTenantID(tenantScope(tenantID))
	case *gen.DictionaryConflictQuery:
		q.(*gen.DictionaryConflictQuery).Filter().WhereTenantID(tenantScope(tenantID))
	case *gen.DictionaryChangeLogQuery:
		q.(*gen.DictionaryChangeLogQuery).Filter().WhereTenantID(tenantScope(tenantID))
	case *gen.EnhancementPolicyQuery:
		q.(*gen.EnhancementPolicyQuery).Filter().WhereTenantID(tenantScope(tenantID))
	case *gen.EnhancementProfileQuery:
		q.(*gen.EnhancementProfileQuery).Filter().WhereTenantID(tenantScope(tenantID))
	case *gen.EnhancementLogQuery:
		q.(*gen.EnhancementLogQuery).Filter().WhereTenantID(tenantScope(tenantID))
	case *gen.AsrRecordQuery:
		q.(*gen.AsrRecordQuery).Filter().WhereTenantID(tenantScope(tenantID))
	case *gen.AsrProviderConfigQuery:
		q.(*gen.AsrProviderConfigQuery).Filter().WhereTenantID(tenantScope(tenantID))
	default:
		return privacy.Denyf("unexpected tenant query type %T", q)
	}
	return privacy.Skip
}

func filterTenantMutation(m ent.Mutation, tenantID uint32) error {
	switch m.(type) {
	case *gen.DictionaryMutation:
		m.(*gen.DictionaryMutation).WhereP(tenantFieldEQ(tenantID))
	case *gen.DictionaryEntryMutation:
		m.(*gen.DictionaryEntryMutation).WhereP(tenantFieldEQ(tenantID))
	case *gen.DictionaryRelationMutation:
		m.(*gen.DictionaryRelationMutation).WhereP(tenantFieldEQ(tenantID))
	case *gen.DictionaryCategoryMutation:
		m.(*gen.DictionaryCategoryMutation).WhereP(tenantFieldEQ(tenantID))
	case *gen.DictionaryVersionMutation:
		m.(*gen.DictionaryVersionMutation).WhereP(tenantFieldEQ(tenantID))
	case *gen.DictionaryConflictMutation:
		m.(*gen.DictionaryConflictMutation).WhereP(tenantFieldEQ(tenantID))
	case *gen.DictionaryChangeLogMutation:
		m.(*gen.DictionaryChangeLogMutation).WhereP(tenantFieldEQ(tenantID))
	case *gen.EnhancementPolicyMutation:
		m.(*gen.EnhancementPolicyMutation).WhereP(tenantFieldEQ(tenantID))
	case *gen.EnhancementProfileMutation:
		m.(*gen.EnhancementProfileMutation).WhereP(tenantFieldEQ(tenantID))
	case *gen.EnhancementLogMutation:
		m.(*gen.EnhancementLogMutation).WhereP(tenantFieldEQ(tenantID))
	case *gen.AsrRecordMutation:
		m.(*gen.AsrRecordMutation).WhereP(tenantFieldEQ(tenantID))
	case *gen.AsrProviderConfigMutation:
		m.(*gen.AsrProviderConfigMutation).WhereP(tenantFieldEQ(tenantID))
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

// tenantScope 返回租户可见范围：当前租户数据 或 平台级全局数据（tenant_id=0）。
// 用于查询过滤：内置分类等全局数据（tenant_id=0）对所有租户可见，租户数据严格隔离。
func tenantScope(tenantID uint32) entql.Uint32P {
	return entql.Uint32Or(entql.Uint32EQ(tenantID), entql.Uint32EQ(0))
}
