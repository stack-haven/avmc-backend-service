package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	entgo "backend-service/pkg/entgo/entc"
	pkgMixin "backend-service/pkg/entgo/mixin"
)

// AsyncTask stores durable background work claimed by one worker at a time.
type AsyncTask struct{ ent.Schema }

func (AsyncTask) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin", Table: "system_async_tasks"},
		entsql.WithComments(true),
		schema.Comment("统一异步任务表"),
		// 2026-11-12 决策：异步任务暂不做租户隔离。任务里含有跨租户的运维动作，
		// 租户维度的过滤会让 worker 无法统一领取，改造成本高于收益。
		// 现状：该表保留 tenant_id 列（记录任务归属，供查询展示），但不施加隐私策略；
		// 访问边界由平台操作面鉴权（platformControlServer + Casbin）承担。
		// 后续若要升级为租户隔离：删除本注解，改为嵌入 mixins.TenantID 并补齐
		// rule 分派，同时调整 worker 的领取逻辑（用 system 上下文跨租户领取）。
		entgo.GlobalScope{},
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
