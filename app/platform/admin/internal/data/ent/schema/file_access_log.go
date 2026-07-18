package schema

import (
	"backend-service/app/platform/admin/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// FileAccessLog holds tenant file access audit records.
type FileAccessLog struct {
	ent.Schema
}

func (FileAccessLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin"},
		entsql.WithComments(true),
		schema.Comment("文件中心访问日志表"),
	}
}

func (FileAccessLog) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("file_id").Comment("文件ID"),
		field.String("file_name").Comment("文件名快照").MaxLen(255).Default(""),
		field.String("action").Comment("访问动作 download/delete/preview").MaxLen(40).NotEmpty(),
		field.Uint32("operator_id").Comment("操作人ID").Optional().Nillable(),
		field.String("operator_name").Comment("操作人名称").MaxLen(80).Default(""),
		field.String("client_ip").Comment("客户端IP").MaxLen(80).Default(""),
		field.String("user_agent").Comment("User-Agent").MaxLen(500).Default(""),
		field.String("result").Comment("结果 success/failure").MaxLen(40).Default("success"),
		field.String("message").Comment("结果说明").MaxLen(500).Default(""),
	}
}

func (FileAccessLog) Edges() []ent.Edge {
	return nil
}

func (FileAccessLog) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
	}
}

func (FileAccessLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "file_id", "created_at"),
		index.Fields("tenant_id", "action", "created_at"),
	}
}
