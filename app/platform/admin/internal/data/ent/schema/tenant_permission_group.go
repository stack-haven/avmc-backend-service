package schema

import (
	pkgMixin "backend-service/pkg/entgo/mixin"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TenantPermissionGroup holds the schema definition for tenant to menu permission group bindings.
type TenantPermissionGroup struct {
	ent.Schema
}

func (TenantPermissionGroup) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("租户菜单权限组绑定表"),
	}
}

// Fields of the TenantPermissionGroup.
func (TenantPermissionGroup) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("tenant_id").
			Comment("租户ID").
			Positive().
			SchemaType(map[string]string{dialect.MySQL: "bigint", dialect.Postgres: "bigint"}),
		field.Uint32("group_id").
			Comment("权限组ID").
			Positive().
			SchemaType(map[string]string{dialect.MySQL: "bigint", dialect.Postgres: "bigint"}),
		field.Bool("enabled").Comment("是否启用").Default(true).Nillable(),
		field.Uint32("bound_by").Comment("绑定操作人ID").Optional().Nillable(),
	}
}

// Edges of the TenantPermissionGroup.
func (TenantPermissionGroup) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("tenant", Tenant.Type).
			Field("tenant_id").
			Unique().
			Required(),
		edge.To("group", MenuPermissionGroup.Type).
			Field("group_id").
			Unique().
			Required(),
	}
}

// Mixin of the TenantPermissionGroup.
func (TenantPermissionGroup) Mixin() []ent.Mixin {
	return []ent.Mixin{
		pkgMixin.ID{},
		pkgMixin.CreatedAt{},
		pkgMixin.UpdatedAt{},
	}
}

// Indexes of the TenantPermissionGroup.
func (TenantPermissionGroup) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "group_id").Unique(),
		index.Fields("tenant_id"),
		index.Fields("group_id"),
	}
}
