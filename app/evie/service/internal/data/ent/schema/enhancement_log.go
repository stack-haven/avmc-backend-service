package schema

import (
	"backend-service/app/evie/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// EnhancementLog 文本增强记录（开发说明第四十九节：可观测性）。
type EnhancementLog struct {
	ent.Schema
}

func (EnhancementLog) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "evie_enhancement_logs",
		},
		entsql.WithComments(true),
		schema.Comment("文本增强记录表：请求记录 + 分阶段耗时 + 反馈预留"),
	}
}

func (EnhancementLog) Fields() []ent.Field {
	return []ent.Field{
		field.String("request_id").MaxLen(128).Optional().Comment("请求标识"),
		field.String("session_id").MaxLen(128).Optional().Comment("会话标识"),
		field.Uint32("policy_id").Optional().Comment("使用的策略 ID"),
		field.String("policy_mode").MaxLen(32).Optional().Comment("策略模式"),
		field.String("context_version").MaxLen(64).Optional().Comment("词库上下文版本"),
		field.String("raw_text").SchemaType(map[string]string{dialect.MySQL: "text"}).NotEmpty().Comment("原文"),
		field.String("enhanced_text").SchemaType(map[string]string{dialect.MySQL: "text"}).Optional().Comment("增强后文本"),
		field.String("changes_json").SchemaType(map[string]string{dialect.MySQL: "text"}).Optional().Comment("变更列表 JSON"),
		field.Int64("processing_time_ms").Default(0).Comment("总处理时间（毫秒）"),
		field.Int64("cleaning_time_ms").Default(0).Comment("文本清洗耗时"),
		field.Int64("filler_time_ms").Default(0).Comment("口水词处理耗时"),
		field.Int64("vocab_match_time_ms").Default(0).Comment("词库匹配耗时"),
		field.Int64("alias_time_ms").Default(0).Comment("别名解析耗时"),
		field.Int64("deterministic_time_ms").Default(0).Comment("确定性替换耗时"),
		field.Int64("pinyin_time_ms").Default(0).Comment("拼音纠错耗时"),
		field.Int64("fuzzy_time_ms").Default(0).Comment("模糊匹配耗时"),
		field.Int64("context_time_ms").Default(0).Comment("上下文纠错耗时"),
		field.Bool("user_corrected").Default(false).Comment("用户是否纠正"),
		field.String("feedback_text").MaxLen(1024).Optional().Comment("反馈文本"),
			field.String("error_message").MaxLen(512).Optional().Comment("错误信息"),
	}
}

func (EnhancementLog) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
	}
}

func (EnhancementLog) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "created_at"),
		index.Fields("session_id"),
	}
}
