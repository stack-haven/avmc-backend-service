package data

import (
	"io"
	"testing"

	"backend-service/api/common/enum"
	pb "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

func TestParameterRepoResolvesTenantOverrideAndReset(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	systemCtx := systemContext()
	client.Tenant.Create().SetID(1).SetName("tenant-one").SetCode("tenant-one").SaveX(systemCtx)
	client.Tenant.Create().SetID(2).SetName("tenant-two").SetCode("tenant-two").SaveX(systemCtx)

	repo := NewParameterRepo(&Data{db: client}, log.NewStdLogger(io.Discard)).(*parameterRepo)
	definition, err := repo.CreateDefinition(systemCtx, &pb.ParameterDefinition{
		Key: "system.page_size", Name: "Page size",
		ValueType:    pb.ParameterValueType_PARAMETER_VALUE_TYPE_INTEGER,
		DefaultValue: "20", TenantOverridable: true,
		Status: enum.Status_STATUS_ENABLED.Enum(),
	})
	if err != nil {
		t.Fatalf("create definition: %v", err)
	}

	items, err := repo.ListResolved(systemCtx, 1, "")
	if err != nil || len(items) != 1 || items[0].GetValue() != "20" {
		t.Fatalf("tenant 1 defaults = %+v, %v", items, err)
	}
	if _, err = repo.SetOverride(systemCtx, 1, definition.GetKey(), "50", 7); err != nil {
		t.Fatalf("set override: %v", err)
	}
	items, err = repo.ListResolved(systemCtx, 1, "")
	if err != nil || items[0].GetValue() != "50" ||
		items[0].GetSource() != pb.ParameterValueSource_PARAMETER_VALUE_SOURCE_TENANT_OVERRIDE {
		t.Fatalf("tenant 1 override = %+v, %v", items, err)
	}
	tenantTwo, err := repo.ListResolved(systemCtx, 2, "")
	if err != nil || tenantTwo[0].GetValue() != "20" {
		t.Fatalf("tenant 2 leaked override = %+v, %v", tenantTwo, err)
	}
	if err := repo.ResetOverride(systemCtx, 1, definition.GetKey()); err != nil {
		t.Fatalf("reset override: %v", err)
	}
	items, err = repo.ListResolved(systemCtx, 1, "")
	if err != nil || items[0].GetValue() != "20" ||
		items[0].GetSource() != pb.ParameterValueSource_PARAMETER_VALUE_SOURCE_PLATFORM_DEFAULT {
		t.Fatalf("tenant 1 reset = %+v, %v", items, err)
	}
}

func TestParameterRepoRejectsInvalidOrForbiddenOverride(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	ctx := systemContext()
	client.Tenant.Create().SetID(1).SetName("tenant-one").SetCode("tenant-one").SaveX(ctx)
	repo := NewParameterRepo(&Data{db: client}, log.NewStdLogger(io.Discard)).(*parameterRepo)

	_, err := repo.CreateDefinition(ctx, &pb.ParameterDefinition{
		Key: "system.fixed_mode", Name: "Fixed mode",
		ValueType:    pb.ParameterValueType_PARAMETER_VALUE_TYPE_BOOLEAN,
		DefaultValue: "false", TenantOverridable: false,
		Status: enum.Status_STATUS_ENABLED.Enum(),
	})
	if err != nil {
		t.Fatalf("create fixed definition: %v", err)
	}
	if _, err := repo.SetOverride(ctx, 1, "system.fixed_mode", "true", 1); !errors.IsForbidden(err) {
		t.Fatalf("non-overridable error = %v, want forbidden", err)
	}

	_, err = repo.CreateDefinition(ctx, &pb.ParameterDefinition{
		Key: "system.retry_count", Name: "Retry count",
		ValueType:    pb.ParameterValueType_PARAMETER_VALUE_TYPE_INTEGER,
		DefaultValue: "3", TenantOverridable: true,
		Status: enum.Status_STATUS_ENABLED.Enum(),
	})
	if err != nil {
		t.Fatalf("create integer definition: %v", err)
	}
	if _, err := repo.SetOverride(ctx, 1, "system.retry_count", "many", 1); err == nil {
		t.Fatal("invalid integer override was accepted")
	}
}

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

func TestParameterOverridePrivacyIsolation(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	definition := client.ParameterDefinition.Create().
		SetKey("system.locale").
		SetName("Locale").
		SetValueType(int32(pb.ParameterValueType_PARAMETER_VALUE_TYPE_STRING)).
		SetDefaultValue("zh-CN").
		SaveX(systemContext())
	client.TenantParameterOverride.Create().
		SetDefinitionID(definition.ID).
		SetValue("en-US").
		SaveX(tenantContext(1))

	if got := client.TenantParameterOverride.Query().CountX(tenantContext(1)); got != 1 {
		t.Fatalf("tenant 1 override count = %d, want 1", got)
	}
	if got := client.TenantParameterOverride.Query().CountX(tenantContext(2)); got != 0 {
		t.Fatalf("tenant 2 override count = %d, want 0", got)
	}
}
