package rule

import (
	"context"

	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/dictionarytype"
	"backend-service/app/platform/admin/internal/data/ent/gen/hook"
	"backend-service/app/platform/admin/internal/data/ent/gen/privacy"
	"backend-service/app/platform/admin/internal/data/ent/viewer"

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
	switch q := q.(type) {
	case *gen.DictionaryTypeQuery:
		q.Filter().WhereTenantID(entql.Uint32EQ(tenantID))
	case *gen.DictionaryItemQuery:
		q.Filter().WhereTenantID(entql.Uint32EQ(tenantID))
	case *gen.OperationLogQuery:
		q.Filter().WhereTenantID(entql.Uint32EQ(tenantID))
	case *gen.LoginLogQuery:
		q.Filter().WhereTenantID(entql.Uint32EQ(tenantID))
	case *gen.DeptQuery:
		q.Filter().WhereTenantID(entql.Uint32EQ(tenantID))
	case *gen.PostQuery:
		q.Filter().WhereTenantID(entql.Uint32EQ(tenantID))
	case *gen.ProjectQuery:
		q.Filter().WhereTenantID(entql.Uint32EQ(tenantID))
	case *gen.RoleQuery:
		q.Filter().WhereTenantID(entql.Uint32EQ(tenantID))
	case *gen.TenantParameterOverrideQuery:
		q.Filter().WhereTenantID(entql.Uint32EQ(tenantID))
	case *gen.UserQuery:
		q.Filter().WhereTenantID(entql.Uint32EQ(tenantID))
	default:
		return privacy.Denyf("unexpected tenant query type %T", q)
	}
	return privacy.Skip
}

func filterTenantMutation(m ent.Mutation, tenantID uint32) error {
	switch m := m.(type) {
	case *gen.DictionaryTypeMutation:
		m.WhereP(tenantFieldEQ(tenantID))
	case *gen.DictionaryItemMutation:
		m.WhereP(tenantFieldEQ(tenantID))
	case *gen.OperationLogMutation:
		m.WhereP(tenantFieldEQ(tenantID))
	case *gen.LoginLogMutation:
		m.WhereP(tenantFieldEQ(tenantID))
	case *gen.DeptMutation:
		m.WhereP(tenantFieldEQ(tenantID))
	case *gen.PostMutation:
		m.WhereP(tenantFieldEQ(tenantID))
	case *gen.ProjectMutation:
		m.WhereP(tenantFieldEQ(tenantID))
	case *gen.RoleMutation:
		m.WhereP(tenantFieldEQ(tenantID))
	case *gen.TenantParameterOverrideMutation:
		m.WhereP(tenantFieldEQ(tenantID))
	case *gen.UserMutation:
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

// DictionaryItemTenantHook prevents a dictionary item from referencing a type
// owned by another tenant. The tenant_id privacy predicate alone cannot protect
// a raw foreign-key assignment.
func DictionaryItemTenantHook() ent.Hook {
	return hook.On(
		func(next ent.Mutator) ent.Mutator {
			return ent.MutateFunc(func(ctx context.Context, mutation ent.Mutation) (ent.Value, error) {
				if viewer.IsSystem(ctx) {
					return next.Mutate(ctx, mutation)
				}
				itemMutation, ok := mutation.(*gen.DictionaryItemMutation)
				if !ok {
					return nil, privacy.Denyf("unexpected dictionary item mutation type %T", mutation)
				}
				typeID, changed := itemMutation.TypeID()
				if !changed {
					return next.Mutate(ctx, mutation)
				}
				exists, err := itemMutation.Client().DictionaryType.Query().
					Where(dictionarytype.IDEQ(typeID)).
					Exist(ctx)
				if err != nil {
					return nil, err
				}
				if !exists {
					return nil, privacy.Denyf("dictionary type does not belong to current tenant")
				}
				return next.Mutate(ctx, mutation)
			})
		},
		ent.OpCreate|ent.OpUpdate|ent.OpUpdateOne,
	)
}
