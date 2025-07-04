// Code generated by ent, DO NOT EDIT.

package ent

import (
	"backend-service/app/version/service/internal/data/ent/predicate"
	"context"
	"errors"
	"fmt"
	"sync"

	"entgo.io/ent"
	"entgo.io/ent/dialect/sql"
)

const (
	// Operation types.
	OpCreate    = ent.OpCreate
	OpDelete    = ent.OpDelete
	OpDeleteOne = ent.OpDeleteOne
	OpUpdate    = ent.OpUpdate
	OpUpdateOne = ent.OpUpdateOne

	// Node types.
	TypeRelease = "Release"
)

// ReleaseMutation represents an operation that mutates the Release nodes in the graph.
type ReleaseMutation struct {
	config
	op            Op
	typ           string
	id            *int
	clearedFields map[string]struct{}
	done          bool
	oldValue      func(context.Context) (*Release, error)
	predicates    []predicate.Release
}

var _ ent.Mutation = (*ReleaseMutation)(nil)

// releaseOption allows management of the mutation configuration using functional options.
type releaseOption func(*ReleaseMutation)

// newReleaseMutation creates new mutation for the Release entity.
func newReleaseMutation(c config, op Op, opts ...releaseOption) *ReleaseMutation {
	m := &ReleaseMutation{
		config:        c,
		op:            op,
		typ:           TypeRelease,
		clearedFields: make(map[string]struct{}),
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// withReleaseID sets the ID field of the mutation.
func withReleaseID(id int) releaseOption {
	return func(m *ReleaseMutation) {
		var (
			err   error
			once  sync.Once
			value *Release
		)
		m.oldValue = func(ctx context.Context) (*Release, error) {
			once.Do(func() {
				if m.done {
					err = errors.New("querying old values post mutation is not allowed")
				} else {
					value, err = m.Client().Release.Get(ctx, id)
				}
			})
			return value, err
		}
		m.id = &id
	}
}

// withRelease sets the old Release of the mutation.
func withRelease(node *Release) releaseOption {
	return func(m *ReleaseMutation) {
		m.oldValue = func(context.Context) (*Release, error) {
			return node, nil
		}
		m.id = &node.ID
	}
}

// Client returns a new `ent.Client` from the mutation. If the mutation was
// executed in a transaction (ent.Tx), a transactional client is returned.
func (m ReleaseMutation) Client() *Client {
	client := &Client{config: m.config}
	client.init()
	return client
}

// Tx returns an `ent.Tx` for mutations that were executed in transactions;
// it returns an error otherwise.
func (m ReleaseMutation) Tx() (*Tx, error) {
	if _, ok := m.driver.(*txDriver); !ok {
		return nil, errors.New("ent: mutation is not running in a transaction")
	}
	tx := &Tx{config: m.config}
	tx.init()
	return tx, nil
}

// ID returns the ID value in the mutation. Note that the ID is only available
// if it was provided to the builder or after it was returned from the database.
func (m *ReleaseMutation) ID() (id int, exists bool) {
	if m.id == nil {
		return
	}
	return *m.id, true
}

// IDs queries the database and returns the entity ids that match the mutation's predicate.
// That means, if the mutation is applied within a transaction with an isolation level such
// as sql.LevelSerializable, the returned ids match the ids of the rows that will be updated
// or updated by the mutation.
func (m *ReleaseMutation) IDs(ctx context.Context) ([]int, error) {
	switch {
	case m.op.Is(OpUpdateOne | OpDeleteOne):
		id, exists := m.ID()
		if exists {
			return []int{id}, nil
		}
		fallthrough
	case m.op.Is(OpUpdate | OpDelete):
		return m.Client().Release.Query().Where(m.predicates...).IDs(ctx)
	default:
		return nil, fmt.Errorf("IDs is not allowed on %s operations", m.op)
	}
}

// Where appends a list predicates to the ReleaseMutation builder.
func (m *ReleaseMutation) Where(ps ...predicate.Release) {
	m.predicates = append(m.predicates, ps...)
}

// WhereP appends storage-level predicates to the ReleaseMutation builder. Using this method,
// users can use type-assertion to append predicates that do not depend on any generated package.
func (m *ReleaseMutation) WhereP(ps ...func(*sql.Selector)) {
	p := make([]predicate.Release, len(ps))
	for i := range ps {
		p[i] = ps[i]
	}
	m.Where(p...)
}

// Op returns the operation name.
func (m *ReleaseMutation) Op() Op {
	return m.op
}

// SetOp allows setting the mutation operation.
func (m *ReleaseMutation) SetOp(op Op) {
	m.op = op
}

// Type returns the node type of this mutation (Release).
func (m *ReleaseMutation) Type() string {
	return m.typ
}

// Fields returns all fields that were changed during this mutation. Note that in
// order to get all numeric fields that were incremented/decremented, call
// AddedFields().
func (m *ReleaseMutation) Fields() []string {
	fields := make([]string, 0, 0)
	return fields
}

// Field returns the value of a field with the given name. The second boolean
// return value indicates that this field was not set, or was not defined in the
// schema.
func (m *ReleaseMutation) Field(name string) (ent.Value, bool) {
	return nil, false
}

// OldField returns the old value of the field from the database. An error is
// returned if the mutation operation is not UpdateOne, or the query to the
// database failed.
func (m *ReleaseMutation) OldField(ctx context.Context, name string) (ent.Value, error) {
	return nil, fmt.Errorf("unknown Release field %s", name)
}

// SetField sets the value of a field with the given name. It returns an error if
// the field is not defined in the schema, or if the type mismatched the field
// type.
func (m *ReleaseMutation) SetField(name string, value ent.Value) error {
	switch name {
	}
	return fmt.Errorf("unknown Release field %s", name)
}

// AddedFields returns all numeric fields that were incremented/decremented during
// this mutation.
func (m *ReleaseMutation) AddedFields() []string {
	return nil
}

// AddedField returns the numeric value that was incremented/decremented on a field
// with the given name. The second boolean return value indicates that this field
// was not set, or was not defined in the schema.
func (m *ReleaseMutation) AddedField(name string) (ent.Value, bool) {
	return nil, false
}

// AddField adds the value to the field with the given name. It returns an error if
// the field is not defined in the schema, or if the type mismatched the field
// type.
func (m *ReleaseMutation) AddField(name string, value ent.Value) error {
	return fmt.Errorf("unknown Release numeric field %s", name)
}

// ClearedFields returns all nullable fields that were cleared during this
// mutation.
func (m *ReleaseMutation) ClearedFields() []string {
	return nil
}

// FieldCleared returns a boolean indicating if a field with the given name was
// cleared in this mutation.
func (m *ReleaseMutation) FieldCleared(name string) bool {
	_, ok := m.clearedFields[name]
	return ok
}

// ClearField clears the value of the field with the given name. It returns an
// error if the field is not defined in the schema.
func (m *ReleaseMutation) ClearField(name string) error {
	return fmt.Errorf("unknown Release nullable field %s", name)
}

// ResetField resets all changes in the mutation for the field with the given name.
// It returns an error if the field is not defined in the schema.
func (m *ReleaseMutation) ResetField(name string) error {
	return fmt.Errorf("unknown Release field %s", name)
}

// AddedEdges returns all edge names that were set/added in this mutation.
func (m *ReleaseMutation) AddedEdges() []string {
	edges := make([]string, 0, 0)
	return edges
}

// AddedIDs returns all IDs (to other nodes) that were added for the given edge
// name in this mutation.
func (m *ReleaseMutation) AddedIDs(name string) []ent.Value {
	return nil
}

// RemovedEdges returns all edge names that were removed in this mutation.
func (m *ReleaseMutation) RemovedEdges() []string {
	edges := make([]string, 0, 0)
	return edges
}

// RemovedIDs returns all IDs (to other nodes) that were removed for the edge with
// the given name in this mutation.
func (m *ReleaseMutation) RemovedIDs(name string) []ent.Value {
	return nil
}

// ClearedEdges returns all edge names that were cleared in this mutation.
func (m *ReleaseMutation) ClearedEdges() []string {
	edges := make([]string, 0, 0)
	return edges
}

// EdgeCleared returns a boolean which indicates if the edge with the given name
// was cleared in this mutation.
func (m *ReleaseMutation) EdgeCleared(name string) bool {
	return false
}

// ClearEdge clears the value of the edge with the given name. It returns an error
// if that edge is not defined in the schema.
func (m *ReleaseMutation) ClearEdge(name string) error {
	return fmt.Errorf("unknown Release unique edge %s", name)
}

// ResetEdge resets all changes to the edge with the given name in this mutation.
// It returns an error if the edge is not defined in the schema.
func (m *ReleaseMutation) ResetEdge(name string) error {
	return fmt.Errorf("unknown Release edge %s", name)
}
