package mixins

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
)

var _ ent.Mixin = (*TenantID)(nil)

// TenantID 在所有 EDAI 实体上提供 tenant_id 列与租户隔离策略。
//
// EDAI V1.0 是 Local-first 单机部署（4-2 ADR-013 决策 3），运行期由中间件把
// tenant_id 固定注入为单一值；列本身保留，未来支持 LAN / 多客户时无需改表。
type TenantID struct {
	mixin.Schema
}

func (TenantID) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("tenant_id").
			Comment("租户ID（V1.0 固定单租户）").
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
