package schema

import (
	"backend-service/app/platform/admin/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// WebhookDeliveryLog stores webhook delivery attempt records.
type WebhookDeliveryLog struct{ ent.Schema }

func (WebhookDeliveryLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin"},
		entsql.WithComments(true),
		schema.Comment("Webhook投递记录表"),
	}
}

func (WebhookDeliveryLog) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("subscription_id").Comment("关联的订阅ID"),
		field.String("event_id").MaxLen(128).NotEmpty().Comment("事件唯一ID"),
		field.Int32("event_type").Comment("事件类型"),
		field.String("target_url").MaxLen(2048).NotEmpty().Comment("投递目标URL"),
		field.Text("request_body").Comment("请求体"),
		field.Int32("response_code").Default(0).Comment("HTTP响应码"),
		field.Text("response_body").Default("").Comment("响应体"),
		field.Int32("delivery_status").Default(1).Comment("投递状态: 1=PENDING 2=SUCCESS 3=FAILED"),
		field.Int32("attempt_number").Default(1).Comment("尝试次数"),
		field.String("error_message").MaxLen(1024).Default("").Comment("错误信息"),
	}
}

func (WebhookDeliveryLog) Edges() []ent.Edge { return nil }

func (WebhookDeliveryLog) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
	}
}

func (WebhookDeliveryLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "subscription_id", "created_at"),
		index.Fields("tenant_id", "event_type", "created_at"),
		index.Fields("event_id"),
		index.Fields("delivery_status", "created_at"),
	}
}
