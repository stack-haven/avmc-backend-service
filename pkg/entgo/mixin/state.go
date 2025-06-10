package mixin

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

var _ ent.Mixin = (*State)(nil)

type State struct {
	mixin.Schema
}

func (State) Fields() []ent.Field {
	return []ent.Field{
		field.Int32("state").
			Comment("状态 0 UNSPECIFIED 开启 1 -> ACTIVE 关闭 2 -> INACTIVE, 禁用 3 -> BANNED").
			Default(1).
			NonNegative().
			SchemaType(map[string]string{
				dialect.MySQL: "tinyint(2)",
			}).
			Nillable(),
	}
}
