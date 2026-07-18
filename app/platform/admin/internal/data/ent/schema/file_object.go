package schema

import (
	"backend-service/app/platform/admin/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// FileObject holds tenant file object metadata.
type FileObject struct {
	ent.Schema
}

func (FileObject) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Charset: "utf8mb4", Collation: "utf8mb4_bin"},
		entsql.WithComments(true),
		schema.Comment("文件中心对象元数据表"),
	}
}

func (FileObject) Fields() []ent.Field {
	return []ent.Field{
		field.String("file_name").Comment("原始文件名").MaxLen(255).NotEmpty(),
		field.String("content_type").Comment("文件MIME类型").MaxLen(120).Default("application/octet-stream"),
		field.Int64("size").Comment("文件大小").Min(0).Default(0),
		field.String("sha256").Comment("文件SHA256").MaxLen(64).Default(""),
		field.String("etag").Comment("对象存储ETag").MaxLen(120).Default(""),
		field.String("provider").Comment("存储渠道").MaxLen(50).Default("s3-compatible"),
		field.Uint32("provider_id").Comment("存储渠道ID快照").Optional().Nillable(),
		field.String("provider_code").Comment("存储渠道编码快照").MaxLen(80).Default(""),
		field.String("bucket").Comment("存储桶").MaxLen(120).NotEmpty(),
		field.String("object_key").Comment("对象Key").MaxLen(500).NotEmpty(),
		field.String("business_type").Comment("业务类型").MaxLen(80).Default(""),
		field.String("business_id").Comment("业务ID").MaxLen(120).Default(""),
		field.String("visibility").Comment("可见性 private/public").MaxLen(20).Default("private"),
		field.String("idempotency_key").Comment("创建幂等键").MaxLen(120).Optional().Nillable(),
		field.Time("upload_expires_at").Comment("上传凭证过期时间"),
		field.Time("confirmed_at").Comment("上传确认时间").Optional().Nillable(),
		field.Uint32("created_by").Comment("创建人ID").Optional().Nillable(),
	}
}

func (FileObject) Edges() []ent.Edge {
	return nil
}

func (FileObject) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

func (FileObject) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id", "object_key").Unique(),
		index.Fields("tenant_id", "idempotency_key").Unique(),
		index.Fields("provider_code", "status"),
		index.Fields("tenant_id", "business_type", "business_id"),
		index.Fields("tenant_id", "status", "created_at"),
	}
}
