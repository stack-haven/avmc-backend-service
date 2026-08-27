package schema

import (
	"backend-service/app/evie/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// DictionaryVersion 表示词库的一个发布版本。
// snapshot 是词条/关系的时点快照（JSON 复制语义，非引用），保证同一次文本增强
// 处理过程中使用的语言知识一致；词库更新后发布新版本，旧版本 Context 失效。
type DictionaryVersion struct {
	ent.Schema
}

func (DictionaryVersion) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "evie_dictionary_versions",
		},
		entsql.WithComments(true),
		schema.Comment("词库版本表：词库发布后的时点快照"),
	}
}

func (DictionaryVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("dictionary_id").Positive().Comment("所属词库 ID"),
		field.Int32("version_no").Default(1).Comment("版本号，递增"),
		field.String("snapshot").SchemaType(map[string]string{
			dialect.MySQL: "mediumtext",
		}).Optional().Comment("词条/关系时点快照（JSON）"),
		field.String("description").MaxLen(512).Optional().Comment("版本说明"),
	}
}

func (DictionaryVersion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("dictionary", Dictionary.Type).Ref("versions").Field("dictionary_id").Unique().Required().Comment("所属词库"),
	}
}

func (DictionaryVersion) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (DictionaryVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "dictionary_id", "version_no").Unique(),
	}
}
