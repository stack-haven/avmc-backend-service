package mixin

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

var _ ent.Mixin = (*Status)(nil)

type Status struct {
	mixin.Schema
}

func (Status) Fields() []ent.Field {
	return []ent.Field{
		field.Int32("status").
			Max(99).
			Min(0).
			Default(1).
			SchemaType(map[string]string{
				dialect.MySQL:    "tinyint(2)",
				dialect.Postgres: "tinyint(2)",
			}).
			Nillable().
			Comment("状态：0=未知 1=正常 2=禁用 3=锁定 4=已删除"),
	}
}

var _ ent.Mixin = (*State)(nil)

type State struct {
	mixin.Schema
}

func (State) Fields() []ent.Field {
	return []ent.Field{
		field.Int32("state").
			Default(1).
			NonNegative().
			SchemaType(map[string]string{
				dialect.MySQL:    "tinyint(2)",
				dialect.Postgres: "tinyint(2)",
			}).
			Nillable().
			Comment("状态 0 UNSPECIFIED 开启 1 -> ACTIVE 关闭 2 -> INACTIVE, 禁用 3 -> BANNED"),
	}
}
