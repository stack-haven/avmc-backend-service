package schema

import (
	"backend-service/app/evie/service/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// CorrectionRule holds the schema definition for the correction rule.
type CorrectionRule struct {
	ent.Schema
}

func (CorrectionRule) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
			Table:     "evie_correction_rules",
		},
		entsql.WithComments(true),
		schema.Comment("纠错规则表"),
	}
}

// Fields of the CorrectionRule.
func (CorrectionRule) Fields() []ent.Field {
	return []ent.Field{
		field.String("source").MaxLen(128).NotEmpty().Comment("源词（替换前）"),
		field.String("target").MaxLen(128).NotEmpty().Comment("目标词（替换后）"),
		field.String("type").Default("dictionary").MaxLen(32).Comment("dictionary/business/system"),
		field.Int32("priority").Default(100).Comment("优先级，数字越大越先执行"),
	}
}

// Mixin of the CorrectionRule.
func (CorrectionRule) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

// Indexes of the CorrectionRule.
func (CorrectionRule) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "source").Unique(),
		index.Fields("tenant_id", "type"),
	}
}
