package schema

import (
	"backend-service/app/evie/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// AsrProviderConfig holds the schema definition for the tenant ASR provider config.
type AsrProviderConfig struct {
	ent.Schema
}

func (AsrProviderConfig) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "evie_asr_provider_configs",
		},
		entsql.WithComments(true),
		schema.Comment("ASR 供应商租户配置表"),
	}
}

// Fields of the AsrProviderConfig.
func (AsrProviderConfig) Fields() []ent.Field {
	return []ent.Field{
		field.String("provider_name").MaxLen(32).NotEmpty().Comment("funasr/whisper/xunfei/aliyun"),
		field.Bool("is_active").Default(false).Comment("是否启用"),
		field.Text("config_json").Comment("Provider 连接配置 JSON"),
		field.Int("sample_rate").Default(16000).Comment("采样率"),
		field.String("language").MaxLen(8).Default("zh").Comment("语言代码"),
	}
}

// Mixin of the AsrProviderConfig.
func (AsrProviderConfig) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.BaseMixin{}, mixins.SoftDeleteMixin{}}
}

// Indexes of the AsrProviderConfig.
func (AsrProviderConfig) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "provider_name").Unique(),
	}
}
