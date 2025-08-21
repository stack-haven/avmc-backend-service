package schema

import (
	"backend-service/app/avmc/admin/internal/data/ent/mixins"
	pkgMixin "backend-service/pkg/entgo/mixin"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
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
		field.String("name").Comment("菜单名称").Default("").MaxLen(32).NotEmpty(),
		field.String("path").Comment("路径,当其类型为'按钮'的时候对应的数据操作名,例如:/user.service.v1.UserService/Login").Default("").Optional().Nillable(),
		field.Int32("type").Comment("菜单类型 0 UNSPECIFIED, 目录 1 -> FOLDER, 菜单 2 -> MENU, 按钮 3 -> BUTTON").Default(1).SchemaType(map[string]string{dialect.MySQL: "tinyint", dialect.Postgres: "tinyint(2)"}).Optional(),
		field.String("component").Comment("组件").Default("").Optional().Nillable(),
		field.Uint32("pid").Comment("父级ID").Default(0).Optional(),
		field.String("redirect").Comment("重定向").Default("").Optional().Nillable(),
		field.String("auth_code").Comment("后端权限标识").Default("").Nillable(),
		// 以下为MenuMata字段仅对目录和菜单有效
		field.String("active_icon").Comment("激活时显示的图标").Default("").Nillable(),
		field.String("active_path").Comment("作为路由时，需要激活的菜单的Path").Default("").Nillable(),
		field.Bool("affix_tab").Comment("固定在标签栏").Default(false).Nillable(),
		field.Int32("affix_tab_order").Comment("在标签栏固定的顺序").Default(0).Nillable(),
		field.String("badge").Comment("徽标内容(当徽标类型为normal时有效)").Default("").Nillable(),
		field.Int32("badge_type").Comment("徽标类型").Default(0).Nillable(),
		field.Int32("badge_variants").Comment("徽标颜色").Default(0).Nillable(),
		field.Bool("hide_children_in_menu").Comment("在菜单中隐藏下级").Default(false).Nillable(),
		field.Bool("hide_in_breadcrumb").Comment("在面包屑中隐藏").Default(false).Nillable(),
		field.Bool("hide_in_menu").Comment("在菜单中隐藏").Default(false).Nillable(),
		field.Bool("hide_in_tab").Comment("在标签栏中隐藏").Default(false).Nillable(),
		field.String("icon").Comment("菜单图标").Default("").MaxLen(128).Nillable(),
		field.String("iframe_src").Comment("内嵌Iframe的URL").Default("").Nillable(),
		field.Bool("keep_alive").Comment("是否缓存页面").Default(false).Nillable(),
		field.String("link").Comment("外链页面的URL").Default("").Nillable(),
		field.Int32("max_num_of_open_tab").Comment("同一个路由最大打开的标签数").Default(0).Nillable(),
		field.Bool("no_basic_layout").Comment("无需基础布局").Default(false).Nillable(),
		field.Bool("open_in_new_window").Comment("是否在新窗口打开").Default(false).Nillable(),
		field.Int32("sort").Comment("菜单排序").Default(10).Nillable(),
		field.String("query").Comment("额外的路由参数").Default("").Optional().Nillable(),
		field.String("title").Comment("菜单标题").Default("").NotEmpty().Nillable(),
	}
}

// Edges of the Menu.
func (Menu) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("children", Menu.Type).
			From("parent").
			Unique().
			Field("pid"),
	}
}

// Mixin of the Menu.
func (Menu) Mixin() []ent.Mixin {
	return []ent.Mixin{
		pkgMixin.ID{},
		pkgMixin.Status{},
		pkgMixin.CreatedAt{},
		pkgMixin.UpdatedAt{},
		mixins.SoftDeleteMixin{},
	}
}

// Indexes of the Menu.
func (Menu) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name"),
		index.Fields("status"),
		index.Fields("pid"),
	}
}
