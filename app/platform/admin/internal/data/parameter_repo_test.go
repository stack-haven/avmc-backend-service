package data

import (
	"io"
	"testing"

	"backend-service/api/common/enum"
	pb "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

func TestParameterRepoDeleteAllowsKeyReuse(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	ctx := systemContext()
	repo := NewParameterRepo(&Data{db: client}, log.NewStdLogger(io.Discard)).(*parameterRepo)
	item := &pb.ParameterDefinition{
		Key: "system.reusable_key", Name: "Reusable key",
		ValueType:    pb.ParameterValueType_PARAMETER_VALUE_TYPE_STRING,
		DefaultValue: "default", TenantOverridable: true,
		Status: enum.Status_STATUS_ENABLED.Enum(),
	}

	created, err := repo.CreateDefinition(ctx, item)
	if err != nil {
		t.Fatalf("create definition: %v", err)
	}
	if err = repo.DeleteDefinition(ctx, created.GetId()); err != nil {
		t.Fatalf("delete definition: %v", err)
	}
	if _, err = repo.CreateDefinition(ctx, item); err != nil {
		t.Fatalf("recreate definition with same key: %v", err)
	}
}
