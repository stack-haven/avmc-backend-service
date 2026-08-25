package mixins

import (
	"backend-service/app/evie/service/internal/data/ent/rule"

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
			Comment("租户ID（0=平台级全局共享，>0=租户隔离）").
			StructTag(`json:"tenant_id,omitempty"`).
			SchemaType(map[string]string{
				dialect.MySQL:    "bigint",
				dialect.Postgres: "bigint",
			}),
	}
}

func (TenantID) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id"),
	}
}

func (TenantID) Policy() ent.Policy {
	return rule.FilterTenantRule()
}
