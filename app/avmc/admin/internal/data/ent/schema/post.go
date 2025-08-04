package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
)

// Post holds the schema definition for the Post entity.
type Post struct {
	ent.Schema
}

func (Post) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("岗位表"),
	}
}

// Fields of the Post.
func (Post) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Comment("名称").MaxLen(50).Default("").Nillable(),
	}
}

// Edges of the Post.
func (Post) Edges() []ent.Edge {
	return nil
}

// Mixin of the Post.
func (Post) Mixin() []ent.Mixin {
	return []ent.Mixin{
		MixinTop{},
	}
}

// Indexes of the Post.
func (Post) Indexes() []ent.Index {
	return []ent.Index{
		// index.Fields("phone", "authority").Unique(),
	}
}
