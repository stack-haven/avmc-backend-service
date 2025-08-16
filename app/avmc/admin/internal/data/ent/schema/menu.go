package schema

import (
	"backend-service/app/avmc/admin/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// Menu holds the schema definition for the Menu entity.
type Menu struct {
	ent.Schema
}

func (Menu) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("菜单表"),
	}
}

// Fields of the Menu.
func (Menu) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Comment("名称").Default("").MaxLen(32).NotEmpty().Nillable(),
		field.String("title").Comment("菜单标题").Default("").NotEmpty().Nillable(),
		field.Uint32("parent_id").Comment("父级ID").Default(0).Optional().Nillable(),
		field.Int32("type").Comment("菜单类型 0 UNSPECIFIED, 目录 1 -> FOLDER, 菜单 2 -> MENU, 按钮 3 -> BUTTON").Default(1).Nillable(),
		field.String("path").Comment("路径,当其类型为'按钮'的时候对应的数据操作名,例如:/user.service.v1.UserService/Login").Default("").Nillable(),
		field.String("component").Comment("前端页面组件").Default("").Nillable(),
		field.String("icon").Comment("图标").Default("").MaxLen(128).Nillable(),
		field.Bool("is_ext").Comment("是否外链").Default(false).Nillable(),
		field.String("ext_url").Comment("外链地址").MaxLen(255).Default("").Nillable(),
		field.Strings("permissions").Comment("权限代码 例如:sys:menu").SchemaType(map[string]string{dialect.MySQL: "json", dialect.Postgres: "jsonb"}).Default([]string{}).Optional(),
		field.String("redirect").Comment("跳转路径").Default("").Nillable(),
		field.String("current_active_menu").Comment("当前激活菜单").Default("").Nillable(),
		field.Bool("keep_alive").Comment("是否缓存").Default(false).Nillable(),
		field.Bool("visible").Comment("是否显示").Default(true).Nillable(),
		field.Bool("hide_tab").Comment("是否显示在标签页导航").Default(true).Nillable(),
		field.Bool("hide_menu").Comment("是否显示在菜单导航").Default(true).Nillable(),
		field.Bool("hide_breadcrumb").Comment("是否显示在面包屑导航").Default(true).Nillable(),
	}
}

// Edges of the Menu.
func (Menu) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("children", Menu.Type).
			From("parent").
			Unique().
			Field("parent_id"),
	}
}

// Mixin of the Menu.
func (Menu) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

// Indexes of the Menu.
func (Menu) Indexes() []ent.Index {
	return []ent.Index{
		// index.Fields("phone", "authority").Unique(),
	}
}
