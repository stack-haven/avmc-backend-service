package schema

import (
	"backend-service/app/platform/admin/internal/data/ent/mixins"
	pkgMixin "backend-service/pkg/entgo/mixin"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TenantParameterOverride stores one tenant-specific parameter value.
type TenantParameterOverride struct{ ent.Schema }

func (TenantParameterOverride) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin", Table: "system_tenant_parameter_overrides"},
		entsql.WithComments(true),
		schema.Comment("租户参数覆盖表"),
	}
}

func (TenantParameterOverride) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("definition_id").Positive().Comment("参数定义ID"),
		field.Text("value").Default("").Comment("租户覆盖值"),
		field.Uint32("updated_by").Optional().Nillable().Comment("最后修改人ID"),
	}
}

func (TenantParameterOverride) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("definition", ParameterDefinition.Type).
			Ref("tenant_overrides").
			Field("definition_id").
			Unique().
			Required(),
	}
}

func (TenantParameterOverride) Mixin() []ent.Mixin {
	return []ent.Mixin{
		pkgMixin.ID{},
		pkgMixin.CreatedAt{},
		pkgMixin.UpdatedAt{},
		mixins.TenantID{},
	}
}

func (TenantParameterOverride) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "definition_id").Unique(),
		index.Fields("definition_id"),
	}
}
