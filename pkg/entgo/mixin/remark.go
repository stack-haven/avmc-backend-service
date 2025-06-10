package mixin

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

var _ ent.Mixin = (*Remark)(nil)

type Remark struct {
	mixin.Schema
}

func (Remark) Fields() []ent.Field {
	return []ent.Field{
		field.String("remark").
			Comment("备注").
			Default("").
			Optional().
			Nillable(),
	}
}
