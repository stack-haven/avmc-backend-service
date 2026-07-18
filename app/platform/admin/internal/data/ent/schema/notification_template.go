package schema

import (
	"backend-service/app/platform/admin/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// NotificationTemplate stores reusable notification templates.
type NotificationTemplate struct{ ent.Schema }

func (NotificationTemplate) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin"},
		entsql.WithComments(true),
		schema.Comment("通知模板表"),
	}
}

func (NotificationTemplate) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(100).NotEmpty().Comment("模板编码"),
		field.String("name").MaxLen(100).NotEmpty().Comment("模板名称"),
		field.Int32("channel").Default(1).Comment("通知渠道"),
		field.String("title").MaxLen(200).NotEmpty().Comment("标题模板"),
		field.Text("content").Comment("内容模板"),
		field.Text("variable_schema").Default("").Comment("变量Schema JSON"),
		field.String("locale").MaxLen(20).Default("zh-CN").Comment("语言"),
		field.String("remark").MaxLen(255).Default("").Comment("备注"),
	}
}

func (NotificationTemplate) Edges() []ent.Edge { return nil }

func (NotificationTemplate) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (NotificationTemplate) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "code").Unique(),
		index.Fields("tenant_id", "channel", "status"),
	}
}
