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

// DictionaryWord holds the schema definition for the dictionary standard word.
type DictionaryWord struct {
	ent.Schema
}

func (DictionaryWord) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "evie_dictionary_words",
		},
		entsql.WithComments(true),
		schema.Comment("语音字典标准词表"),
	}
}

// Fields of the DictionaryWord.
func (DictionaryWord) Fields() []ent.Field {
	return []ent.Field{
		field.String("word").MaxLen(128).NotEmpty().Comment("标准词"),
		// level 表示字典层级（四级字典模型），与 category（实体分类）正交：
		//   platform/system → tenant_id=0（所有租户共享）
		//   tenant/user      → tenant_id>0（租户/个人数据）
		field.String("level").Default("tenant").MaxLen(32).Comment("字典层级: platform/system/tenant/user"),
		field.String("category").Default("term").MaxLen(32).Comment("实体分类: person/org/product/term"),
		field.String("source").Default("manual").MaxLen(32).Comment("来源: manual/sync_org/sync_biz/feedback"),
		field.Int32("priority").Default(0).Comment("匹配优先级，越大越优先"),
	}
}

// Edges of the DictionaryWord.
func (DictionaryWord) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("aliases", DictionaryAlias.Type).Comment("别名列表"),
	}
}

// Mixin of the DictionaryWord.
func (DictionaryWord) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

// Indexes of the DictionaryWord.
func (DictionaryWord) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "word").Unique(),
		index.Fields("tenant_id", "level", "category"),
		index.Fields("word"),
	}
}
