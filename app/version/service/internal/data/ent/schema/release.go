package schema

import "entgo.io/ent"

// Release holds the schema definition for the Release entity.
type Release struct {
	ent.Schema
}

// Fields of the Release.
func (Release) Fields() []ent.Field {
	return nil
}

// Edges of the Release.
func (Release) Edges() []ent.Edge {
	return nil
}
