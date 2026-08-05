package biz

import (
	"context"
	"io"
	"testing"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/core/service/v1"
	"backend-service/pkg/auth/authn"
)

func TestValidateParameterDefinition(t *testing.T) {
	tests := []struct {
		name    string
		item    *pb.ParameterDefinition
		wantErr bool
	}{
		{
			name: "valid integer",
			item: &pb.ParameterDefinition{
				Key: "system.page_size", Name: "Page size",
				ValueType: pb.ParameterValueType_PARAMETER_VALUE_TYPE_INTEGER, DefaultValue: "20",
			},
		},
		{
			name: "invalid key",
			item: &pb.ParameterDefinition{
				Key: "PageSize", Name: "Page size",
				ValueType: pb.ParameterValueType_PARAMETER_VALUE_TYPE_INTEGER, DefaultValue: "20",
			},
			wantErr: true,
		},
		{
			name: "secret rejected",
			item: &pb.ParameterDefinition{
				Key: "integration.api_key", Name: "API key",
				ValueType: pb.ParameterValueType_PARAMETER_VALUE_TYPE_STRING, DefaultValue: "secret",
			},
			wantErr: true,
		},
		{
			name: "invalid boolean",
			item: &pb.ParameterDefinition{
				Key: "feature.enabled", Name: "Feature enabled",
				ValueType: pb.ParameterValueType_PARAMETER_VALUE_TYPE_BOOLEAN, DefaultValue: "yes",
			},
			wantErr: true,
		},
		{
			name: "invalid json",
			item: &pb.ParameterDefinition{
				Key: "feature.options", Name: "Feature options",
				ValueType: pb.ParameterValueType_PARAMETER_VALUE_TYPE_JSON, DefaultValue: "{",
			},
			wantErr: true,
		},
		{
			name: "token ttl is not a secret",
			item: &pb.ParameterDefinition{
				Key: "auth.access_token_ttl", Name: "Token TTL",
				ValueType: pb.ParameterValueType_PARAMETER_VALUE_TYPE_INTEGER, DefaultValue: "3600",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateParameterDefinition(test.item)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateParameterDefinition() error = %v, wantErr %t", err, test.wantErr)
			}
		})
	}
}

type parameterRepoStub struct {
	tenantID   uint32
	key        string
	value      string
	operatorID uint32
	resetID    uint32
}

func (*parameterRepoStub) ListDefinitions(context.Context, *pb.ListParameterDefinitionsRequest) ([]*pb.ParameterDefinition, int32, error) {
	return nil, 0, nil
}
func (*parameterRepoStub) GetDefinition(context.Context, uint32) (*pb.ParameterDefinition, error) {
	return nil, nil
}
func (*parameterRepoStub) CreateDefinition(context.Context, *pb.ParameterDefinition) (*pb.ParameterDefinition, error) {
	return nil, nil
}
func (*parameterRepoStub) UpdateDefinition(context.Context, *pb.ParameterDefinition) (*pb.ParameterDefinition, error) {
	return nil, nil
}
func (*parameterRepoStub) DeleteDefinition(context.Context, uint32) error { return nil }
func (r *parameterRepoStub) ListResolved(_ context.Context, tenantID uint32, key string) ([]*pb.ResolvedParameter, error) {
	r.tenantID = tenantID
	r.key = key
	return []*pb.ResolvedParameter{{Key: key}}, nil
}
func (r *parameterRepoStub) SetOverride(_ context.Context, tenantID uint32, key, value string, operatorID uint32) (*pb.ResolvedParameter, error) {
	r.tenantID = tenantID
	r.key = key
	r.value = value
	r.operatorID = operatorID
	return &pb.ResolvedParameter{Key: key, Value: value}, nil
}
func (r *parameterRepoStub) ResetOverride(_ context.Context, tenantID uint32, key string) error {
	r.resetID = tenantID
	r.key = key
	return nil
}

type parameterTestUser struct {
	subject string
	tenant  string
}

func (u parameterTestUser) Name() string                           { return "test" }
func (u parameterTestUser) ParseFromContext(context.Context) error { return nil }
func (u parameterTestUser) GetSubject() string                     { return u.subject }
func (u parameterTestUser) GetObject() string                      { return "" }
func (u parameterTestUser) GetAction() string                      { return "" }
func (u parameterTestUser) GetTenant() string                      { return u.tenant }

func TestParameterUsecaseCurrentTenantContext(t *testing.T) {
	t.Parallel()

	repo := &parameterRepoStub{}
	uc := NewParameterUsecase(repo, log.NewStdLogger(io.Discard))
	ctx := authn.ContextWithAuthUser(context.Background(), parameterTestUser{subject: "7", tenant: "10"})

	if _, err := uc.ListCurrent(ctx, "system.page_size"); err != nil {
		t.Fatalf("ListCurrent() error = %v", err)
	}
	if repo.tenantID != 10 || repo.key != "system.page_size" {
		t.Fatalf("list resolved tenant=%d key=%q", repo.tenantID, repo.key)
	}

	if _, err := uc.SetCurrent(ctx, "system.page_size", "50"); err != nil {
		t.Fatalf("SetCurrent() error = %v", err)
	}
	if repo.tenantID != 10 || repo.operatorID != 7 || repo.value != "50" {
		t.Fatalf("set override tenant=%d operator=%d value=%q", repo.tenantID, repo.operatorID, repo.value)
	}

	if err := uc.ResetCurrent(ctx, "system.page_size"); err != nil {
		t.Fatalf("ResetCurrent() error = %v", err)
	}
	if repo.resetID != 10 {
		t.Fatalf("reset tenant = %d, want 10", repo.resetID)
	}
}

func TestParameterUsecaseCurrentTenantRequiresContext(t *testing.T) {
	t.Parallel()

	uc := NewParameterUsecase(&parameterRepoStub{}, log.NewStdLogger(io.Discard))
	if _, err := uc.ListCurrent(context.Background(), "system.page_size"); !errors.IsForbidden(err) {
		t.Fatalf("ListCurrent() error = %v, want forbidden", err)
	}
	if _, err := uc.SetCurrent(context.Background(), "system.page_size", "50"); !errors.IsForbidden(err) {
		t.Fatalf("SetCurrent() error = %v, want forbidden", err)
	}
	if err := uc.ResetCurrent(context.Background(), "system.page_size"); !errors.IsForbidden(err) {
		t.Fatalf("ResetCurrent() error = %v, want forbidden", err)
	}
}

func TestParameterUsecaseExplicitTenantRequiresTenantID(t *testing.T) {
	t.Parallel()

	uc := NewParameterUsecase(&parameterRepoStub{}, log.NewStdLogger(io.Discard))
	if _, err := uc.ListTenant(context.Background(), 0, "system.page_size"); !errors.IsBadRequest(err) {
		t.Fatalf("ListTenant() error = %v, want bad request", err)
	}
	if _, err := uc.SetTenant(context.Background(), 0, "system.page_size", "50"); !errors.IsBadRequest(err) {
		t.Fatalf("SetTenant() error = %v, want bad request", err)
	}
	if err := uc.ResetTenant(context.Background(), 0, "system.page_size"); !errors.IsBadRequest(err) {
		t.Fatalf("ResetTenant() error = %v, want bad request", err)
	}
}

func TestParameterUsecaseExplicitTenantUsesOperatorFromContext(t *testing.T) {
	t.Parallel()

	repo := &parameterRepoStub{}
	uc := NewParameterUsecase(repo, log.NewStdLogger(io.Discard))
	ctx := authn.ContextWithAuthUser(context.Background(), parameterTestUser{subject: "7", tenant: "10"})

	if _, err := uc.SetTenant(ctx, 99, "system.page_size", "50"); err != nil {
		t.Fatalf("SetTenant() error = %v", err)
	}
	if repo.tenantID != 99 || repo.operatorID != 7 {
		t.Fatalf("set tenant override tenant=%d operator=%d", repo.tenantID, repo.operatorID)
	}
}
