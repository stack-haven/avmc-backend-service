package schema

import (
	"backend-service/app/evie/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// DictionaryCategory 表示词条的分类（性质），如 PERSON/ORGANIZATION/PRODUCT。
// Category 只描述词条性质，不决定处理行为——处理方式由 DictionaryRelation + Policy 决定。
// 内置分类 tenant_id=0（全局共享），自定义分类 tenant_id>0（租户级）。
type DictionaryCategory struct {
	ent.Schema
}

func (DictionaryCategory) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "evie_dictionary_categories",
		},
		entsql.WithComments(true),
		schema.Comment("词条分类表：词条性质，非处理行为"),
	}
}

func (DictionaryCategory) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").MaxLen(64).NotEmpty().Comment("分类编码: PERSON/ORGANIZATION/PRODUCT/LOCATION/PERSON_TITLE/BUSINESS_TERM/TECH_TERM/OTHER"),
		field.String("name").MaxLen(128).NotEmpty().Comment("分类名称"),
		field.Bool("builtin").Default(false).Comment("是否内置分类（内置只读）"),
		field.Int32("sort").Default(0).Comment("排序"),
	}
}

func (DictionaryCategory) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
	}
}

func (DictionaryCategory) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "code").Unique(),
	}
}
