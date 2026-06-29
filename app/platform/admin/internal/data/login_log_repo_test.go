package data

import (
	"io"
	"testing"

	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

func TestLoginLogRepoTenantIsolationAndFiltering(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()
	repo := NewLoginLogRepo(&Data{db: client}, log.NewStdLogger(io.Discard))

	success := "success"
	failure := "failure"
	if err := repo.Append(tenantContext(1), &pbCore.LoginLog{
		TenantId: 1, Identity: "admin", LoginType: "username", Result: success,
	}); err != nil {
		t.Fatalf("append tenant 1 success: %v", err)
	}
	if err := repo.Append(tenantContext(1), &pbCore.LoginLog{
		TenantId: 1, Identity: "unknown", LoginType: "username", Result: failure,
	}); err != nil {
		t.Fatalf("append tenant 1 failure: %v", err)
	}
	if err := repo.Append(tenantContext(2), &pbCore.LoginLog{
		TenantId: 2, Identity: "admin", LoginType: "username", Result: success,
	}); err != nil {
		t.Fatalf("append tenant 2 success: %v", err)
	}

	items, total, err := repo.List(tenantContext(1), &pbCore.ListLoginLogsRequest{Result: &success})
	if err != nil {
		t.Fatalf("list tenant 1: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].GetTenantId() != 1 {
		t.Fatalf("tenant 1 filtered logs = total:%d items:%+v", total, items)
	}
	items, total, err = repo.List(tenantContext(2), &pbCore.ListLoginLogsRequest{})
	if err != nil {
		t.Fatalf("list tenant 2: %v", err)
	}
	if total != 1 || len(items) != 1 || items[0].GetTenantId() != 2 {
		t.Fatalf("tenant 2 logs = total:%d items:%+v", total, items)
	}
}
