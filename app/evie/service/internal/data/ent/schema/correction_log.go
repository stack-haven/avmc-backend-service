package schema

import (
	"backend-service/app/evie/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// CorrectionLog holds the schema definition for the correction log.
type CorrectionLog struct {
	ent.Schema
}

func (CorrectionLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "evie_correction_logs",
		},
		entsql.WithComments(true),
		schema.Comment("纠错记录表（只追加不删除）"),
	}
}

// Fields of the CorrectionLog.
func (CorrectionLog) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("user_id").Optional().Comment("用户ID"),
		field.String("session_id").MaxLen(64).Comment("会话ID"),
		field.Text("original_text").NotEmpty().Comment("原始文本"),
		field.Text("corrected_text").Comment("纠正后文本"),
		field.Text("changes_json").Comment("修正明细 JSON"),
		field.Float("confidence").Comment("整体置信度"),
		field.Bool("need_confirm").Default(false).Comment("是否需要用户确认"),
		field.Int64("duration_ms").Comment("纠错耗时(ms)"),
		field.Int("rule_hits").Default(0).Comment("规则命中次数"),
		field.Int("pinyin_hits").Default(0).Comment("拼音命中次数"),
		field.Int("entity_hits").Default(0).Comment("实体命中次数"),
		field.Int("llm_hits").Default(0).Comment("LLM命中次数"),
	}
}

// Mixin of the CorrectionLog.
func (CorrectionLog) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.BaseMixin{}} // 无软删除，只追加
}

// Indexes of the CorrectionLog.
func (CorrectionLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "created_at"),
		index.Fields("session_id"),
	}
}
