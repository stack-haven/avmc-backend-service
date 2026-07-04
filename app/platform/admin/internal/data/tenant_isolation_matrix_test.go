package data

import (
	"context"
	"fmt"
	"testing"

	"backend-service/app/platform/admin/internal/data/ent/gen"
)

type tenantEntityAdapter struct {
	name   string
	create func(context.Context, string) uint32
	count  func(context.Context) int
	update func(context.Context, uint32) error
	delete func(context.Context, uint32) error
	exists func(context.Context, uint32) bool
}

func TestTenantPrivacyPolicyIsolationMatrix(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	adapters := []tenantEntityAdapter{
		{
			name: "user",
			create: func(ctx context.Context, suffix string) uint32 {
				return client.User.Create().SetName("usr_" + suffix).SetPassword("password").SaveX(ctx).ID
			},
			count: func(ctx context.Context) int { return client.User.Query().CountX(ctx) },
			update: func(ctx context.Context, id uint32) error {
				_, err := client.User.UpdateOneID(id).SetNickname("updated").Save(ctx)
				return err
			},
			delete: func(ctx context.Context, id uint32) error { return client.User.DeleteOneID(id).Exec(ctx) },
			exists: func(ctx context.Context, id uint32) bool {
				return client.User.Query().Where().ExistX(ctx) && entityExists(client.User.Get(ctx, id))
			},
		},
		{
			name: "role",
			create: func(ctx context.Context, suffix string) uint32 {
				return client.Role.Create().SetName("role_" + suffix).SaveX(ctx).ID
			},
			count: func(ctx context.Context) int { return client.Role.Query().CountX(ctx) },
			update: func(ctx context.Context, id uint32) error {
				_, err := client.Role.UpdateOneID(id).SetDefaultRouter("/updated").Save(ctx)
				return err
			},
			delete: func(ctx context.Context, id uint32) error { return client.Role.DeleteOneID(id).Exec(ctx) },
			exists: func(ctx context.Context, id uint32) bool { return entityExists(client.Role.Get(ctx, id)) },
		},
		{
			name: "dept",
			create: func(ctx context.Context, suffix string) uint32 {
				return client.Dept.Create().SetName("dept_" + suffix).SaveX(ctx).ID
			},
			count: func(ctx context.Context) int { return client.Dept.Query().CountX(ctx) },
			update: func(ctx context.Context, id uint32) error {
				_, err := client.Dept.UpdateOneID(id).SetRemark("updated").Save(ctx)
				return err
			},
			delete: func(ctx context.Context, id uint32) error { return client.Dept.DeleteOneID(id).Exec(ctx) },
			exists: func(ctx context.Context, id uint32) bool { return entityExists(client.Dept.Get(ctx, id)) },
		},
		{
			name: "post",
			create: func(ctx context.Context, suffix string) uint32 {
				return client.Post.Create().SetName("post_" + suffix).SaveX(ctx).ID
			},
			count: func(ctx context.Context) int { return client.Post.Query().CountX(ctx) },
			update: func(ctx context.Context, id uint32) error {
				_, err := client.Post.UpdateOneID(id).SetRemark("updated").Save(ctx)
				return err
			},
			delete: func(ctx context.Context, id uint32) error { return client.Post.DeleteOneID(id).Exec(ctx) },
			exists: func(ctx context.Context, id uint32) bool { return entityExists(client.Post.Get(ctx, id)) },
		},
		{
			name: "project",
			create: func(ctx context.Context, suffix string) uint32 {
				return client.Project.Create().SetName("project_" + suffix).SetCode("code_" + suffix).SaveX(ctx).ID
			},
			count: func(ctx context.Context) int { return client.Project.Query().CountX(ctx) },
			update: func(ctx context.Context, id uint32) error {
				_, err := client.Project.UpdateOneID(id).SetDescription("updated").Save(ctx)
				return err
			},
			delete: func(ctx context.Context, id uint32) error { return client.Project.DeleteOneID(id).Exec(ctx) },
			exists: func(ctx context.Context, id uint32) bool { return entityExists(client.Project.Get(ctx, id)) },
		},
		{
			name: "dictionary_type",
			create: func(ctx context.Context, suffix string) uint32 {
				return client.DictionaryType.Create().SetName("dict_" + suffix).SetCode("dict_" + suffix).SaveX(ctx).ID
			},
			count: func(ctx context.Context) int { return client.DictionaryType.Query().CountX(ctx) },
			update: func(ctx context.Context, id uint32) error {
				_, err := client.DictionaryType.UpdateOneID(id).SetRemark("updated").Save(ctx)
				return err
			},
			delete: func(ctx context.Context, id uint32) error { return client.DictionaryType.DeleteOneID(id).Exec(ctx) },
			exists: func(ctx context.Context, id uint32) bool { return entityExists(client.DictionaryType.Get(ctx, id)) },
		},
		{
			name: "operation_log",
			create: func(ctx context.Context, suffix string) uint32 {
				return uint32(client.OperationLog.Create().SetModule("module_" + suffix).SetAction("create").SaveX(ctx).ID)
			},
			count: func(ctx context.Context) int { return client.OperationLog.Query().CountX(ctx) },
			update: func(ctx context.Context, id uint32) error {
				_, err := client.OperationLog.UpdateOneID(id).SetAction("updated").Save(ctx)
				return err
			},
			delete: func(ctx context.Context, id uint32) error {
				return client.OperationLog.DeleteOneID(id).Exec(ctx)
			},
			exists: func(ctx context.Context, id uint32) bool {
				return entityExists(client.OperationLog.Get(ctx, id))
			},
		},
		{
			name: "login_log",
			create: func(ctx context.Context, suffix string) uint32 {
				return uint32(client.LoginLog.Create().
					SetIdentity("identity_" + suffix).
					SetLoginType("password").
					SetResult("success").
					SaveX(ctx).ID)
			},
			count: func(ctx context.Context) int { return client.LoginLog.Query().CountX(ctx) },
			update: func(ctx context.Context, id uint32) error {
				_, err := client.LoginLog.UpdateOneID(id).SetResult("updated").Save(ctx)
				return err
			},
			delete: func(ctx context.Context, id uint32) error {
				return client.LoginLog.DeleteOneID(id).Exec(ctx)
			},
			exists: func(ctx context.Context, id uint32) bool {
				return entityExists(client.LoginLog.Get(ctx, id))
			},
		},
	}

	for _, adapter := range adapters {
		t.Run(adapter.name, func(t *testing.T) {
			ctx1 := tenantContext(1)
			ctx2 := tenantContext(2)
			systemCtx := systemContext()
			id1 := adapter.create(ctx1, adapter.name+"_one")
			adapter.create(ctx2, adapter.name+"_two")

			if got := adapter.count(ctx1); got != 1 {
				t.Fatalf("tenant 1 count = %d, want 1", got)
			}
			if got := adapter.count(ctx2); got != 1 {
				t.Fatalf("tenant 2 count = %d, want 1", got)
			}
			if got := adapter.count(systemCtx); got != 2 {
				t.Fatalf("system count = %d, want 2", got)
			}
			if err := adapter.update(ctx2, id1); !gen.IsNotFound(err) {
				t.Fatalf("cross-tenant update error = %v, want not found", err)
			}
			if err := adapter.delete(ctx2, id1); !gen.IsNotFound(err) {
				t.Fatalf("cross-tenant delete error = %v, want not found", err)
			}
			if !adapter.exists(systemCtx, id1) {
				t.Fatal("cross-tenant mutation changed target entity")
			}
		})
	}
}

func TestDictionaryItemCannotCrossTenantTypeBoundary(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	typeOne := client.DictionaryType.Create().SetName("type_one").SetCode("type_one").SaveX(tenantContext(1))
	typeTwo := client.DictionaryType.Create().SetName("type_two").SetCode("type_two").SaveX(tenantContext(2))
	itemOne := client.DictionaryItem.Create().
		SetTypeID(typeOne.ID).
		SetLabel("one").
		SetValue("one").
		SaveX(tenantContext(1))

	if _, err := client.DictionaryItem.Create().
		SetTypeID(typeOne.ID).
		SetLabel("blocked").
		SetValue("blocked").
		Save(tenantContext(2)); err == nil {
		t.Fatal("tenant 2 created an item under tenant 1 dictionary type")
	}
	if _, err := client.DictionaryItem.UpdateOneID(itemOne.ID).SetTypeID(typeTwo.ID).Save(tenantContext(1)); err == nil {
		t.Fatal("tenant 1 moved an item to tenant 2 dictionary type")
	}
}

func entityExists[T any](entity *T, err error) bool {
	if err != nil {
		return false
	}
	return entity != nil
}

func TestTenantCreateRejectsExplicitMismatchedTenantIDAcrossSchemas(t *testing.T) {
	client := newTestClient(t)
	defer client.Close()

	tests := []struct {
		name string
		run  func() error
	}{
		{"role", func() error {
			_, err := client.Role.Create().SetTenantID(2).SetName("mismatch_role").Save(tenantContext(1))
			return err
		}},
		{"project", func() error {
			_, err := client.Project.Create().SetTenantID(2).SetName("mismatch_project").SetCode("mismatch_project").Save(tenantContext(1))
			return err
		}},
		{"dictionary", func() error {
			_, err := client.DictionaryType.Create().SetTenantID(2).SetName("mismatch_dict").SetCode("mismatch_dict").Save(tenantContext(1))
			return err
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.run(); err == nil {
				t.Fatalf("%s accepted mismatched tenant_id", fmt.Sprint(test.name))
			}
		})
	}
}
