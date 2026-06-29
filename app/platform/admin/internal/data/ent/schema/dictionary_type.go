package schema

import (
	"backend-service/app/platform/admin/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type DictionaryType struct{ ent.Schema }

func (DictionaryType) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin"}, entsql.WithComments(true), schema.Comment("租户数据字典类型表")}
}
func (DictionaryType) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(50).Comment("字典名称"),
		field.String("code").MaxLen(50).Comment("字典编码"),
		field.Int32("sort").Default(10).Comment("排序"),
		field.String("remark").MaxLen(255).Default("").Comment("备注"),
	}
}
func (DictionaryType) Edges() []ent.Edge {
	return []ent.Edge{edge.To("items", DictionaryItem.Type)}
}
func (DictionaryType) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.BaseMixin{}, mixins.SoftDeleteMixin{}}
}
func (DictionaryType) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "code").Unique()}
}
