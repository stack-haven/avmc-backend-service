package schema

import (
	"backend-service/app/platform/admin/internal/data/ent/mixins"
	"regexp"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

func (User) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "system_users",
		},
		entsql.WithComments(true),
		schema.Comment("用户表"),
	}
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MinLen(3).MaxLen(32).Match(regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)).Nillable().Comment("用户名，域内唯一"),
		field.String("password").Sensitive().MinLen(6).MaxLen(100).Nillable().Comment("密码哈希"),
		field.String("realname").Optional().MaxLen(50).Nillable().Comment("用户真实姓名"),
		field.String("nickname").Optional().MaxLen(50).Nillable().Comment("用户昵称"),
		field.String("email").Optional().MaxLen(100).Match(regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)).Nillable().Comment("电子邮箱，域内唯一"),
		field.String("phone").Optional().MaxLen(20).Nillable().Comment("手机号码，域内唯一"),
		field.String("avatar").Optional().MaxLen(255).Nillable().Comment("头像URL"),
		field.Time("birthday").Optional().SchemaType(map[string]string{dialect.MySQL: "date"}).Nillable().Comment("生日"),
		field.Int32("gender").Max(5).Min(0).Default(0).SchemaType(map[string]string{dialect.MySQL: "tinyint", dialect.Postgres: "tinyint(2)"}).Nillable().Comment("性别：0=未知 1=男 2=女"),
		field.Int("age").Optional().Min(0).Max(150).Nillable().Comment("年龄"),
		field.Time("last_login_at").Optional().Nillable().Nillable().Comment("最后登录时间"),
		field.String("last_login_ip").Optional().MaxLen(50).Nillable().Comment("最后登录IP"),
		field.Int("login_count").Default(0).Nillable().Comment("登录次数"),
		field.JSON("settings", []string{}).Optional().Default([]string{}).Comment("用户设置，JSON格式"),
		field.JSON("metadata", []string{}).Optional().Default([]string{}).Comment("元数据，JSON格式"),
		field.String("description").Optional().MaxLen(255).Nillable().Comment("个人说明"),
		field.Uint32("dept_id").Optional().Nillable().Comment("所属主部门ID"),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("roles", Role.Type).StorageKey(edge.Table("system_user_roles")).Comment("用户角色关联表"),
		edge.To("posts", Post.Type).Comment("用户岗位关联表"),
		edge.From("dept", Dept.Type).
			Ref("users").
			Field("dept_id").
			Unique(),
		edge.From("projects", Project.Type).Ref("members"),
	}
}

// Mixin of the User.
func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

// Indexes of the User.
func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "name").Unique(),
		index.Fields("tenant_id", "phone").Unique(),
		index.Fields("status"),
		index.Fields("tenant_id", "email").Unique(),
	}
}
