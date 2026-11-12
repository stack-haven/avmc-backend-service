//go:build !ent_bootstrap
// +build !ent_bootstrap

package rule

import (
	"context"

	"entgo.io/ent"
	"entgo.io/ent/privacy"

	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/app/platform/service/internal/data/ent/gen/dictionarytype"
	"backend-service/app/platform/service/internal/data/ent/gen/hook"
	"backend-service/pkg/entgo/viewer"
)

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
