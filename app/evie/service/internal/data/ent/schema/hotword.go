package schema

import (
	"backend-service/app/evie/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Hotword holds the schema definition for the ASR hotword.
type Hotword struct {
	ent.Schema
}

func (Hotword) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "evie_hotwords",
		},
		entsql.WithComments(true),
		schema.Comment("ASR 热词表"),
	}
}

// Fields of the Hotword.
func (Hotword) Fields() []ent.Field {
	return []ent.Field{
		field.String("word").MaxLen(64).NotEmpty().Comment("热词原文"),
		field.String("target").MaxLen(64).Optional().Comment("期望识别结果，空=用 word"),
		field.Float("weight").Default(5.0).Comment("权重 0-10"),
		field.String("category").Default("term").MaxLen(32).Comment("person/org/product/term"),
	}
}

// Mixin of the Hotword.
func (Hotword) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.BaseMixin{}} // 无软删除，直接物理删除
}

// Indexes of the Hotword.
func (Hotword) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "word").Unique(),
		index.Fields("tenant_id", "category"),
	}
}
