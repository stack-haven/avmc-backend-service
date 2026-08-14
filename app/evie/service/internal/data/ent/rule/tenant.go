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
	case *gen.DictionaryWordQuery:
		q.(*gen.DictionaryWordQuery).Filter().WhereTenantID(entql.Uint32EQ(tenantID))
	case *gen.DictionaryAliasQuery:
		q.(*gen.DictionaryAliasQuery).Filter().WhereTenantID(entql.Uint32EQ(tenantID))
	case *gen.AsrRecordQuery:
		q.(*gen.AsrRecordQuery).Filter().WhereTenantID(entql.Uint32EQ(tenantID))
	case *gen.HotwordQuery:
		q.(*gen.HotwordQuery).Filter().WhereTenantID(entql.Uint32EQ(tenantID))
	case *gen.AsrProviderConfigQuery:
		q.(*gen.AsrProviderConfigQuery).Filter().WhereTenantID(entql.Uint32EQ(tenantID))
	case *gen.CorrectionRuleQuery:
		q.(*gen.CorrectionRuleQuery).Filter().WhereTenantID(entql.Uint32EQ(tenantID))
	case *gen.CorrectionLogQuery:
		q.(*gen.CorrectionLogQuery).Filter().WhereTenantID(entql.Uint32EQ(tenantID))
	default:
		return privacy.Denyf("unexpected tenant query type %T", q)
	}
	return privacy.Skip
}

func filterTenantMutation(m ent.Mutation, tenantID uint32) error {
	switch m.(type) {
	case *gen.DictionaryWordMutation:
		m.(*gen.DictionaryWordMutation).WhereP(tenantFieldEQ(tenantID))
	case *gen.DictionaryAliasMutation:
		m.(*gen.DictionaryAliasMutation).WhereP(tenantFieldEQ(tenantID))
	case *gen.AsrRecordMutation:
		m.(*gen.AsrRecordMutation).WhereP(tenantFieldEQ(tenantID))
	case *gen.HotwordMutation:
		m.(*gen.HotwordMutation).WhereP(tenantFieldEQ(tenantID))
	case *gen.AsrProviderConfigMutation:
		m.(*gen.AsrProviderConfigMutation).WhereP(tenantFieldEQ(tenantID))
	case *gen.CorrectionRuleMutation:
		m.(*gen.CorrectionRuleMutation).WhereP(tenantFieldEQ(tenantID))
	case *gen.CorrectionLogMutation:
		m.(*gen.CorrectionLogMutation).WhereP(tenantFieldEQ(tenantID))
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
