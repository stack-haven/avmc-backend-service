package mixins

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
)

var _ ent.Mixin = (*DomainID)(nil)

type DomainID struct {
	mixin.Schema
}

func (DomainID) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("domain_id").
			Comment("域ID").
			Positive().
			StructTag(`json:"domain_id,omitempty"`).
			SchemaType(map[string]string{
				dialect.MySQL:    "bigint",
				dialect.Postgres: "bigint",
			}),
	}
}

func (DomainID) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("domain_id"),
	}
}
