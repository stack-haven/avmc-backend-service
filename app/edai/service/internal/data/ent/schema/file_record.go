package schema

import (
	"backend-service/app/edai/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// FileRecord 物理文件记录。
//
// 关键约束（系统设计基线 §5.2）：
//   - 路径不是文件身份：raw_path 可随文件移动而变化，业务身份留给资产层；
//   - content_hash 只用于去重与完整性，不代替资产ID；
//   - 原始路径 / 文件名 / 文件系统时间必须原样保留，不被推断结果覆盖；
//   - 文件消失或被拒绝访问要标记状态，不删除记录。
type FileRecord struct{ ent.Schema }

func (FileRecord) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
	}
}

func (FileRecord) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("source_id").
			Comment("所属数据源ID"),
		field.String("raw_path").
			Comment("原始完整路径（原样记录）").
			NotEmpty().
			MaxLen(4096),
		field.String("raw_parent_path").
			Comment("原始父目录").
			MaxLen(4096).
			Default(""),
		field.String("file_name").
			Comment("文件名").
			NotEmpty().
			MaxLen(512),
		field.String("extension").
			Comment("扩展名（保留原始大小写，可空）").
			Optional().
			Nillable().
			MaxLen(64),
		field.Int64("file_size").
			Comment("文件大小（字节）").
			Default(0),
		field.Time("fs_created_at").
			Comment("文件系统创建时间（原始语义）").
			Optional().
			Nillable(),
		field.Time("fs_modified_at").
			Comment("文件系统修改时间（原始语义）").
			Optional().
			Nillable(),
		field.String("content_hash").
			Comment("内容哈希 SHA256（可空=未计算）").
			Optional().
			Nillable().
			MaxLen(64),
		field.String("quick_fingerprint").
			Comment("快速指纹（size+mtime），用于增量判断").
			MaxLen(128).
			Default(""),
		field.Time("first_seen_at").
			Comment("首次发现时间"),
		field.Time("last_scanned_at").
			Comment("最近一次扫描看到它的时间"),
		field.String("file_status").
			Comment("文件状态：present/missing/unreachable").
			Default("present").
			MaxLen(32),
		field.String("status_message").
			Comment("状态说明（不可访问原因）").
			Optional().
			Nillable().
			MaxLen(512),
		field.Bool("is_duplicate").
			Comment("是否与其它记录内容重复").
			Default(false),
	}
}

func (FileRecord) Indexes() []ent.Index {
	return []ent.Index{
		// 同一数据源内路径唯一：同一路径只对应一条记录，移动文件产生新路径记录
		index.Fields("tenant_id", "source_id", "raw_path").Unique(),
		index.Fields("source_id"),
		index.Fields("content_hash"),
		index.Fields("extension"),
		index.Fields("file_status"),
		index.Fields("file_size"),
	}
}
