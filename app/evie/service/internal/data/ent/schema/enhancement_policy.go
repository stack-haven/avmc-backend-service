package schema

import (
	"backend-service/app/evie/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// EnhancementPolicy 文本增强策略：决定增强引擎执行哪些层（开发说明第三十/三十一节）。
// 模式（HIGH_PERFORMANCE/STANDARD/HIGH_ACCURACY）为预设，各层开关可细粒度覆盖。
type EnhancementPolicy struct {
	ent.Schema
}

func (EnhancementPolicy) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "evie_enhancement_policies",
		},
		entsql.WithComments(true),
		schema.Comment("文本增强策略表"),
	}
}

func (EnhancementPolicy) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(128).NotEmpty().Comment("策略名称"),
		field.String("mode").Default("STANDARD").MaxLen(32).Comment("模式: HIGH_PERFORMANCE/STANDARD/HIGH_ACCURACY"),
		field.Bool("text_cleaning").Default(true).Comment("文本清洗开关"),
		field.Bool("filler_removal").Default(true).Comment("口水词处理开关"),
		field.Bool("alias_resolution").Default(true).Comment("别名解析开关"),
		field.Bool("deterministic_replacement").Default(true).Comment("确定性替换开关"),
		field.Bool("pinyin_correction").Default(true).Comment("拼音纠错开关"),
		field.Bool("fuzzy_matching").Default(true).Comment("模糊匹配开关"),
		field.Bool("context_correction").Default(false).Comment("上下文纠错开关"),
		field.String("description").MaxLen(512).Optional().Comment("描述"),
	}
}

func (EnhancementPolicy) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (EnhancementPolicy) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("profiles", EnhancementProfile.Type).Comment("绑定场景列表"),
	}
}

func (EnhancementPolicy) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "name").Unique(),
	}
}
