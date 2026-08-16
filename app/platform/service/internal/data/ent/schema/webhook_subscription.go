package schema

import (
	"backend-service/app/platform/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// WebhookSubscription stores tenant webhook endpoint subscriptions.
type WebhookSubscription struct{ ent.Schema }

func (WebhookSubscription) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin", Table: "system_webhook_subscriptions"},
		entsql.WithComments(true),
		schema.Comment("Webhook订阅表"),
	}
}

func (WebhookSubscription) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(100).NotEmpty().Comment("订阅名称"),
		field.String("url").MaxLen(2048).NotEmpty().Comment("回调URL"),
		field.String("secret").MaxLen(256).NotEmpty().Sensitive().Comment("HMAC签名密钥"),
		field.JSON("event_types", []int32{}).Comment("订阅的事件类型列表"),
	}
}

func (WebhookSubscription) Edges() []ent.Edge { return nil }

func (WebhookSubscription) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (WebhookSubscription) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "status"),
	}
}
