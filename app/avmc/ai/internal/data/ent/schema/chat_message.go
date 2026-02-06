package schema

import (
	"backend-service/app/avmc/ai/internal/data/ent/mixins"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// ChatMessage holds the schema definition for the ChatMessage entity.
type ChatMessage struct {
	ent.Schema
}

func (ChatMessage) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ai_chat_messages",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("AI对话消息表"),
	}
}

// Fields of the ChatMessage.
func (ChatMessage) Fields() []ent.Field {
	return []ent.Field{
		field.Uint32("chat_id").Comment("对话ID").Default(0).Optional().Nillable(),
		field.Text("content").Comment("对话内容").Nillable(),
		field.Enum("role").Values("user", "assistant").Comment("角色").Default("user").Nillable(),
		field.Uint32("user_id").Comment("用户ID").Default(0).Optional().Nillable(),
		field.Uint32("assistant_id").Comment("助手ID").Default(0).Optional().Nillable(),
		field.Uint32("parent_id").Comment("父消息ID").Default(0).Optional().Nillable(),
	}
}

// Edges of the ChatMessage.
func (ChatMessage) Edges() []ent.Edge {
	return []ent.Edge{}
}

// Mixin of the ChatMessage.
func (ChatMessage) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

// Indexes of the ChatMessage.
func (ChatMessage) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("chat_id"),
		index.Fields("status"),
	}
}
