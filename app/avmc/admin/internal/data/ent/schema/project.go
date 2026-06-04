package schema

import (
	"backend-service/app/avmc/admin/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Project holds the schema definition for the Project entity.
type Project struct {
	ent.Schema
}

func (Project) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("项目表"),
	}
}

// Fields of the Project.
func (Project) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Comment("项目名称，域内唯一").MaxLen(50),
		field.String("code").Comment("项目标识，域内唯一").MaxLen(50).Optional().Nillable(),
		field.Uint32("owner_id").Comment("项目负责人ID").Optional().Nillable(),
		field.String("description").Comment("项目描述").MaxLen(500).Default("").Nillable(),
	}
}

// Edges of the Project.
func (Project) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("members", User.Type),
	}
}

// Mixin of the Project.
func (Project) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

// Indexes of the Project.
func (Project) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("domain_id", "name").Unique(),
		index.Fields("domain_id", "code").Unique(),
		index.Fields("owner_id"),
		index.Fields("status"),
	}
}
