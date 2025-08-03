package mixin

import (
	"backend-service/app/avmc/admin/internal/data/ent/hook"
	"backend-service/app/avmc/admin/internal/data/ent/intercept"
	"context"
	"fmt"
	"time"

	"backend-service/app/avmc/admin/internal/data/ent"

	entgo "entgo.io/ent"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

var _ entgo.Mixin = (*DeletedAt)(nil)

// DeletedAt implements the soft delete pattern for schemas.
type DeletedAt struct {
	mixin.Schema
}

// Fields of the DeletedAt.
func (DeletedAt) Fields() []entgo.Field {
	return []entgo.Field{
		// 删除时间
		field.Time("deleted_at").
			Optional().
			Nillable().
			Comment("删除时间"),
	}
}

type softDeleteKey struct{}

// SkipSoftDelete returns a new context that skips the soft-delete interceptor/mutators.
func SkipSoftDelete(parent context.Context) context.Context {
	return context.WithValue(parent, softDeleteKey{}, true)
}

// Interceptors of the DeletedAt.
func (d DeletedAt) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{
		intercept.TraverseFunc(func(ctx context.Context, q intercept.Query) error {
			// Skip soft-delete, means include soft-deleted entities.
			if skip, _ := ctx.Value(softDeleteKey{}).(bool); skip {
				return nil
			}
			d.P(q)
			return nil
		}),
	}
}

// Hooks of the DeletedAt.
func (d DeletedAt) Hooks() []ent.Hook {
	return []ent.Hook{
		hook.On(
			func(next ent.Mutator) ent.Mutator {
				return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
					// Skip soft-delete, means delete the entity permanently.
					if skip, _ := ctx.Value(softDeleteKey{}).(bool); skip {
						return next.Mutate(ctx, m)
					}
					mx, ok := m.(interface {
						SetOp(ent.Op)
						Client() *ent.Client
						SetDeleteTime(time.Time)
						WhereP(...func(*sql.Selector))
					})
					if !ok {
						return nil, fmt.Errorf("unexpected mutation type %T", m)
					}
					d.P(mx)
					mx.SetOp(ent.OpUpdate)
					mx.SetDeleteTime(time.Now())
					return mx.Client().Mutate(ctx, m)
				})
			},
			ent.OpDeleteOne|ent.OpDelete,
		),
	}
}

// P adds a storage-level predicate to the queries and mutations.
func (d DeletedAt) P(w interface{ WhereP(...func(*sql.Selector)) }) {
	w.WhereP(
		sql.FieldIsNull(d.Fields()[0].Descriptor().Name),
	)
}
