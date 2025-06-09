package schema

import (
	"regexp"
	"time"

	"entgo.io/contrib/entproto"
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
)

// User holds the schema definition for the User entity.
type User struct {
	ent.Schema
}

// Fields of the User.
func (User) Fields() []ent.Field {
	return []ent.Field{
		// field.Int32("id").Immutable().Unique().Comment("用户唯一标识"),
		field.String("username").Unique().MinLen(3).MaxLen(32).Match(regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)).Comment("用户名，唯一"),
		field.String("password").Sensitive().MinLen(6).MaxLen(100).Comment("密码哈希"),
		field.String("name").MaxLen(50).Comment("用户真实姓名"),
		field.String("nickname").Optional().MaxLen(50).Comment("用户昵称"),
		field.String("email").Optional().Unique().MaxLen(100).Match(regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)).Comment("电子邮箱，唯一"),
		field.String("mobile").Optional().Unique().MaxLen(20).Comment("手机号码，唯一"),
		field.String("avatar").Optional().MaxLen(255).Comment("头像URL"),
		field.Enum("gender").Values("male", "female", "unknown").Default("unknown").Comment("性别"),
		field.Int("age").Optional().Min(0).Max(150).Comment("年龄"),
		field.String("role").Default("user").Comment("用户角色：admin, user等"),
		field.Enum("status").Values("active", "inactive", "locked", "deleted").Default("active").Comment("用户状态"),
		field.Time("last_login_at").Optional().Nillable().Comment("最后登录时间"),
		field.String("last_login_ip").Optional().MaxLen(50).Comment("最后登录IP"),
		field.Int("login_count").Default(0).Comment("登录次数"),
		field.JSON("settings", map[string]interface{}{}).Optional().Comment("用户设置，JSON格式"),
		field.JSON("metadata", map[string]interface{}{}).Optional().Comment("元数据，JSON格式"),
		field.Time("deleted_at").Optional().Nillable().Comment("删除时间，用于软删除"),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return nil
}

// Mixin of the User.
func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		AutoIncrementId{},
		TimeMixin{},
	}
}

// Indexes of the User.
func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("username"),
		index.Fields("email"),
		index.Fields("mobile"),
		index.Fields("status"),
		index.Fields("role"),
	}
}

// TimeMixin implements the time mixin for schemas.
type TimeMixin struct {
	mixin.Schema
}

// Fields of the TimeMixin.
func (TimeMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Time("created_at").Default(time.Now).Immutable().Comment("创建时间"),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now).Comment("更新时间"),
	}
}

type AutoIncrementId struct{ mixin.Schema }

func (AutoIncrementId) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("id").
			Comment("id").
			StructTag(`json:"id,omitempty"`).
			SchemaType(map[string]string{
				dialect.MySQL:    "bigint",
				dialect.Postgres: "serial",
			}).
			Annotations(
				entproto.Field(1),
			).
			Positive().
			Immutable().
			Unique(),
	}
}

// Indexes of the AutoIncrementId.
func (AutoIncrementId) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("id"),
	}
}
