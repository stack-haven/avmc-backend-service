package mixins

import (
	"context"
	"fmt"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"

	// 替换为你的 ent 生成包路径
	"backend-service/app/avmc/admin/internal/data/ent/gen"
	"backend-service/app/avmc/admin/internal/data/ent/gen/hook"
	"backend-service/app/avmc/admin/internal/data/ent/gen/intercept"
)

// 注意：
// 1. 软删除Mixin会自动为所有实体添加deleted_at字段
// 2. 删除操作会将deleted_at设置为当前时间，而不是实际删除
// 3. 查询操作默认会过滤掉已软删除的实体
// 4. 可以使用SkipSoftDelete(ctx)上下文来跳过软删除过滤

// 重点：
// 1. 不能放到base中使用，否则无法自动生成拦截器，同时会导致循环依赖
// 2. 软删除Mixin只能在实体中使用一次

// 举例：
// ```go
// func (User) Mixin() []ent.Mixin {
// 	return []ent.Mixin{
// 		mixins.BaseMixin{},
//		# 软删除Mixin只能在实体中使用一次
//		# 不能放到base中使用，否则无法自动生成拦截器，同时会导致循环依赖
// 		mixins.SoftDeleteMixin{},
// 	}
// }
// ```

// SoftDeleteMixin 实现软删除模式
type SoftDeleteMixin struct {
	mixin.Schema
}

// Fields 定义软删除的字段
func (SoftDeleteMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Time("deleted_at").
			Optional().
			Nillable().
			Comment("删除时间"),
	}
}

type softDeleteKey struct{}

// SkipSoftDelete 返回一个跳过软删除的上下文
func SkipSoftDelete(parent context.Context) context.Context {
	return context.WithValue(parent, softDeleteKey{}, true)
}

// Interceptors 定义查询拦截器
func (d SoftDeleteMixin) Interceptors() []ent.Interceptor {
	return []ent.Interceptor{
		intercept.TraverseFunc(func(ctx context.Context, q intercept.Query) error {
			// 跳过软删除，包含已软删除的实体
			if skip, _ := ctx.Value(softDeleteKey{}).(bool); skip {
				return nil
			}
			d.P(q)
			return nil
		}),
	}
}

// Hooks 定义删除钩子
func (d SoftDeleteMixin) Hooks() []ent.Hook {
	return []ent.Hook{
		hook.On(
			func(next ent.Mutator) ent.Mutator {
				return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
					// 跳过软删除，执行硬删除
					if skip, _ := ctx.Value(softDeleteKey{}).(bool); skip {
						return next.Mutate(ctx, m)
					}
					mx, ok := m.(interface {
						SetOp(ent.Op)
						Client() *gen.Client // 替换为你的实际 Client 类型
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

// P 为查询和变更添加存储级别的谓词
func (d SoftDeleteMixin) P(w interface{ WhereP(...func(*sql.Selector)) }) {
	w.WhereP(
		sql.FieldIsNull(d.Fields()[0].Descriptor().Name),
	)
}
