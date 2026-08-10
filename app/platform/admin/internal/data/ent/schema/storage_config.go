package schema

import (
	"backend-service/app/platform/admin/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// StorageConfig holds per-tenant object storage configuration.
// Each tenant may own multiple storage configs targeting different providers.
type StorageConfig struct {
	ent.Schema
}

func (StorageConfig) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin", Table: "storage_configs"},
		entsql.WithComments(true),
		schema.Comment("租户存储配置表"),
	}
}

func (StorageConfig) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").MaxLen(120).NotEmpty().Comment("配置名称"),
		field.String("provider").
			MaxLen(40).NotEmpty().
			Comment("s3-compatible / aliyun-oss / tencent-cos / qiniu-kodo / local"),
		field.String("purpose").MaxLen(40).Default("").Comment("用途：default / audio / backup"),
		field.String("bucket").MaxLen(120).Default("").Comment("默认桶名"),
		field.Bool("is_default").Default(false).Comment("是否默认存储"),
		field.Text("config_json").Comment("供应商专属配置 JSON（加密存储）"),
		field.String("health_status").MaxLen(20).Default("unknown").Comment("unknown / healthy / unhealthy"),
		field.Time("last_checked_at").Optional().Nillable().Comment("最后健康检查时间"),
	}
}

func (StorageConfig) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (StorageConfig) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "name").Unique(),
		index.Fields("tenant_id", "provider"),
		index.Fields("tenant_id", "is_default"),
	}
}
