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

// DictionaryAlias holds the schema definition for the dictionary alias.
type DictionaryAlias struct {
	ent.Schema
}

func (DictionaryAlias) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "evie_dictionary_aliases",
		},
		entsql.WithComments(true),
		schema.Comment("语音字典别名表"),
	}
}

// Fields of the DictionaryAlias.
func (DictionaryAlias) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("word_id").Positive().Comment("关联标准词ID"),
		field.String("alias").MaxLen(128).NotEmpty().Comment("别名"),
		field.String("pinyin").MaxLen(256).Optional().Comment("拼音"),
		field.Float("weight").Default(1.0).Comment("匹配权重"),
		field.String("source").Default("manual").MaxLen(32).Comment("manual/auto/feedback"),
	}
}

// Edges of the DictionaryAlias.
func (DictionaryAlias) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("word", DictionaryWord.Type).
			Ref("aliases").
			Field("word_id").
			Unique().
			Required(),
	}
}

// Mixin of the DictionaryAlias.
func (DictionaryAlias) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

// Indexes of the DictionaryAlias.
func (DictionaryAlias) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("alias"),
		index.Fields("pinyin"),
		index.Fields("word_id", "alias").Unique(),
	}
}
