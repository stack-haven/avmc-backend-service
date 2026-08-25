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

// DictionaryEntry 表示一个标准语言概念（词或短语），如「田华」「技术研发部」。
// 一个 Entry 可以有多个 DictionaryRelation（别名/纠错/同音等语言关系）。
type DictionaryEntry struct {
	ent.Schema
}

func (DictionaryEntry) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "evie_dictionary_entries",
		},
		entsql.WithComments(true),
		schema.Comment("词条表：标准语言概念（词/短语）"),
	}
}

func (DictionaryEntry) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("dictionary_id").Positive().Comment("所属词库 ID"),
		field.String("standard_text").MaxLen(255).NotEmpty().Comment("标准词/短语"),
		field.String("entry_type").Default("WORD").MaxLen(16).Comment("词条类型: WORD/PHRASE"),
		field.String("category").Default("OTHER").MaxLen(64).Comment("分类: PERSON/ORGANIZATION/PRODUCT/LOCATION/PERSON_TITLE/BUSINESS_TERM/TECH_TERM/OTHER"),
		field.String("description").MaxLen(512).Optional().Comment("描述"),
		field.String("source").Default("MANUAL").MaxLen(32).Comment("来源: PLATFORM/SYSTEM/MANUAL/IMPORT/SYNC/API"),
		field.String("source_id").MaxLen(128).Optional().Comment("外部来源 ID（业务实体 ID，仅关联不承担主数据职责）"),
		field.Int32("priority").Default(0).Comment("匹配优先级，越大越优先"),
		field.String("pinyin").MaxLen(512).Optional().Comment("全拼"),
		field.String("pinyin_initial").MaxLen(128).Optional().Comment("拼音首字母"),
		field.String("normalized_text").MaxLen(255).Optional().Comment("规范化文本（去除空白/大小写等）"),
	}
}

func (DictionaryEntry) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("dictionary", Dictionary.Type).Ref("entries").Field("dictionary_id").Unique().Required().Comment("所属词库"),
		edge.To("relations", DictionaryRelation.Type).Comment("语言关系列表"),
	}
}

func (DictionaryEntry) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (DictionaryEntry) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "dictionary_id", "standard_text").Unique(),
		index.Fields("tenant_id", "category"),
		index.Fields("standard_text"),
		index.Fields("pinyin"),
	}
}
