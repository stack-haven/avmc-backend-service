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

// Dictionary 表示一组具有共同作用域和生命周期的语言知识集合。
// 作用域 scope：PLATFORM（平台级）/ SYSTEM（系统级）/ TENANT（租户级）。
// PLATFORM/SYSTEM 词库 tenant_id=0（所有租户共享），TENANT 词库 tenant_id>0（严格隔离）。
type Dictionary struct {
	ent.Schema
}

func (Dictionary) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "evie_dictionaries",
		},
		entsql.WithComments(true),
		schema.Comment("词库表：语言知识集合，按作用域隔离"),
	}
}

func (Dictionary) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(128).NotEmpty().Comment("词库名称"),
		field.String("scope").Default("TENANT").MaxLen(32).Comment("作用域: PLATFORM/SYSTEM/TENANT"),
		field.String("source").Default("MANUAL").MaxLen(32).Comment("来源: PLATFORM/SYSTEM/MANUAL/IMPORT/SYNC/API"),
		field.String("description").MaxLen(512).Optional().Comment("描述"),
	}
}

func (Dictionary) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("entries", DictionaryEntry.Type).Comment("词条列表"),
		edge.To("versions", DictionaryVersion.Type).Comment("版本列表"),
	}
}

func (Dictionary) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (Dictionary) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "name").Unique(),
		index.Fields("scope"),
	}
}
