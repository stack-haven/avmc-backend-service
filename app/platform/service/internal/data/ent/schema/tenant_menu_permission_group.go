package schema

import (
	"backend-service/app/platform/service/internal/data/ent/mixins"
	pkgMixin "backend-service/pkg/entgo/mixin"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// TenantMenuPermissionGroup holds the schema definition for reusable tenant menu permission groups.
type TenantMenuPermissionGroup struct {
	ent.Schema
}

func (TenantMenuPermissionGroup) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "system_tenant_menu_permission_groups",
		},
		entsql.WithComments(true),
		schema.Comment("租户套餐表"),
	}
}

// Fields of the TenantMenuPermissionGroup.
func (TenantMenuPermissionGroup) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Comment("套餐名称").MaxLen(50).Default("").NotEmpty(),
		field.String("code").Comment("套餐编码").MaxLen(64).Default("").NotEmpty(),
		field.Bool("is_system").Comment("是否系统内置").Default(false).Nillable(),
		field.Int32("sort").Comment("排序").Default(10).SchemaType(map[string]string{dialect.MySQL: "int", dialect.Postgres: "int"}).Nillable(),
		field.String("description").Comment("描述").MaxLen(255).Default("").Nillable(),
		field.String("remark").Comment("备注").MaxLen(255).Default("").Nillable(),
		field.Uint32("current_version_id").Comment("当前发布版本ID").Optional().Nillable(),
		field.Strings("api_permissions").Comment("接口能力权限码列表").Optional(),
		field.JSON("feature_flags", map[string]bool{}).Comment("功能开关配置").Optional(),
		field.JSON("resource_quotas", map[string]int64{}).Comment("资源额度配置").Optional(),
	}
}

// Edges of the TenantMenuPermissionGroup.
func (TenantMenuPermissionGroup) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("menus", Menu.Type).StorageKey(edge.Table("system_tenant_menu_permission_group_menus")).Comment("租户套餐菜单关联表"),
		edge.To("current_version", TenantMenuPermissionGroupVersion.Type).
			Field("current_version_id").
			Unique(),
		edge.From("versions", TenantMenuPermissionGroupVersion.Type).
			Ref("group"),
		edge.From("tenants", Tenant.Type).
			Ref("groups"),
	}
}

// Mixin of the TenantMenuPermissionGroup.
func (TenantMenuPermissionGroup) Mixin() []ent.Mixin {
	return []ent.Mixin{
		pkgMixin.ID{},
		pkgMixin.Status{},
		pkgMixin.CreatedAt{},
		pkgMixin.UpdatedAt{},
		mixins.SoftDeleteMixin{},
	}
}

// Indexes of the TenantMenuPermissionGroup.
func (TenantMenuPermissionGroup) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name").Unique(),
		index.Fields("code").Unique(),
		index.Fields("status"),
	}
}
