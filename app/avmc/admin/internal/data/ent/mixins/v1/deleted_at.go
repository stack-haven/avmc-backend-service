package v1

import (
	"context"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

// DeletedAtMixin a mixin that adds soft delete capabilities to a schema.
type DeletedAtMixin struct {
	mixin.Schema
}

// Fields of the DeletedAtMixin.
func (DeletedAtMixin) Fields() []ent.Field {
	return []ent.Field{
		// 关键字段：deleted_at，类型为时间，允许为 nil
		field.Time("deleted_at").
			Optional().
			Nillable().
			Comment("删除时间"),
	}
}

// Hooks of the DeletedAtMixin.
// 这是实现自动软删除的关键！
func (d DeletedAtMixin) Hooks() []ent.Hook {
	return []ent.Hook{
		// 定义一个钩子，它会在 OpDelete 和 OpDeleteOne 操作上触发
		hookOn(d.softDeleteHook, ent.OpDelete|ent.OpDeleteOne),
	}
}

// softDeleteHook is the hook that sets the "deleted_at" field instead of deleting the row.
func (d DeletedAtMixin) softDeleteHook(next ent.Mutator) ent.Mutator {
	// ent.MutateFunc 是一个适配器，允许我们将普通函数用作 Mutator
	return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
		// 检查此 schema 是否有 "deleted_at" 字段
		// if _, ok := m.Type().Fields["deleted_at"]; !ok {
		// 	return next.Mutate(ctx, m)
		// }
		// 获取查询条件
		mx, ok := m.(interface {
			OldDeletedAt(context.Context) (time.Time, error)
			SetOp(ent.Op)
			SetDeletedAt(time.Time)
		})
		if !ok {
			return next.Mutate(ctx, m)
		}

		// 将操作类型从 Delete* 强制改为 Update
		mx.SetOp(ent.OpUpdate)
		// 设置 deleted_at 字段为当前时间
		mx.SetDeletedAt(time.Now())
		return next.Mutate(ctx, m)
	})
}

// hookOn is a helper function to create a hook that runs only on specific operations.
func hookOn(h ent.Hook, op ent.Op) ent.Hook {
	return func(next ent.Mutator) ent.Mutator {
		return ent.MutateFunc(func(ctx context.Context, m ent.Mutation) (ent.Value, error) {
			// 如果当前操作不是我们指定的类型，则跳过钩子
			if !m.Op().Is(op) {
				return next.Mutate(ctx, m)
			}
			// 否则，执行我们的钩子逻辑
			return h(next).Mutate(ctx, m)
		})
	}
}
