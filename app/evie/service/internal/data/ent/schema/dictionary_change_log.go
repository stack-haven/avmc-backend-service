package schema

import (
	"backend-service/app/evie/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// DictionaryChangeLog 表示词库/词条/关系的变更记录（审计追溯）。
// 词条与词库不物理删除，变更（含停用/归档）通过本表留痕。
type DictionaryChangeLog struct {
	ent.Schema
}

func (DictionaryChangeLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "evie_dictionary_change_logs",
		},
		entsql.WithComments(true),
		schema.Comment("词库变更记录表：审计追溯"),
	}
}

func (DictionaryChangeLog) Fields() []ent.Field {
	return []ent.Field{
		field.String("entity_type").MaxLen(32).NotEmpty().Comment("实体类型: dictionary/entry/relation/category/version"),
		field.Uint32("entity_id").Comment("实体 ID"),
		field.String("action").MaxLen(32).NotEmpty().Comment("动作: create/update/disable/enable/publish/archive"),
		field.String("before_snapshot").MaxLen(4096).Optional().Comment("变更前快照（JSON）"),
		field.String("after_snapshot").MaxLen(4096).Optional().Comment("变更后快照（JSON）"),
		field.Uint32("operator_id").Optional().Comment("操作人 ID"),
	}
}

func (DictionaryChangeLog) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
	}
}

func (DictionaryChangeLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "entity_type", "entity_id"),
		index.Fields("created_at"),
	}
}
