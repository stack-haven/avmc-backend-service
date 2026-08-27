package schema

import (
	"backend-service/app/evie/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// DictionaryConflict 表示词库冲突记录。
// 当平台/系统/租户词库对同一输入给出不同候选时，按 TENANT > SYSTEM > PLATFORM 解析，
// 但冲突不静默吞掉，记录供后台查询（开发说明第十六节）。
type DictionaryConflict struct {
	ent.Schema
}

func (DictionaryConflict) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "evie_dictionary_conflicts",
		},
		entsql.WithComments(true),
		schema.Comment("词库冲突记录表：多作用域候选冲突"),
	}
}

func (DictionaryConflict) Fields() []ent.Field {
	return []ent.Field{
		field.String("input").MaxLen(255).NotEmpty().Comment("输入表达"),
		field.String("candidate").MaxLen(255).NotEmpty().Comment("候选结果"),
		field.String("source_scope").MaxLen(32).Comment("来源作用域: PLATFORM/SYSTEM/TENANT"),
		field.String("source_dictionary").MaxLen(128).Comment("来源词库标识"),
		field.Int32("priority").Default(0).Comment("候选优先级"),
		field.String("resolved_candidate").MaxLen(255).Optional().Comment("解析后的候选（最终采用）"),
	}
}

func (DictionaryConflict) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (DictionaryConflict) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "input", "source_scope", "source_dictionary").Unique(),
	}
}
