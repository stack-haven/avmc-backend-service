package mixin

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

var _ ent.Mixin = (*Switch)(nil)

type Switch struct {
	mixin.Schema
}

func (Switch) Fields() []ent.Field {
	return []ent.Field{
		field.Int32("switch").
			Comment("开关").
			Default(1).
			Optional().
			SchemaType(map[string]string{
				dialect.MySQL: "tinyint(1)",
			}).
			Nillable(),
	}
}
