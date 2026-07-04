package schema

import (
	pkgMixin "backend-service/pkg/entgo/mixin"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AsyncTask stores durable background work claimed by one worker at a time.
type AsyncTask struct{ ent.Schema }

func (AsyncTask) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin"},
		entsql.WithComments(true),
		schema.Comment("统一异步任务表"),
	}
}
func (AsyncTask) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("tenant_id").Optional().Nillable().Comment("租户ID，平台任务为空"),
		field.String("task_type").MaxLen(100).NotEmpty().Comment("已注册任务类型"),
		field.String("queue").MaxLen(50).Default("default").Comment("队列"),
		field.Int32("status").Default(1).Comment("状态"),
		field.Int32("priority").Default(0).Comment("优先级，值越大越优先"),
		field.Text("payload").Default("{}").Comment("内部任务载荷JSON"),
		field.String("payload_summary").MaxLen(500).Default("").Comment("脱敏载荷摘要"),
		field.String("result_summary").MaxLen(1000).Default("").Comment("执行结果摘要"),
		field.Text("last_error").Default("").Comment("最近一次错误摘要"),
		field.String("idempotency_key").MaxLen(150).Optional().Nillable().Unique().Comment("幂等键"),
		field.Int32("attempts").Default(0).Comment("已执行次数"),
		field.Int32("max_attempts").Default(3).Comment("最大执行次数"),
		field.Time("scheduled_at").Comment("计划执行时间"),
		field.Time("started_at").Optional().Nillable().Comment("最近开始时间"),
		field.Time("completed_at").Optional().Nillable().Comment("完成时间"),
		field.String("lease_owner").MaxLen(150).Default("").Comment("租约持有者"),
		field.Time("lease_expires_at").Optional().Nillable().Comment("租约过期时间"),
		field.Uint32("created_by").Optional().Nillable().Comment("创建人"),
	}
}

func (AsyncTask) Edges() []ent.Edge { return nil }

func (AsyncTask) Mixin() []ent.Mixin {
	return []ent.Mixin{
		pkgMixin.ID{},
		pkgMixin.CreatedAt{},
		pkgMixin.UpdatedAt{},
	}
}

func (AsyncTask) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "queue", "scheduled_at", "priority"),
		index.Fields("status", "lease_expires_at"),
		index.Fields("tenant_id", "created_at"),
		index.Fields("task_type", "created_at"),
	}
}
