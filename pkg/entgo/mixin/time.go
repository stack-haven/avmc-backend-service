package mixin

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/mixin"
)

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ ent.Mixin = (*CreatedAt)(nil)

type CreatedAt struct{ mixin.Schema }

func (CreatedAt) Fields() []ent.Field {
	return []ent.Field{
		// 创建时间
		field.Time("created_at").
			Default(time.Now).
			Immutable().
			Comment("创建时间"),
	}
}

////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

var _ ent.Mixin = (*UpdatedAt)(nil)

type UpdatedAt struct{ mixin.Schema }

func (UpdatedAt) Fields() []ent.Field {
	return []ent.Field{
		// 更新时间
		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			Comment("更新时间"),
	}
}

// 说明（2026-11-12）：本包曾同时提供 created_at/updated_at 与
// create_time/update_time 两套命名，以及 DeletedAt/TimeAt/Time 等零引用变体。
// 后者全部删除：一个共享包同时给两套列名，只会让新 schema 选错列名而不报错。
// 当前仅保留 ID/CreatedAt/UpdatedAt/Status 四个实际被引用的 mixin。
