package schema

import (
	"backend-service/app/evie/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// EnhancementProfile 增强场景：绑定一个增强策略（开发说明第十二节 EnhancementProfile）。
type EnhancementProfile struct {
	ent.Schema
}

func (EnhancementProfile) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "evie_enhancement_profiles",
		},
		entsql.WithComments(true),
		schema.Comment("文本增强场景表"),
	}
}

func (EnhancementProfile) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("policy_id").Positive().Comment("绑定策略 ID"),
		field.String("name").MaxLen(128).NotEmpty().Comment("场景名称"),
		field.String("description").MaxLen(512).Optional().Comment("描述"),
	}
}

func (EnhancementProfile) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("policy", EnhancementPolicy.Type).Ref("profiles").Field("policy_id").Unique().Required().Comment("绑定策略"),
	}
}

func (EnhancementProfile) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (EnhancementProfile) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "name").Unique(),
	}
}
