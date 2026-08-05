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

// TenantMenuPermissionGroupVersion holds an immutable menu snapshot for one package.
type TenantMenuPermissionGroupVersion struct {
	ent.Schema
}

func (TenantMenuPermissionGroupVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin", Table: "system_tenant_menu_permission_group_versions"},
		entsql.WithComments(true),
		schema.Comment("租户套餐版本表"),
	}
}

func (TenantMenuPermissionGroupVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("group_id").Comment("套餐ID").Positive().
			SchemaType(map[string]string{dialect.MySQL: "bigint", dialect.Postgres: "bigint"}),
		field.Int32("version").Comment("套餐内递增版本号").Positive(),
		field.Int32("state").Comment("版本状态：1已发布 2已取代").Default(1),
		field.String("change_summary").Comment("变更说明").MaxLen(255).Default("").Nillable(),
		field.Uint32("created_by").Comment("创建人ID").Optional().Nillable(),
		field.Uint32("published_by").Comment("发布人ID").Optional().Nillable(),
		field.Time("effective_at").Comment("生效时间").Optional().Nillable(),
		field.Time("published_at").Comment("发布时间").Optional().Nillable(),
		field.Strings("api_permissions").Comment("接口能力权限码快照").Optional(),
		field.JSON("feature_flags", map[string]bool{}).Comment("功能开关配置快照").Optional(),
		field.JSON("resource_quotas", map[string]int64{}).Comment("资源额度配置快照").Optional(),
	}
}

func (TenantMenuPermissionGroupVersion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("group", TenantMenuPermissionGroup.Type).
			Field("group_id").
			Unique().
			Required(),
		edge.To("menus", Menu.Type).StorageKey(edge.Table("system_tenant_menu_permission_group_version_menus")).Comment("租户套餐版本菜单关联表"),
	}
}

func (TenantMenuPermissionGroupVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{
		pkgMixin.ID{},
		pkgMixin.CreatedAt{},
	}
}

func (TenantMenuPermissionGroupVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("group_id", "version").Unique(),
		index.Fields("group_id", "state"),
		index.Fields("effective_at"),
	}
}
