// Code generated by ent, DO NOT EDIT.

package intercept

import (
	"context"
	"fmt"

	"backend-service/app/avmc/admin/internal/data/ent"
	"backend-service/app/avmc/admin/internal/data/ent/dept"
	"backend-service/app/avmc/admin/internal/data/ent/menu"
	"backend-service/app/avmc/admin/internal/data/ent/post"
	"backend-service/app/avmc/admin/internal/data/ent/predicate"
	"backend-service/app/avmc/admin/internal/data/ent/role"
	"backend-service/app/avmc/admin/internal/data/ent/user"

	"entgo.io/ent/dialect/sql"
)

// The Query interface represents an operation that queries a graph.
// By using this interface, users can write generic code that manipulates
// query builders of different types.
type Query interface {
	// Type returns the string representation of the query type.
	Type() string
	// Limit the number of records to be returned by this query.
	Limit(int)
	// Offset to start from.
	Offset(int)
	// Unique configures the query builder to filter duplicate records.
	Unique(bool)
	// Order specifies how the records should be ordered.
	Order(...func(*sql.Selector))
	// WhereP appends storage-level predicates to the query builder. Using this method, users
	// can use type-assertion to append predicates that do not depend on any generated package.
	WhereP(...func(*sql.Selector))
}

// The Func type is an adapter that allows ordinary functions to be used as interceptors.
// Unlike traversal functions, interceptors are skipped during graph traversals. Note that the
// implementation of Func is different from the one defined in entgo.io/ent.InterceptFunc.
type Func func(context.Context, Query) error

// Intercept calls f(ctx, q) and then applied the next Querier.
func (f Func) Intercept(next ent.Querier) ent.Querier {
	return ent.QuerierFunc(func(ctx context.Context, q ent.Query) (ent.Value, error) {
		query, err := NewQuery(q)
		if err != nil {
			return nil, err
		}
		if err := f(ctx, query); err != nil {
			return nil, err
		}
		return next.Query(ctx, q)
	})
}

// The TraverseFunc type is an adapter to allow the use of ordinary function as Traverser.
// If f is a function with the appropriate signature, TraverseFunc(f) is a Traverser that calls f.
type TraverseFunc func(context.Context, Query) error

// Intercept is a dummy implementation of Intercept that returns the next Querier in the pipeline.
func (f TraverseFunc) Intercept(next ent.Querier) ent.Querier {
	return next
}

// Traverse calls f(ctx, q).
func (f TraverseFunc) Traverse(ctx context.Context, q ent.Query) error {
	query, err := NewQuery(q)
	if err != nil {
		return err
	}
	return f(ctx, query)
}

// The DeptFunc type is an adapter to allow the use of ordinary function as a Querier.
type DeptFunc func(context.Context, *ent.DeptQuery) (ent.Value, error)

// Query calls f(ctx, q).
func (f DeptFunc) Query(ctx context.Context, q ent.Query) (ent.Value, error) {
	if q, ok := q.(*ent.DeptQuery); ok {
		return f(ctx, q)
	}
	return nil, fmt.Errorf("unexpected query type %T. expect *ent.DeptQuery", q)
}

// The TraverseDept type is an adapter to allow the use of ordinary function as Traverser.
type TraverseDept func(context.Context, *ent.DeptQuery) error

// Intercept is a dummy implementation of Intercept that returns the next Querier in the pipeline.
func (f TraverseDept) Intercept(next ent.Querier) ent.Querier {
	return next
}

// Traverse calls f(ctx, q).
func (f TraverseDept) Traverse(ctx context.Context, q ent.Query) error {
	if q, ok := q.(*ent.DeptQuery); ok {
		return f(ctx, q)
	}
	return fmt.Errorf("unexpected query type %T. expect *ent.DeptQuery", q)
}

// The MenuFunc type is an adapter to allow the use of ordinary function as a Querier.
type MenuFunc func(context.Context, *ent.MenuQuery) (ent.Value, error)

// Query calls f(ctx, q).
func (f MenuFunc) Query(ctx context.Context, q ent.Query) (ent.Value, error) {
	if q, ok := q.(*ent.MenuQuery); ok {
		return f(ctx, q)
	}
	return nil, fmt.Errorf("unexpected query type %T. expect *ent.MenuQuery", q)
}

// The TraverseMenu type is an adapter to allow the use of ordinary function as Traverser.
type TraverseMenu func(context.Context, *ent.MenuQuery) error

// Intercept is a dummy implementation of Intercept that returns the next Querier in the pipeline.
func (f TraverseMenu) Intercept(next ent.Querier) ent.Querier {
	return next
}

// Traverse calls f(ctx, q).
func (f TraverseMenu) Traverse(ctx context.Context, q ent.Query) error {
	if q, ok := q.(*ent.MenuQuery); ok {
		return f(ctx, q)
	}
	return fmt.Errorf("unexpected query type %T. expect *ent.MenuQuery", q)
}

// The PostFunc type is an adapter to allow the use of ordinary function as a Querier.
type PostFunc func(context.Context, *ent.PostQuery) (ent.Value, error)

// Query calls f(ctx, q).
func (f PostFunc) Query(ctx context.Context, q ent.Query) (ent.Value, error) {
	if q, ok := q.(*ent.PostQuery); ok {
		return f(ctx, q)
	}
	return nil, fmt.Errorf("unexpected query type %T. expect *ent.PostQuery", q)
}

// The TraversePost type is an adapter to allow the use of ordinary function as Traverser.
type TraversePost func(context.Context, *ent.PostQuery) error

// Intercept is a dummy implementation of Intercept that returns the next Querier in the pipeline.
func (f TraversePost) Intercept(next ent.Querier) ent.Querier {
	return next
}

// Traverse calls f(ctx, q).
func (f TraversePost) Traverse(ctx context.Context, q ent.Query) error {
	if q, ok := q.(*ent.PostQuery); ok {
		return f(ctx, q)
	}
	return fmt.Errorf("unexpected query type %T. expect *ent.PostQuery", q)
}

// The RoleFunc type is an adapter to allow the use of ordinary function as a Querier.
type RoleFunc func(context.Context, *ent.RoleQuery) (ent.Value, error)

// Query calls f(ctx, q).
func (f RoleFunc) Query(ctx context.Context, q ent.Query) (ent.Value, error) {
	if q, ok := q.(*ent.RoleQuery); ok {
		return f(ctx, q)
	}
	return nil, fmt.Errorf("unexpected query type %T. expect *ent.RoleQuery", q)
}

// The TraverseRole type is an adapter to allow the use of ordinary function as Traverser.
type TraverseRole func(context.Context, *ent.RoleQuery) error

// Intercept is a dummy implementation of Intercept that returns the next Querier in the pipeline.
func (f TraverseRole) Intercept(next ent.Querier) ent.Querier {
	return next
}

// Traverse calls f(ctx, q).
func (f TraverseRole) Traverse(ctx context.Context, q ent.Query) error {
	if q, ok := q.(*ent.RoleQuery); ok {
		return f(ctx, q)
	}
	return fmt.Errorf("unexpected query type %T. expect *ent.RoleQuery", q)
}

// The UserFunc type is an adapter to allow the use of ordinary function as a Querier.
type UserFunc func(context.Context, *ent.UserQuery) (ent.Value, error)

// Query calls f(ctx, q).
func (f UserFunc) Query(ctx context.Context, q ent.Query) (ent.Value, error) {
	if q, ok := q.(*ent.UserQuery); ok {
		return f(ctx, q)
	}
	return nil, fmt.Errorf("unexpected query type %T. expect *ent.UserQuery", q)
}

// The TraverseUser type is an adapter to allow the use of ordinary function as Traverser.
type TraverseUser func(context.Context, *ent.UserQuery) error

// Intercept is a dummy implementation of Intercept that returns the next Querier in the pipeline.
func (f TraverseUser) Intercept(next ent.Querier) ent.Querier {
	return next
}

// Traverse calls f(ctx, q).
func (f TraverseUser) Traverse(ctx context.Context, q ent.Query) error {
	if q, ok := q.(*ent.UserQuery); ok {
		return f(ctx, q)
	}
	return fmt.Errorf("unexpected query type %T. expect *ent.UserQuery", q)
}

// NewQuery returns the generic Query interface for the given typed query.
func NewQuery(q ent.Query) (Query, error) {
	switch q := q.(type) {
	case *ent.DeptQuery:
		return &query[*ent.DeptQuery, predicate.Dept, dept.OrderOption]{typ: ent.TypeDept, tq: q}, nil
	case *ent.MenuQuery:
		return &query[*ent.MenuQuery, predicate.Menu, menu.OrderOption]{typ: ent.TypeMenu, tq: q}, nil
	case *ent.PostQuery:
		return &query[*ent.PostQuery, predicate.Post, post.OrderOption]{typ: ent.TypePost, tq: q}, nil
	case *ent.RoleQuery:
		return &query[*ent.RoleQuery, predicate.Role, role.OrderOption]{typ: ent.TypeRole, tq: q}, nil
	case *ent.UserQuery:
		return &query[*ent.UserQuery, predicate.User, user.OrderOption]{typ: ent.TypeUser, tq: q}, nil
	default:
		return nil, fmt.Errorf("unknown query type %T", q)
	}
}

type query[T any, P ~func(*sql.Selector), R ~func(*sql.Selector)] struct {
	typ string
	tq  interface {
		Limit(int) T
		Offset(int) T
		Unique(bool) T
		Order(...R) T
		Where(...P) T
	}
}

func (q query[T, P, R]) Type() string {
	return q.typ
}

func (q query[T, P, R]) Limit(limit int) {
	q.tq.Limit(limit)
}

func (q query[T, P, R]) Offset(offset int) {
	q.tq.Offset(offset)
}

func (q query[T, P, R]) Unique(unique bool) {
	q.tq.Unique(unique)
}

func (q query[T, P, R]) Order(orders ...func(*sql.Selector)) {
	rs := make([]R, len(orders))
	for i := range orders {
		rs[i] = orders[i]
	}
	q.tq.Order(rs...)
}

func (q query[T, P, R]) WhereP(ps ...func(*sql.Selector)) {
	p := make([]P, len(ps))
	for i := range ps {
		p[i] = ps[i]
	}
	q.tq.Where(p...)
}
