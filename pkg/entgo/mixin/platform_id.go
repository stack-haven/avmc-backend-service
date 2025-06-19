package mixin

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
)

var _ ent.Mixin = (*PlatformID)(nil)

type PlatformID struct {
	mixin.Schema
}

func (PlatformID) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("platform_id").
			Comment("平台ID").
			DefaultFunc(NewSnowflakeID().Uint32).
			Positive().
			StructTag(`json:"platform_id,omitempty"`).
			SchemaType(map[string]string{
				dialect.MySQL:    "bigint",
				dialect.Postgres: "bigint",
			}),
	}
}

// Indexes of the PlatformID.
func (PlatformID) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("platform_id"),
	}
}
