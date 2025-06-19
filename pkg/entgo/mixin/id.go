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

var _ ent.Mixin = (*ID)(nil)

type ID struct{ mixin.Schema }

func (ID) Fields() []ent.Field {
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

// Indexes of the ID.
func (ID) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("id"),
	}
}

var _ ent.Mixin = (*SnowflakeID)(nil)

type SnowflakeID struct {
	mixin.Schema
	*snowflake.Node
}

func NewSnowflakeID() *SnowflakeID {
	newSnowflakeID := new(SnowflakeID)
	if err := newSnowflakeID.Init(); err != nil {
		log.Fatalf("snowflake.NewNode: %s", err)
		return nil
	}
	return newSnowflakeID
}

func (s *SnowflakeID) Init() error {
	if s.Node == nil {
		nodeID := rand.Int63n(1023)
		sf, err := snowflake.NewNode(nodeID)
		if err != nil {
			log.Fatalf("snowflake.NewNode(%d): %s", nodeID, err)
			return err
		}
		s.Node = sf
	}
	return nil
}

func (s SnowflakeID) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Comment("id").
			DefaultFunc(s.Uint32()).
			Positive().
			Immutable().
			StructTag(`json:"id,omitempty"`).
			SchemaType(map[string]string{
				dialect.MySQL:    "bigint",
				dialect.Postgres: "bigint",
			}),
	}
}

// Indexes of the snowflakeID.
func (SnowflakeID) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("id"),
	}
}

func (s SnowflakeID) Uint32() uint32 {
	if err := s.Init(); err != nil {
		log.Fatalf("snowflakeID.Uint32: %s", err)
		return 0
	}
	return uint32(s.Node.Generate())
}
