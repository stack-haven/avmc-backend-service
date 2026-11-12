package mixins

import (
	"context"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

// SoftDeleteMixin 软删除：所有删除操作改为写 deleted_at，查询默认过滤已删除行。
//
// 注意（沿用 evie 的既有约定）：
//  1. 不能合并进 BaseMixin，否则拦截器/钩子生成不出来，并会引发循环依赖；
//  2. 每个实体只能使用一次。
type SoftDeleteMixin struct {
	mixin.Schema
}

func (SoftDeleteMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Time("deleted_at").
			Optional().
			Nillable().
			Comment("删除时间"),
	}
}

type softDeleteKey struct{}

// SkipSoftDelete 返回跳过软删除过滤的上下文。
func SkipSoftDelete(parent context.Context) context.Context {
	return context.WithValue(parent, softDeleteKey{}, true)
}

func (d SoftDeleteMixin) P(w interface{ WhereP(...func(*sql.Selector)) }) {
	w.WhereP(sql.FieldIsNull(d.Fields()[0].Descriptor().Name))
}
