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

type OperationLog struct{ ent.Schema }

func (OperationLog) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin", Table: "system_operation_logs"}, entsql.WithComments(true), schema.Comment("操作审计日志表")}
}
func (OperationLog) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("operator_id").Optional().Nillable().Comment("操作人ID"),
		field.String("operator_name").MaxLen(100).Default("").Comment("操作人名称快照"),
		field.String("module").MaxLen(64).Comment("模块"),
		field.String("action").MaxLen(64).Comment("动作"),
		field.String("resource_type").MaxLen(64).Default("").Comment("资源类型"),
		field.String("resource_id").MaxLen(128).Default("").Comment("资源ID"),
		field.String("operation").MaxLen(255).Default("").Comment("Kratos operation"),
		field.String("method").MaxLen(16).Default("").Comment("请求方法"),
		field.String("path").MaxLen(512).Default("").Comment("请求路径"),
		field.Text("request_summary").Default("").Comment("脱敏请求摘要"),
		field.Text("before_data").Default("").Comment("变更前数据"),
		field.Text("after_data").Default("").Comment("变更后数据"),
		field.String("ip").MaxLen(64).Default("").Comment("客户端IP"),
		field.String("user_agent").MaxLen(512).Default("").Comment("User-Agent"),
		field.String("trace_id").MaxLen(64).Default("").Comment("链路追踪ID"),
		field.Bool("success").Default(true).Comment("是否成功"),
		field.Int64("duration_ms").Default(0).Comment("执行耗时毫秒"),
		field.Text("error_message").Default("").Comment("错误摘要"),
	}
}
func (OperationLog) Edges() []ent.Edge { return nil }
func (OperationLog) Mixin() []ent.Mixin {
	return []ent.Mixin{pkgMixin.ID{}, pkgMixin.CreatedAt{}, mixins.TenantID{}}
}
func (OperationLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "created_at"),
		index.Fields("tenant_id", "module", "action"),
		index.Fields("tenant_id", "operator_id"),
		index.Fields("trace_id"),
	}
}
