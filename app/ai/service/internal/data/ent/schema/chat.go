package schema

import (
	"backend-service/app/ai/service/internal/data/ent/mixins"
	"regexp"

	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// Chat holds the schema definition for the Chat entity.
type Chat struct {
	ent.Schema
}

func (Chat) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{
			Table:     "ai_chats",
			Charset:   "utf8mb4",
			Collation: "utf8mb4_bin",
		},
		entsql.WithComments(true),
		schema.Comment("AI对话表"),
	}
}

// Fields of the Chat.
func (Chat) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").Unique().MinLen(3).MaxLen(32).Match(regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)).Nillable().Comment("用户名，唯一"),
		field.Uint32("user_id").Comment("用户ID").Default(0).Optional().Nillable(),
		field.Time("last_active_time").Optional().Nillable().Comment("最后活跃时间"),
		field.String("channel").MinLen(3).MaxLen(32).Match(regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)).Nillable().Comment("对话渠道"),
	}
}

// Edges of the Chat.
func (Chat) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("chat_messages", ChatMessage.Type),
	}
}

// Mixin of the Chat.
func (Chat) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixins.BaseMixin{},
		mixins.SoftDeleteMixin{},
	}
}

// Indexes of the Chat.
func (Chat) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name"),
		index.Fields("status"),
	}
}
