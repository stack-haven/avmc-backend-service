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

// TenantResourceQuotaUsage records generic per-tenant resource usage counters.
type TenantResourceQuotaUsage struct {
	ent.Schema
}

func (TenantResourceQuotaUsage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin"},
		entsql.WithComments(true),
		schema.Comment("租户资源额度使用量表"),
	}
}

func (TenantResourceQuotaUsage) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("tenant_id").
			Comment("租户ID").
			Positive().
			SchemaType(map[string]string{dialect.MySQL: "bigint", dialect.Postgres: "bigint"}),
		field.String("resource_key").
			Comment("资源额度键").
			MaxLen(100).
			NotEmpty(),
		field.Int64("used").
			Comment("已使用额度").
			Min(0).
			Default(0).
			SchemaType(map[string]string{dialect.MySQL: "bigint", dialect.Postgres: "bigint"}),
		field.Uint32("updated_by").Comment("最后更新人ID").Optional().Nillable(),
	}
}

func (TenantResourceQuotaUsage) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("tenant", Tenant.Type).
			Field("tenant_id").
			Unique().
			Required(),
	}
}

func (TenantResourceQuotaUsage) Mixin() []ent.Mixin {
	return []ent.Mixin{
		pkgMixin.ID{},
		pkgMixin.CreatedAt{},
		pkgMixin.UpdatedAt{},
	}
}

func (TenantResourceQuotaUsage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "resource_key").Unique(),
		index.Fields("tenant_id"),
		index.Fields("resource_key"),
	}
}
