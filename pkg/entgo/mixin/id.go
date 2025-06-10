package mixin

import (
	"log"
	"math/rand"

	"entgo.io/contrib/entproto"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
	"github.com/bwmarrin/snowflake"
)

var _ ent.Mixin = (*Id)(nil)

type Id struct{ mixin.Schema }

func (Id) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("id").
			Comment("id").
			StructTag(`json:"id,omitempty"`).
			SchemaType(map[string]string{
				dialect.MySQL:    "bigint",
				dialect.Postgres: "serial",
			}).
			Annotations(
				entproto.Field(1),
			).
			Positive().
			Immutable().
			Unique(),
	}
}

// Indexes of the Id.
func (Id) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("id"),
	}
}

var _ ent.Mixin = (*SnowflackId)(nil)

var nodeId = rand.Int63n(9000)

func NewSnowflakeId() snowflake.ID {
	sf, err := snowflake.NewNode(nodeId)
	if err != nil {
		log.Fatalf("snowflake.NewNode(%d): %s", nodeId, err)
		return 0
	}

	return sf.Generate()
}

type SnowflackId struct {
	mixin.Schema
}

func (SnowflackId) Fields() []ent.Field {
	return []ent.Field{
		field.Uint64("id").
			Comment("id").
			DefaultFunc(NewSnowflakeId().Int64).
			Positive().
			Immutable().
			StructTag(`json:"id,omitempty"`).
			SchemaType(map[string]string{
				dialect.MySQL:    "bigint",
				dialect.Postgres: "bigint",
			}),
	}
}

// Indexes of the snowflackId.
func (SnowflackId) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("id"),
	}
}
