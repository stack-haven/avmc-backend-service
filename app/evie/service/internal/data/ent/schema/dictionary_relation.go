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

// DictionaryRelation 表示词条的语言关系（别名/纠错/同音/近音/简称/相关）。
// 这是词库中心最核心的模型：一个标准词条可拥有多个语言关系。
// 关系类型决定文本增强引擎的处理方式（确定性 vs 推断性），见开发说明第八节。
type DictionaryRelation struct {
	ent.Schema
}

func (DictionaryRelation) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "evie_dictionary_relations",
		},
		entsql.WithComments(true),
		schema.Comment("词条关系表：别名/纠错/同音等语言关系"),
	}
}

func (DictionaryRelation) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("entry_id").Positive().Comment("所属词条 ID"),
		field.String("relation_type").MaxLen(32).NotEmpty().Comment("关系类型: ALIAS/CORRECTION/HOMOPHONE/PHONETIC_SIMILAR/ABBREVIATION/RELATED"),
		field.String("related_text").MaxLen(255).NotEmpty().Comment("关联表达（如别名「小田」、错误表达「金种子」）"),
		field.Uint32("target_entry_id").Optional().Comment("目标词条 ID（指向同一词库的另一词条，可选）"),
		field.String("source").Default("MANUAL").MaxLen(32).Comment("来源: MANUAL/SYNC/IMPORT/API"),
		field.String("description").MaxLen(512).Optional().Comment("描述"),
	}
}

func (DictionaryRelation) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("entry", DictionaryEntry.Type).Ref("relations").Field("entry_id").Unique().Required().Comment("所属词条"),
	}
}

func (DictionaryRelation) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (DictionaryRelation) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "entry_id", "relation_type", "related_text").Unique(),
		index.Fields("tenant_id", "relation_type"),
		index.Fields("related_text"),
	}
}
