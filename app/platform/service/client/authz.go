// Package client 提供 Ark Platform 服务的 gRPC 客户端 SDK，供产品服务（evie 等）复用。
// 客户端依赖 platform 的 pb 契约，与 platform 服务放在一起，避免 pkg 反向依赖 api 造成循环。
package client

import (
	"context"

	pb "backend-service/api/platform/service/v1"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/auth/authz"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var _ authz.Authorizer = (*Authorizer)(nil)

// Authorizer 通过 gRPC 委托 platform 的 AuthService.IsAuthorized 做鉴权决策。
type Authorizer struct {
	client pb.AuthServiceClient
	conn   *grpc.ClientConn
	opts   authz.Options
}

// NewAuthorizer 创建 gRPC 鉴权委托客户端，连接 platform auth 服务。
func NewAuthorizer(ctx context.Context, endpoint string, opts ...authz.Option) (*Authorizer, error) {
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
	// 原样转发当前请求的 Authorization（含 "Bearer " 前缀），让 platform 独立验证 JWT，
	// 而非只信任 sub/tenant 参数（防产品服务被攻破后伪造身份）。
	ctx = authn.ForwardAuthToken(ctx)
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

func (a *Authorizer) Name() string           { return "grpc" }
func (a *Authorizer) Options() authz.Options { return a.opts }
func (a *Authorizer) Close() error           { return a.conn.Close() }

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
func (a *Authorizer) AddPolicies(ctx context.Context, ps []authz.Policy) (bool, error) {
	return false, nil
}
func (a *Authorizer) RemovePolicies(ctx context.Context, ps []authz.Policy) (bool, error) {
	return false, nil
}
func (a *Authorizer) GetAllSubjects(ctx context.Context) ([]authz.Subject, error) {
	return nil, nil
}
func (a *Authorizer) GetAllObjects(ctx context.Context) ([]authz.Object, error) { return nil, nil }
func (a *Authorizer) GetAllActions(ctx context.Context) ([]authz.Action, error) { return nil, nil }
func (a *Authorizer) GetAllTenants(ctx context.Context) ([]authz.Tenant, error) { return nil, nil }
func (a *Authorizer) GetAllRoles(ctx context.Context) ([]authz.Subject, error)  { return nil, nil }
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
