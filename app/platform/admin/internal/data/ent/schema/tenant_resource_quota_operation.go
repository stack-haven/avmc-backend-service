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

// TenantResourceQuotaOperation records idempotent resource quota mutations.
type TenantResourceQuotaOperation struct {
	ent.Schema
}

func (TenantResourceQuotaOperation) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin"},
		entsql.WithComments(true),
		schema.Comment("租户资源额度操作幂等流水表"),
	}
}

func (TenantResourceQuotaOperation) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("tenant_id").
			Comment("租户ID").
			Positive().
			SchemaType(map[string]string{dialect.MySQL: "bigint", dialect.Postgres: "bigint"}),
		field.String("resource_key").
			Comment("资源额度键").
			MaxLen(100).
			NotEmpty(),
		field.String("operation_type").
			Comment("操作类型: consume/release").
			MaxLen(20).
			NotEmpty(),
		field.String("idempotency_key").
			Comment("业务幂等键").
			MaxLen(120).
			NotEmpty(),
		field.Int64("amount").
			Comment("操作额度数量").
			Positive().
			SchemaType(map[string]string{dialect.MySQL: "bigint", dialect.Postgres: "bigint"}),
		field.Int64("used_after").
			Comment("操作完成后的已使用额度").
			Min(0).
			SchemaType(map[string]string{dialect.MySQL: "bigint", dialect.Postgres: "bigint"}),
		field.Uint32("updated_by").Comment("操作人ID").Optional().Nillable(),
	}
}

func (TenantResourceQuotaOperation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("tenant", Tenant.Type).
			Field("tenant_id").
			Unique().
			Required(),
	}
}

func (TenantResourceQuotaOperation) Mixin() []ent.Mixin {
	return []ent.Mixin{
		pkgMixin.ID{},
		pkgMixin.CreatedAt{},
		pkgMixin.UpdatedAt{},
	}
}

func (TenantResourceQuotaOperation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "operation_type", "idempotency_key").Unique(),
		index.Fields("tenant_id", "resource_key"),
		index.Fields("idempotency_key"),
	}
}
