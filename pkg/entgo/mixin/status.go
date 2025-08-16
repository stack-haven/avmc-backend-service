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
			Comment("状态：0=未知 1=启用 2=禁用"),
	}
}
