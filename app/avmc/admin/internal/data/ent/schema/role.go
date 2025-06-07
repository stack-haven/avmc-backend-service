package schema

import "entgo.io/ent"

// Role holds the schema definition for the Role entity.
type Role struct {
	ent.Schema
}

// Fields of the Role.
func (Role) Fields() []ent.Field {
	return nil
}

// Edges of the Role.
func (Role) Edges() []ent.Edge {
	return nil
}
