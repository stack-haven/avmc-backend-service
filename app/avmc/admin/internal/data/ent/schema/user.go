package schema

import (
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
		},
		entsql.WithComments(true),
		schema.Comment("用户表"),
	}
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Unique().MinLen(3).MaxLen(32).Match(regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)).Nillable().Comment("用户名，唯一"),
		field.String("password").Sensitive().MinLen(6).MaxLen(100).Nillable().Comment("密码哈希"),
		field.String("realname").Optional().MaxLen(50).Nillable().Comment("用户真实姓名"),
		field.String("nickname").Optional().MaxLen(50).Nillable().Comment("用户昵称"),
		field.String("email").Optional().Unique().MaxLen(100).Match(regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)).Nillable().Comment("电子邮箱，唯一"),
		field.String("phone").Optional().Unique().MaxLen(20).Nillable().Comment("手机号码，唯一"),
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
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("roles", Role.Type),
		edge.To("posts", Post.Type),
	}
}

// Mixin of the User.
func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		MixinTop{},
	}
}

// Indexes of the User.
func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name"),
		index.Fields("phone"),
		index.Fields("status"),
		index.Fields("email"),
	}
}
