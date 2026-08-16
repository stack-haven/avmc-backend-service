//go:build mock

package main

import (
	"context"
	"fmt"

	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/app/platform/service/internal/data/ent/gen/dept"
	"backend-service/app/platform/service/internal/data/ent/gen/project"
	"backend-service/app/platform/service/internal/data/ent/gen/role"
	"backend-service/app/platform/service/internal/data/ent/gen/user"
)

// verify 校验 seed 结果，确保关键数据就绪。
func verify(ctx context.Context, client *gen.Client) error {
	type check struct {
		name string
		fn   func(context.Context) (int, error)
		min  int
	}
	checks := []check{
		{"tenants", func(ctx context.Context) (int, error) { return client.Tenant.Query().Count(ctx) }, 2},
		{"menus", func(ctx context.Context) (int, error) { return client.Menu.Query().Count(ctx) }, 105},
		{"tenant 1 users", func(ctx context.Context) (int, error) {
			return client.User.Query().Where(user.TenantIDEQ(1)).Count(ctx)
		}, 4},
		{"tenant 2 users", func(ctx context.Context) (int, error) {
			return client.User.Query().Where(user.TenantIDEQ(2)).Count(ctx)
		}, 1},
		{"packages", func(ctx context.Context) (int, error) { return client.TenantMenuPermissionGroup.Query().Count(ctx) }, 2},
		{"tenant 1 roles", func(ctx context.Context) (int, error) {
			return client.Role.Query().Where(role.TenantIDEQ(1)).Count(ctx)
		}, 2},
		{"tenant 2 roles", func(ctx context.Context) (int, error) {
			return client.Role.Query().Where(role.TenantIDEQ(2)).Count(ctx)
		}, 1},
		{"tenant 1 depts", func(ctx context.Context) (int, error) {
			return client.Dept.Query().Where(dept.TenantIDEQ(1)).Count(ctx)
		}, 2},
		{"tenant 2 depts", func(ctx context.Context) (int, error) {
			return client.Dept.Query().Where(dept.TenantIDEQ(2)).Count(ctx)
		}, 1},
		{"tenant 1 projects", func(ctx context.Context) (int, error) {
			return client.Project.Query().Where(project.TenantIDEQ(1)).Count(ctx)
		}, 1},
		{"tenant 2 projects", func(ctx context.Context) (int, error) {
			return client.Project.Query().Where(project.TenantIDEQ(2)).Count(ctx)
		}, 1},
		{"dictionaries", func(ctx context.Context) (int, error) { return client.DictionaryType.Query().Count(ctx) }, 2},
		{"dictionary items", func(ctx context.Context) (int, error) { return client.DictionaryItem.Query().Count(ctx) }, 5},
		{"parameters", func(ctx context.Context) (int, error) { return client.ParameterDefinition.Query().Count(ctx) }, 3},
		{"notification templates", func(ctx context.Context) (int, error) { return client.NotificationTemplate.Query().Count(ctx) }, 2},
		{"notification providers", func(ctx context.Context) (int, error) { return client.NotificationProvider.Query().Count(ctx) }, 2},
		{"storage providers", func(ctx context.Context) (int, error) { return client.StorageProvider.Query().Count(ctx) }, 1},
	}
	for _, c := range checks {
		count, err := c.fn(ctx)
		if err != nil {
			return fmt.Errorf("check %s: %w", c.name, err)
		}
		if count < c.min {
			return fmt.Errorf("%s count=%d < %d", c.name, count, c.min)
		}
		fmt.Printf("verified %-25s count=%d\n", c.name, count)
	}
	return nil
}
