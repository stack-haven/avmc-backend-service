package schema

import (
	"backend-service/app/edai/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// User EDAI 本地账号。
//
// EDAI 不依赖 platform/service 的用户/会话（4-2 ADR-013 决策 4），
// 账号与密码由本服务自持，JWT 由 pkg/auth 自签。
type User struct{ ent.Schema }

func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.BaseMixin{}}
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.String("username").
			Comment("登录账号").
			NotEmpty().
			MaxLen(64),
		field.String("password_hash").
			Comment("密码 bcrypt 哈希").
			NotEmpty().
			MaxLen(255).
			Sensitive(),
		field.String("display_name").
			Comment("显示名称").
			Optional().
			Nillable().
			MaxLen(64),
		field.String("email").
			Comment("邮箱").
			Optional().
			Nillable().
			MaxLen(128),
		field.String("role").
			Comment("角色：admin/operator/viewer").
			Default("viewer").
			MaxLen(32),
		field.Time("last_login_at").
			Comment("最近登录时间").
			Optional().
			Nillable().
			Default(nil),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		// 账号在租户内唯一（V1.0 单租户，等价于全局唯一，但保留 tenant_id 维度以便扩展）
		index.Fields("tenant_id", "username").Unique(),
	}
}
