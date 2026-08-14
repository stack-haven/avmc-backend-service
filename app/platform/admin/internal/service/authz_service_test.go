package service

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/go-kratos/kratos/v2/log"

	corepb "backend-service/api/core/service/v1"
	authzEngine "backend-service/pkg/auth/authz"
)

// stubAuthorizer is a minimal authz.Authorizer used to test AuthzService.IsAuthorized.
type stubAuthorizer struct {
	allowed bool
	err     error
	gotSub  authzEngine.Subject
	gotObj  authzEngine.Object
	gotAct  authzEngine.Action
	gotTen  authzEngine.Tenant
}

func (s *stubAuthorizer) Enforce(_ context.Context, sub authzEngine.Subject, obj authzEngine.Object, act authzEngine.Action, tenant authzEngine.Tenant) (bool, error) {
	s.gotSub, s.gotObj, s.gotAct, s.gotTen = sub, obj, act, tenant
	return s.allowed, s.err
}

// The following methods satisfy the authz.Authorizer interface; only Enforce is exercised.
func (s *stubAuthorizer) Init(context.Context, ...authzEngine.Option) error { return nil }
func (s *stubAuthorizer) BatchEnforce(context.Context, []authzEngine.Subject, []authzEngine.Object, []authzEngine.Action, []authzEngine.Tenant) ([]bool, error) {
	return nil, nil
}
func (s *stubAuthorizer) AddPolicy(context.Context, authzEngine.Policy) (bool, error) { return false, nil }
func (s *stubAuthorizer) RemovePolicy(context.Context, authzEngine.Policy) (bool, error) {
	return false, nil
}
func (s *stubAuthorizer) AddPolicies(context.Context, []authzEngine.Policy) (bool, error) {
	return false, nil
}
func (s *stubAuthorizer) RemovePolicies(context.Context, []authzEngine.Policy) (bool, error) {
	return false, nil
}
func (s *stubAuthorizer) GetAllSubjects(context.Context) ([]authzEngine.Subject, error) { return nil, nil }
func (s *stubAuthorizer) GetAllObjects(context.Context) ([]authzEngine.Object, error)  { return nil, nil }
func (s *stubAuthorizer) GetAllActions(context.Context) ([]authzEngine.Action, error)  { return nil, nil }
func (s *stubAuthorizer) GetAllTenants(context.Context) ([]authzEngine.Tenant, error)  { return nil, nil }
func (s *stubAuthorizer) GetAllRoles(context.Context) ([]authzEngine.Subject, error)   { return nil, nil }
func (s *stubAuthorizer) GetRolesForUser(context.Context, authzEngine.Subject, authzEngine.Tenant) ([]authzEngine.Subject, error) {
	return nil, nil
}
func (s *stubAuthorizer) GetUsersForRole(context.Context, authzEngine.Subject, authzEngine.Tenant) ([]authzEngine.Subject, error) {
	return nil, nil
}
func (s *stubAuthorizer) HasRoleForUser(context.Context, authzEngine.Subject, authzEngine.Subject, authzEngine.Tenant) (bool, error) {
	return false, nil
}
func (s *stubAuthorizer) AddRoleForUser(context.Context, authzEngine.Subject, authzEngine.Subject, authzEngine.Tenant) (bool, error) {
	return false, nil
}
func (s *stubAuthorizer) DeleteRoleForUser(context.Context, authzEngine.Subject, authzEngine.Subject, authzEngine.Tenant) (bool, error) {
	return false, nil
}
func (s *stubAuthorizer) Name() string              { return "stub" }
func (s *stubAuthorizer) Options() authzEngine.Options { return authzEngine.Options{} }
func (s *stubAuthorizer) Close() error              { return nil }

func TestAuthzServiceIsAuthorized(t *testing.T) {
	logger := log.NewStdLogger(io.Discard)

	t.Run("allowed", func(t *testing.T) {
		stub := &stubAuthorizer{allowed: true}
		svc := NewAuthzService(stub, logger)
		_, err := svc.IsAuthorized(context.Background(), &corepb.IsAuthorizedRequest{
			Subject:  "2",
			Resource: "/evie.service.v1.UserService/ListUsers",
			Action:   "GET",
			Project:  "1",
		})
		if err != nil {
			t.Fatalf("expected allowed, got error: %v", err)
		}
		if stub.gotSub != "2" || stub.gotObj != "/evie.service.v1.UserService/ListUsers" || stub.gotAct != "GET" || stub.gotTen != "1" {
			t.Fatalf("unexpected enforce args: sub=%s obj=%s act=%s tenant=%s", stub.gotSub, stub.gotObj, stub.gotAct, stub.gotTen)
		}
	})

	t.Run("denied", func(t *testing.T) {
		stub := &stubAuthorizer{allowed: false}
		svc := NewAuthzService(stub, logger)
		_, err := svc.IsAuthorized(context.Background(), &corepb.IsAuthorizedRequest{
			Subject:  "2",
			Resource: "/evie.service.v1.UserService/DeleteUser",
			Action:   "DELETE",
			Project:  "1",
		})
		if err == nil {
			t.Fatal("expected permission denied error")
		}
	})

	t.Run("authorizer error", func(t *testing.T) {
		stub := &stubAuthorizer{allowed: false, err: errors.New("backend unavailable")}
		svc := NewAuthzService(stub, logger)
		_, err := svc.IsAuthorized(context.Background(), &corepb.IsAuthorizedRequest{
			Subject:  "2",
			Resource: "/evie.service.v1.UserService/ListUsers",
			Action:   "GET",
			Project:  "1",
		})
		if err == nil {
			t.Fatal("expected error")
		}
	})

	t.Run("incomplete request", func(t *testing.T) {
		stub := &stubAuthorizer{allowed: true}
		svc := NewAuthzService(stub, logger)
		_, err := svc.IsAuthorized(context.Background(), &corepb.IsAuthorizedRequest{
			Subject: "2",
		})
		if err == nil {
			t.Fatal("expected bad request error for incomplete request")
		}
	})
}
