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

// Dept holds the schema definition for the Dept entity.
type Dept struct {
	ent.Schema
}

func (Dept) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("部门表"),
	}
}

// Fields of the Dept.
func (Dept) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Comment("名称").MaxLen(128).Nillable(),
		field.Uint32("parent_id").Comment("父级ID").Default(0).Optional().Nillable(),
		field.Uint32("leader_id").Comment("负责人ID").Default(0).Optional().Nillable(),
		field.Ints("ancestors").Comment("祖级列表").Default([]int{}).Optional(),
		field.Int32("sort").Comment("排序").Default(10).Nillable(),
		field.String("remark").Comment("备注").MaxLen(255).Default("").Optional().Nillable(),
	}
}

// Edges of the Dept.
func (Dept) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("children", Dept.Type).
			From("parent").
			Unique().
			Field("parent_id"),
	}
}

// Mixin of the Dept.
func (Dept) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

// Indexes of the Dept.
func (Dept) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "name").Unique(),
	}
}
