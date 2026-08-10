// Package grpc provides a gRPC-based Authorizer implementation
// that delegates authorization decisions to the platform's AuthService.
package grpc

import (
	"context"

	pb "backend-service/api/core/service/v1"
	"backend-service/pkg/auth/authz"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var _ authz.Authorizer = (*Authorizer)(nil)

// Authorizer delegates Enforce calls to the platform's AuthService.IsAuthorized RPC.
type Authorizer struct {
	client pb.AuthServiceClient
	conn   *grpc.ClientConn
	opts   authz.Options
}

// New creates a gRPC-based Authorizer connected to the platform auth service.
func New(ctx context.Context, endpoint string, opts ...authz.Option) (*Authorizer, error) {
	conn, err := grpc.NewClient(endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	a := &Authorizer{
		client: pb.NewAuthServiceClient(conn),
		conn:   conn,
	}
	for _, o := range opts {
		o(&a.opts)
	}
	return a, nil
}

func (a *Authorizer) Enforce(ctx context.Context, sub authz.Subject, obj authz.Object, act authz.Action, tenant authz.Tenant) (bool, error) {
	_, err := a.client.IsAuthorized(ctx, &pb.IsAuthorizedRequest{
		Subject:  string(sub),
		Resource: string(obj),
		Action:   string(act),
		Project:  string(tenant),
	})
	if err != nil {
		return false, err
	}
	return true, nil
}

func (a *Authorizer) Init(ctx context.Context, opts ...authz.Option) error {
	for _, o := range opts {
		o(&a.opts)
	}
	return nil
}

func (a *Authorizer) Name() string          { return "grpc" }
func (a *Authorizer) Options() authz.Options { return a.opts }
func (a *Authorizer) Close() error          { return a.conn.Close() }

// ──────────────── Stubs for unimplemented methods ────────────────

func (a *Authorizer) BatchEnforce(ctx context.Context, subs []authz.Subject, objs []authz.Object, acts []authz.Action, tenants []authz.Tenant) ([]bool, error) {
	result := make([]bool, len(subs))
	for i := range subs {
		ok, err := a.Enforce(ctx, subs[i], objs[i], acts[i], tenants[i])
		if err != nil {
			return nil, err
		}
		result[i] = ok
	}
	return result, nil
}

func (a *Authorizer) AddPolicy(ctx context.Context, p authz.Policy) (bool, error) {
	return false, nil
}
func (a *Authorizer) RemovePolicy(ctx context.Context, p authz.Policy) (bool, error) {
	return false, nil
}
func (a *Authorizer) AddPolicies(ctx context.Context, ps []authz.Policy) (bool, error) { return false, nil }
func (a *Authorizer) RemovePolicies(ctx context.Context, ps []authz.Policy) (bool, error) {
	return false, nil
}
func (a *Authorizer) GetAllSubjects(ctx context.Context) ([]authz.Subject, error) {
	return nil, nil
}
func (a *Authorizer) GetAllObjects(ctx context.Context) ([]authz.Object, error) { return nil, nil }
func (a *Authorizer) GetAllActions(ctx context.Context) ([]authz.Action, error) { return nil, nil }
func (a *Authorizer) GetAllTenants(ctx context.Context) ([]authz.Tenant, error) { return nil, nil }
func (a *Authorizer) GetAllRoles(ctx context.Context) ([]authz.Subject, error) { return nil, nil }
func (a *Authorizer) GetRolesForUser(ctx context.Context, user authz.Subject, tenant authz.Tenant) ([]authz.Subject, error) {
	return nil, nil
}
func (a *Authorizer) GetUsersForRole(ctx context.Context, role authz.Subject, tenant authz.Tenant) ([]authz.Subject, error) {
	return nil, nil
}
func (a *Authorizer) HasRoleForUser(ctx context.Context, user authz.Subject, role authz.Subject, tenant authz.Tenant) (bool, error) {
	return false, nil
}
func (a *Authorizer) AddRoleForUser(ctx context.Context, user authz.Subject, role authz.Subject, tenant authz.Tenant) (bool, error) {
	return false, nil
}
func (a *Authorizer) DeleteRoleForUser(ctx context.Context, user authz.Subject, role authz.Subject, tenant authz.Tenant) (bool, error) {
	return false, nil
}
