package mixin

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
)

var _ ent.Mixin = (*TenantId)(nil)

type TenantId struct {
	mixin.Schema
}

func (TenantId) Fields() []ent.Field {
	return []ent.Field{
		field.Uint64("tenant_id").
			Comment("租户ID").
			DefaultFunc(NewSnowflakeId().Int64).
			Positive().
			StructTag(`json:"tenant_id,omitempty"`).
			SchemaType(map[string]string{
				dialect.MySQL:    "bigint",
				dialect.Postgres: "bigint",
			}),
	}
}

// Indexes of the TenantId.
func (TenantId) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id"),
	}
}
