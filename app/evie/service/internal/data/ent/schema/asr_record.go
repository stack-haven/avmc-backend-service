package schema

import (
	"backend-service/app/evie/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AsrRecord holds the schema definition for the ASR recognition record.
type AsrRecord struct {
	ent.Schema
}

func (AsrRecord) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "evie_asr_records",
		},
		entsql.WithComments(true),
		schema.Comment("ASR 识别记录表（只追加不删除）"),
	}
}

// Fields of the AsrRecord.
func (AsrRecord) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("user_id").Optional().Comment("用户ID"),
		field.String("session_id").MaxLen(64).Comment("会话ID"),
		field.Text("raw_text").Comment("ASR 原始文本"),
		field.Float("confidence").Comment("识别置信度"),
		field.Int64("duration_ms").Comment("处理耗时(ms)"),
		field.Int("audio_duration_ms").Comment("音频时长(ms)"),
		field.String("audio_url").MaxLen(512).Optional().Comment("文件中心文件ID（供预览重识别）"),
		field.String("audio_format").Default("pcm").MaxLen(16).Comment("pcm/wav/mp3/opus"),
		field.String("engine").Default("funasr").MaxLen(32).Comment("funasr/whisper/sensevoice"),
	}
}

// Mixin of the AsrRecord.
func (AsrRecord) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.BaseMixin{}} // 无软删除，只追加
}

// Indexes of the AsrRecord.
func (AsrRecord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "created_at"),
		index.Fields("session_id"),
	}
}
