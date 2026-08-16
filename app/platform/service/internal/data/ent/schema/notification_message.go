package schema

import (
	"backend-service/app/platform/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// NotificationMessage stores in-app notification records.
type NotificationMessage struct{ ent.Schema }

func (NotificationMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin", Table: "system_notification_messages"},
		entsql.WithComments(true),
		schema.Comment("通知消息表"),
	}
}

func (NotificationMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("recipient_user_id").Comment("接收用户ID"),
		field.Uint32("template_id").Optional().Nillable().Comment("模板ID"),
		field.String("template_code").MaxLen(100).Default("").Comment("模板编码快照"),
		field.Int32("channel").Default(1).Comment("通知渠道"),
		field.String("title").MaxLen(200).NotEmpty().Comment("通知标题"),
		field.Text("content").Comment("通知内容"),
		field.Int32("message_status").Default(1).Comment("消息状态"),
		field.Int32("priority").Default(0).Comment("优先级"),
		field.String("business_type").MaxLen(100).Default("").Comment("业务类型"),
		field.String("business_id").MaxLen(100).Default("").Comment("业务ID"),
		field.Time("read_at").Optional().Nillable().Comment("阅读时间"),
		field.Uint32("sender_user_id").Optional().Nillable().Comment("发送人ID"),
		field.String("sender_name").MaxLen(100).Default("").Comment("发送人名称快照"),
	}
}

func (NotificationMessage) Edges() []ent.Edge { return nil }

func (NotificationMessage) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (NotificationMessage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "recipient_user_id", "message_status", "created_at"),
		index.Fields("tenant_id", "business_type", "business_id"),
		index.Fields("tenant_id", "template_code", "created_at"),
	}
}
