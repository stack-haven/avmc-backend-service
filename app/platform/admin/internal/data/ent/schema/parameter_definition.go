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

// ParameterDefinition is a platform-owned runtime configuration definition.
type ParameterDefinition struct{ ent.Schema }

func (ParameterDefinition) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin"},
		entsql.WithComments(true),
		schema.Comment("平台参数定义表"),
	}
}

func (ParameterDefinition) Fields() []ent.Field {
	return []ent.Field{
		field.String("key").MaxLen(100).NotEmpty().Comment("参数键"),
		field.String("name").MaxLen(100).NotEmpty().Comment("参数名称"),
		field.Int32("value_type").Comment("值类型：1字符串 2整数 3布尔 4JSON"),
		field.Text("default_value").Default("").Comment("平台默认值"),
		field.String("description").MaxLen(500).Default("").Comment("参数说明"),
		field.Bool("tenant_overridable").Default(true).Comment("是否允许租户覆盖"),
		field.Int32("status").Default(1).Comment("状态"),
		field.Int32("sort").Default(10).Comment("排序"),
	}
}

func (ParameterDefinition) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("tenant_overrides", TenantParameterOverride.Type),
	}
}

func (ParameterDefinition) Mixin() []ent.Mixin {
	return []ent.Mixin{
		pkgMixin.ID{},
		pkgMixin.CreatedAt{},
		pkgMixin.UpdatedAt{},
		mixins.SoftDeleteMixin{},
	}
}

func (ParameterDefinition) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("key").Unique(),
		index.Fields("status", "sort"),
	}
}
