package schema

import (
	"backend-service/app/platform/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Device holds user mobile device registrations for APP push.
// 用户设备注册表（APP 推送前置：极光/个推等）。
type Device struct {
	ent.Schema
}

func (Device) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin", Table: "system_devices"},
		entsql.WithComments(true),
		schema.Comment("用户设备注册表"),
	}
}

func (Device) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("user_id").Comment("用户ID"),
		field.String("device_token").Comment("设备令牌（极光 registration id / 个推 cid）").MaxLen(200).NotEmpty(),
		field.String("platform").Comment("平台 android/ios").MaxLen(20).NotEmpty(),
		field.String("app_key").Comment("应用标识（支持多 APP）").MaxLen(80).Default(""),
		field.String("device_name").Comment("设备名称").MaxLen(120).Default(""),
		field.String("app_version").Comment("APP 版本").MaxLen(40).Default(""),
		field.Time("last_active_at").Comment("最后活跃时间"),
	}
}

func (Device) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
	}
}

func (Device) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "device_token").Unique(),
		index.Fields("tenant_id", "user_id"),
		index.Fields("tenant_id", "platform"),
	}
}
