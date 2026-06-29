package schema

import (
	"backend-service/app/platform/admin/internal/data/ent/mixins"
	pkgMixin "backend-service/pkg/entgo/mixin"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Tenant holds the schema definition for platform tenants.
type Tenant struct {
	ent.Schema
}

func (Tenant) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("租户表"),
	}
}

// Fields of the Tenant.
func (Tenant) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Comment("租户名称").MaxLen(50).Default("").NotEmpty(),
		field.String("code").Comment("租户编码").MaxLen(64).Default("").NotEmpty(),
		field.Int32("sort").Comment("排序").Default(10).SchemaType(map[string]string{dialect.MySQL: "int", dialect.Postgres: "int"}).Nillable(),
		field.String("remark").Comment("备注").MaxLen(255).Default("").Nillable(),
		field.Int32("lifecycle_status").Comment("生命周期状态：1待开通 2正常 3暂停 4到期 5注销").Default(2),
		field.Time("activated_at").Comment("激活时间").Optional().Nillable(),
		field.Time("expires_at").Comment("到期时间").Optional().Nillable(),
		field.Time("suspended_at").Comment("暂停时间").Optional().Nillable(),
		field.Time("cancelled_at").Comment("注销时间").Optional().Nillable(),
	}
}

// Edges of the Tenant.
func (Tenant) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("permission_group_bindings", TenantPermissionGroup.Type).
			Ref("tenant"),
	}
}

// Mixin of the Tenant.
func (Tenant) Mixin() []ent.Mixin {
	return []ent.Mixin{
		pkgMixin.ID{},
		pkgMixin.Status{},
		pkgMixin.CreatedAt{},
		pkgMixin.UpdatedAt{},
		mixins.SoftDeleteMixin{},
	}
}

// Indexes of the Tenant.
func (Tenant) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").Unique(),
		index.Fields("code").Unique(),
		index.Fields("status"),
		index.Fields("lifecycle_status"),
		index.Fields("expires_at"),
	}
}
