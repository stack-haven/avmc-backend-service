package mixin

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
)

var _ ent.Mixin = (*TenantID)(nil)

type TenantID struct {
	mixin.Schema
}

func (TenantID) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("tenant_id").
			Comment("租户ID").
			DefaultFunc(NewSnowflakeID().Uint32).
			Positive().
			StructTag(`json:"tenant_id,omitempty"`).
			SchemaType(map[string]string{
				dialect.MySQL:    "bigint",
				dialect.Postgres: "bigint",
			}),
	}
}

// Indexes of the TenantID.
func (TenantID) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id"),
	}
}
