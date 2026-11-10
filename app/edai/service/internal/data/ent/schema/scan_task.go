package schema

import (
	"backend-service/app/edai/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ScanTask 扫描任务。
//
// 任务引擎自建（4-2 ADR-013 决策 5）：不扩展 platform 的任务中心。
// 状态机：pending → running → succeeded / failed / cancelled；
// running 期间可置 cancel_requested，由 worker 在安全点转为 cancelled。
type ScanTask struct{ ent.Schema }

func (ScanTask) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixinNoStatus{},
	}
}

func (ScanTask) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("source_id").
			Comment("数据源ID"),
		field.String("status").
			Comment("状态：pending/running/succeeded/failed/cancelled/cancel_requested").
			Default("pending").
			MaxLen(32),
		field.String("root_path").
			Comment("发起时的扫描根路径快照").
			MaxLen(4096).
			Default(""),
		field.String("sub_path").
			Comment("限定的子路径（相对根路径）").
			Optional().
			Nillable().
			MaxLen(4096),
		field.Int32("discovered").
			Comment("已发现文件数").
			Default(0),
		field.Int32("created_count").
			Comment("新登记数").
			Default(0),
		field.Int32("updated_count").
			Comment("更新数（指纹变化）").
			Default(0),
		field.Int32("marked_missing").
			Comment("标记消失数").
			Default(0),
		field.Int32("skipped").
			Comment("跳过数（排除规则命中）").
			Default(0),
		field.Int32("failed").
			Comment("失败数（权限/IO 错误）").
			Default(0),
		field.Bool("full_scan").
			Comment("是否全量扫描").
			Default(false),
		field.Bool("compute_hash").
			Comment("是否计算内容哈希").
			Default(false),
		field.String("error_message").
			Comment("错误信息").
			Optional().
			Nillable().
			MaxLen(2048),
		field.Time("started_at").
			Comment("开始时间"),
		field.Time("finished_at").
			Comment("结束时间").
			Optional().
			Nillable(),
	}
}

func (ScanTask) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("source_id"),
		index.Fields("status"),
		index.Fields("created_at"),
	}
}
