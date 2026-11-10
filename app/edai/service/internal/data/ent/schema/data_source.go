package schema

import (
	"backend-service/app/edai/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// DataSource 文件数据源配置。
//
// 原始文件默认只读（系统设计基线 §1.3 原则 2）：本表只保存「去哪扫描」与
// 「扫描哪些」，不保存文件内容，也不代表已登记的文件。
type DataSource struct{ ent.Schema }

func (DataSource) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.BaseMixin{}}
}

func (DataSource) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("数据源名称").
			NotEmpty().
			MaxLen(128),
		field.String("type").
			Comment("类型：local/smb").
			Default("local").
			MaxLen(16),
		field.String("root_path").
			Comment("根路径（本地绝对路径或 SMB UNC）").
			NotEmpty().
			MaxLen(1024),
		field.Strings("include_patterns").
			Comment("包含规则（glob），空=全部").
			Optional(),
		field.Strings("exclude_patterns").
			Comment("排除规则（glob）").
			Optional(),
		field.String("connection_status").
			Comment("连接状态：unknown/reachable/unreachable").
			Default("unknown").
			MaxLen(32),
		field.String("connection_message").
			Comment("连接测试说明").
			Optional().
			Nillable().
			MaxLen(512),
		field.Time("last_checked_at").
			Comment("最近连接测试时间").
			Optional().
			Nillable(),
		field.Time("last_scanned_at").
			Comment("最近扫描时间").
			Optional().
			Nillable(),
		field.String("remark").
			Comment("备注").
			Optional().
			Nillable().
			MaxLen(255),
	}
}

func (DataSource) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "name").Unique(),
	}
}
