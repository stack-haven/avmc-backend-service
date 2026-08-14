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

// NotificationProvider holds platform-owned notification channel configuration.
// 平台级通知渠道配置（站内信 / 短信 / 邮件 / Webhook）。
type NotificationProvider struct {
	ent.Schema
}

func (NotificationProvider) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin", Table: "system_notification_providers"},
		entsql.WithComments(true),
		schema.Comment("通知渠道配置表"),
	}
}

func (NotificationProvider) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").Comment("渠道配置编码").MaxLen(80).NotEmpty(),
		field.String("name").Comment("渠道配置名称").MaxLen(120).NotEmpty(),
		field.String("channel").Comment("渠道类型 in-app/sms/email/webhook/push").MaxLen(40).NotEmpty(),
		field.String("provider_type").Comment("提供商类型 aliyun-sms/yunpian/jpush/getui").MaxLen(80).Default(""),
		field.String("endpoint").Comment("服务商 endpoint").MaxLen(255).Default(""),
		field.String("access_key_id").Comment("访问密钥ID").MaxLen(120).Default(""),
		field.String("access_key_secret").Comment("访问密钥（加密存储）").MaxLen(200).Default(""),
		field.String("sign_name").Comment("短信签名").MaxLen(80).Default(""),
		field.String("template_code").Comment("短信模板代码").MaxLen(80).Default(""),
		field.Bool("is_default").Comment("是否默认渠道配置").Default(false),
		field.String("remark").Comment("备注").MaxLen(500).Default(""),
	}
}

func (NotificationProvider) Mixin() []ent.Mixin {
	return []ent.Mixin{
		pkgMixin.ID{},
		pkgMixin.Status{},
		pkgMixin.CreatedAt{},
		pkgMixin.UpdatedAt{},
		mixins.SoftDeleteMixin{},
	}
}

func (NotificationProvider) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").Unique(),
		index.Fields("channel", "status"),
		index.Fields("is_default"),
	}
}
