package mixins

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
)

var _ ent.Mixin = (*TenantID)(nil)

// TenantID 为 ai 服务提供租户维度字段与隐私策略。
//
// 与 pkg/entgo/mixin 的历史 TenantID 的关键差别：**没有 DefaultFunc**。
// 带雪花默认值时，忘记注入租户的写入会静默拿到一个随机租户，行变成谁都查不到的
// 孤儿；没有默认值时，漏注入会被非空/正值校验硬失败，正是我们要的 fail-fast。
type TenantID struct {
	mixin.Schema
}

func (TenantID) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("tenant_id").
			Comment("租户ID").
			Positive().
			StructTag(`json:"tenant_id,omitempty"`).
			SchemaType(map[string]string{
				dialect.MySQL:    "bigint",
				dialect.Postgres: "bigint",
			}),
	}
}

func (TenantID) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("tenant_id"),
	}
}
