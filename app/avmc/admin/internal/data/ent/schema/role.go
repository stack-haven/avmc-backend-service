package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Role holds the schema definition for the Role entity.
type Role struct {
	ent.Schema
}

func (Role) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("角色表"),
	}
}

// Fields of the Role.
func (Role) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Comment("名称").MaxLen(50).Default("").Nillable(),
		field.String("default_router").Comment("默认路由").MaxLen(255).Default("").Nillable(),
		field.Int32("data_scope").Comment("数据范围（0：未指定 1：全部数据权限 2：本人数据权限 3：本部门数据权限 4：本部门及以下数据权限 5：自定部门数据权限 ）").SchemaType(map[string]string{dialect.MySQL: "tinyint(2)", dialect.Postgres: "tinyint(2)"}).Default(1).Nillable(),
		field.Int32("menu_check_strictly").Comment("菜单树选择项是否关联显示").SchemaType(map[string]string{dialect.MySQL: "tinyint(2)", dialect.Postgres: "tinyint(2)"}).Default(1).Nillable(),
		field.Int32("dept_check_strictly").Comment("部门树选择项是否关联显示").SchemaType(map[string]string{dialect.MySQL: "tinyint(2)", dialect.Postgres: "tinyint(2)"}).Default(1).Nillable(),
	}
}

// Edges of the Role.
func (Role) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("users", User.Type).
			Ref("roles"),
	}
}

// Mixin of the Role.
func (Role) Mixin() []ent.Mixin {
	return []ent.Mixin{
		MixinTop{},
	}
}

// Indexes of the Role.
func (Role) Indexes() []ent.Index {
	return []ent.Index{
		// index.Fields("phone", "authority").Unique(),
		index.Fields("name"),
	}
}
