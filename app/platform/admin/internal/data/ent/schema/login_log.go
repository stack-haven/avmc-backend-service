package schema

import (
	"backend-service/app/platform/admin/internal/data/ent/mixins"
	pkgMixin "backend-service/pkg/entgo/mixin"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type LoginLog struct{ ent.Schema }

func (LoginLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin"},
		entsql.WithComments(true),
		schema.Comment("不可变登录安全日志表"),
	}
}

func (LoginLog) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("user_id").Optional().Nillable().Comment("用户ID，未知账号时为空"),
		field.String("identity").MaxLen(100).Comment("登录身份快照"),
		field.String("login_type").MaxLen(32).Comment("登录方式"),
		field.String("result").MaxLen(32).Comment("登录结果"),
		field.String("failure_reason").MaxLen(255).Default("").Comment("失败原因"),
		field.String("ip").MaxLen(64).Default("").Comment("客户端IP"),
		field.String("user_agent").MaxLen(512).Default("").Comment("User-Agent"),
		field.String("trace_id").MaxLen(64).Default("").Comment("链路追踪ID"),
		field.String("session_id").MaxLen(64).Default("").Comment("会话ID"),
	}
}

func (LoginLog) Edges() []ent.Edge { return nil }

func (LoginLog) Mixin() []ent.Mixin {
	return []ent.Mixin{pkgMixin.ID{}, pkgMixin.CreatedAt{}, mixins.TenantID{}}
}

func (LoginLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "created_at"),
		index.Fields("tenant_id", "user_id", "created_at"),
		index.Fields("tenant_id", "result", "created_at"),
		index.Fields("trace_id"),
		index.Fields("session_id"),
	}
}
