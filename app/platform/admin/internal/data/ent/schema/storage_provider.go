package schema

import (
	"backend-service/app/platform/admin/internal/data/ent/mixins"
	pkgMixin "backend-service/pkg/entgo/mixin"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// StorageProvider holds platform-owned file storage channel configuration.
type StorageProvider struct {
	ent.Schema
}

func (StorageProvider) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin", Table: "system_storage_providers"},
		entsql.WithComments(true),
		schema.Comment("存储渠道表"),
	}
}

func (StorageProvider) Fields() []ent.Field {
	return []ent.Field{
		field.String("code").Comment("存储渠道编码").MaxLen(80).NotEmpty(),
		field.String("name").Comment("存储渠道名称").MaxLen(120).NotEmpty(),
		field.String("type").Comment("存储类型 s3-compatible/local").MaxLen(40).NotEmpty(),
		field.String("endpoint").Comment("S3 endpoint 或本地兼容地址").MaxLen(255).Default(""),
		field.String("region").Comment("S3 region").MaxLen(80).Default(""),
		field.String("access_key").Comment("访问密钥").MaxLen(120).Default(""),
		field.String("secret_key").Comment("密钥").MaxLen(200).Default(""),
		field.String("session_token").Comment("会话令牌").MaxLen(500).Default(""),
		field.Bool("use_ssl").Comment("是否使用 SSL").Default(false),
		field.Bool("force_path_style").Comment("是否强制 path-style").Default(true),
		field.String("public_base_url").Comment("公开访问或代理访问基础 URL").MaxLen(255).Default(""),
		field.String("default_bucket").Comment("默认 bucket").MaxLen(120).Default("tenant-files"),
		field.String("local_base_path").Comment("本地存储根目录").MaxLen(500).Default(""),
		field.Bool("is_default").Comment("是否默认渠道").Default(false),
		field.String("health_status").Comment("健康状态 unknown/healthy/unhealthy").MaxLen(20).Default("unknown"),
		field.Time("last_checked_at").Comment("最后健康检查时间").Optional().Nillable(),
		field.String("remark").Comment("备注").MaxLen(500).Default(""),
	}
}

func (StorageProvider) Mixin() []ent.Mixin {
	return []ent.Mixin{
		pkgMixin.ID{},
		pkgMixin.Status{},
		pkgMixin.CreatedAt{},
		pkgMixin.UpdatedAt{},
		mixins.SoftDeleteMixin{},
	}
}

func (StorageProvider) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("code").Unique(),
		index.Fields("type", "status"),
		index.Fields("is_default"),
	}
}
