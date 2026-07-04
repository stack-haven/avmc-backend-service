package schema

import (
	"backend-service/app/platform/admin/internal/data/ent/mixins"
	"backend-service/app/platform/admin/internal/data/ent/rule"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type DictionaryItem struct{ ent.Schema }

func (DictionaryItem) Annotations() []schema.Annotation {
	return []schema.Annotation{entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin"}, entsql.WithComments(true), schema.Comment("租户数据字典项表")}
}
func (DictionaryItem) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("type_id").Positive().Comment("字典类型ID"),
		field.String("label").MaxLen(50).Comment("显示标签"),
		field.String("value").MaxLen(100).Comment("字典值"),
		field.Int32("sort").Default(10).Comment("排序"),
		field.String("color").MaxLen(32).Default("").Comment("展示颜色"),
		field.String("remark").MaxLen(255).Default("").Comment("备注"),
	}
}
func (DictionaryItem) Edges() []ent.Edge {
	return []ent.Edge{edge.From("type", DictionaryType.Type).Ref("items").Field("type_id").Unique().Required()}
}
func (DictionaryItem) Mixin() []ent.Mixin {
	return []ent.Mixin{mixins.BaseMixin{}, mixins.SoftDeleteMixin{}}
}
func (DictionaryItem) Hooks() []ent.Hook {
	return []ent.Hook{rule.DictionaryItemTenantHook()}
}
func (DictionaryItem) Indexes() []ent.Index {
	return []ent.Index{index.Fields("tenant_id", "type_id", "value").Unique()}
}
