package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"backend-service/app/avmc/admin/internal/data"
	"backend-service/app/avmc/admin/internal/data/ent/gen"
	"backend-service/app/avmc/admin/internal/data/ent/gen/dept"
	"backend-service/app/avmc/admin/internal/data/ent/gen/menu"
	"backend-service/app/avmc/admin/internal/data/ent/gen/post"
	"backend-service/app/avmc/admin/internal/data/ent/gen/project"
	"backend-service/app/avmc/admin/internal/data/ent/gen/role"
	"backend-service/app/avmc/admin/internal/data/ent/gen/user"
	"backend-service/app/avmc/admin/internal/runtimeconfig"
	"backend-service/pkg/utils/crypto"

	"github.com/go-kratos/kratos/v2/log"
)

const mockPassword = "MockAdmin@123456"

var flagconf string

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf ./configs")
}

func main() {
	flag.Parse()
	if err := run(context.Background(), log.NewStdLogger(os.Stdout)); err != nil {
		fmt.Fprintf(os.Stderr, "admin mock failed: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger log.Logger) error {
	bc, err := runtimeconfig.Load(flagconf)
	if err != nil {
		return err
	}
	if err := data.RunSchemaMigration(ctx, bc.Data, logger); err != nil {
		return err
	}
	client, err := data.NewEntClient(bc.Data, logger)
	if err != nil {
		return err
	}
	defer client.Close()

	if err := seed(ctx, client); err != nil {
		return err
	}
	if err := verify(ctx, client); err != nil {
		return err
	}
	fmt.Printf("admin mock data ready: tenants=[1,2] users=[mock_admin,mock_operator,mock_tenant2] password=%s\n", mockPassword)
	return nil
}

func seed(ctx context.Context, client *gen.Client) error {
	hash, err := crypto.HashPassword(mockPassword)
	if err != nil {
		return err
	}
	admin, err := ensureUser(ctx, client, 1, "mock_admin", hash, "mock_admin@example.com")
	if err != nil {
		return err
	}
	operator, err := ensureUser(ctx, client, 1, "mock_operator", hash, "mock_operator@example.com")
	if err != nil {
		return err
	}
	tenantTwo, err := ensureUser(ctx, client, 2, "mock_tenant2", hash, "mock_tenant2@example.com")
	if err != nil {
		return err
	}

	rootMenu, err := ensureMenu(ctx, client, "mock_system", "/mock", 0)
	if err != nil {
		return err
	}
	userMenu, err := ensureMenu(ctx, client, "mock_users", "/mock/users", rootMenu.ID)
	if err != nil {
		return err
	}
	projectMenu, err := ensureMenu(ctx, client, "mock_projects", "/mock/projects", rootMenu.ID)
	if err != nil {
		return err
	}
	if err := ensureRole(ctx, client, 1, "mock_super_admin", []uint32{rootMenu.ID, userMenu.ID, projectMenu.ID}); err != nil {
		return err
	}
	if err := ensureRole(ctx, client, 1, "mock_operator_role", []uint32{rootMenu.ID, projectMenu.ID}); err != nil {
		return err
	}
	if err := ensureRole(ctx, client, 2, "mock_tenant2_role", []uint32{rootMenu.ID}); err != nil {
		return err
	}
	if err := ensurePost(ctx, client, 1, "mock_developer"); err != nil {
		return err
	}
	if err := ensurePost(ctx, client, 1, "mock_manager"); err != nil {
		return err
	}
	if err := ensurePost(ctx, client, 2, "mock_developer"); err != nil {
		return err
	}
	rootDept, err := ensureDept(ctx, client, 1, "mock_headquarters", 0, admin.ID)
	if err != nil {
		return err
	}
	if _, err := ensureDept(ctx, client, 1, "mock_engineering", rootDept.ID, operator.ID); err != nil {
		return err
	}
	if _, err := ensureDept(ctx, client, 2, "mock_tenant2_dept", 0, tenantTwo.ID); err != nil {
		return err
	}
	if err := ensureProject(ctx, client, 1, "mock_admin_project", "MOCK-ADMIN", admin.ID, []uint32{admin.ID, operator.ID}); err != nil {
		return err
	}
	return ensureProject(ctx, client, 2, "mock_tenant2_project", "MOCK-TENANT2", tenantTwo.ID, []uint32{tenantTwo.ID})
}

func ensureUser(ctx context.Context, client *gen.Client, tenantID uint32, name, password, email string) (*gen.User, error) {
	item, err := client.User.Query().Where(user.TenantIDEQ(tenantID), user.Name(name)).Only(ctx)
	if err == nil {
		return item, nil
	}
	if !gen.IsNotFound(err) {
		return nil, err
	}
	return client.User.Create().SetTenantID(tenantID).SetName(name).SetPassword(password).SetEmail(email).SetStatus(1).Save(ctx)
}

func ensureMenu(ctx context.Context, client *gen.Client, name, path string, parentID uint32) (*gen.Menu, error) {
	item, err := client.Menu.Query().Where(menu.Name(name)).Only(ctx)
	if err == nil {
		return item, nil
	}
	if !gen.IsNotFound(err) {
		return nil, err
	}
	builder := client.Menu.Create().SetName(name).SetTitle(name).SetPath(path).SetStatus(1).SetType(2)
	if parentID > 0 {
		builder.SetParentID(parentID)
	}
	return builder.Save(ctx)
}

func ensureRole(ctx context.Context, client *gen.Client, tenantID uint32, name string, menuIDs []uint32) error {
	item, err := client.Role.Query().Where(role.TenantIDEQ(tenantID), role.Name(name)).Only(ctx)
	if gen.IsNotFound(err) {
		_, err = client.Role.Create().SetTenantID(tenantID).SetName(name).SetStatus(1).AddMenuIDs(menuIDs...).Save(ctx)
		return err
	}
	if err != nil {
		return err
	}
	_, err = item.Update().ClearMenus().AddMenuIDs(menuIDs...).Save(ctx)
	return err
}

func ensurePost(ctx context.Context, client *gen.Client, tenantID uint32, name string) error {
	exists, err := client.Post.Query().Where(post.TenantIDEQ(tenantID), post.Name(name)).Exist(ctx)
	if err != nil || exists {
		return err
	}
	_, err = client.Post.Create().SetTenantID(tenantID).SetName(name).SetStatus(1).SetSort(10).SetRemark("mock data").Save(ctx)
	return err
}

func ensureDept(ctx context.Context, client *gen.Client, tenantID uint32, name string, parentID, leaderID uint32) (*gen.Dept, error) {
	item, err := client.Dept.Query().Where(dept.TenantIDEQ(tenantID), dept.Name(name)).Only(ctx)
	if err == nil {
		return item, nil
	}
	if !gen.IsNotFound(err) {
		return nil, err
	}
	builder := client.Dept.Create().SetTenantID(tenantID).SetName(name).SetStatus(1).SetLeaderID(leaderID)
	if parentID > 0 {
		builder.SetParentID(parentID).SetAncestors([]int{int(parentID)})
	}
	return builder.Save(ctx)
}

func ensureProject(ctx context.Context, client *gen.Client, tenantID uint32, name, code string, ownerID uint32, memberIDs []uint32) error {
	item, err := client.Project.Query().Where(project.TenantIDEQ(tenantID), project.Name(name)).Only(ctx)
	if gen.IsNotFound(err) {
		_, err = client.Project.Create().SetTenantID(tenantID).SetName(name).SetCode(code).SetOwnerID(ownerID).SetStatus(1).SetDescription("mock data").AddMemberIDs(memberIDs...).Save(ctx)
		return err
	}
	if err != nil {
		return err
	}
	_, err = item.Update().SetOwnerID(ownerID).ClearMembers().AddMemberIDs(memberIDs...).Save(ctx)
	return err
}

func verify(ctx context.Context, client *gen.Client) error {
	checks := []struct {
		name  string
		count func(context.Context) (int, error)
		min   int
	}{
		{"tenant 1 users", func(ctx context.Context) (int, error) {
			return client.User.Query().Where(user.TenantIDEQ(1)).Count(ctx)
		}, 2},
		{"tenant 2 users", func(ctx context.Context) (int, error) {
			return client.User.Query().Where(user.TenantIDEQ(2)).Count(ctx)
		}, 1},
		{"menus", func(ctx context.Context) (int, error) {
			return client.Menu.Query().Where(menu.NameHasPrefix("mock_")).Count(ctx)
		}, 3},
		{"tenant 1 roles", func(ctx context.Context) (int, error) {
			return client.Role.Query().Where(role.TenantIDEQ(1)).Count(ctx)
		}, 2},
		{"tenant 2 roles", func(ctx context.Context) (int, error) {
			return client.Role.Query().Where(role.TenantIDEQ(2)).Count(ctx)
		}, 1},
		{"tenant 1 posts", func(ctx context.Context) (int, error) {
			return client.Post.Query().Where(post.TenantIDEQ(1)).Count(ctx)
		}, 2},
		{"tenant 2 posts", func(ctx context.Context) (int, error) {
			return client.Post.Query().Where(post.TenantIDEQ(2)).Count(ctx)
		}, 1},
		{"tenant 1 departments", func(ctx context.Context) (int, error) {
			return client.Dept.Query().Where(dept.TenantIDEQ(1)).Count(ctx)
		}, 2},
		{"tenant 2 departments", func(ctx context.Context) (int, error) {
			return client.Dept.Query().Where(dept.TenantIDEQ(2)).Count(ctx)
		}, 1},
		{"tenant 1 projects", func(ctx context.Context) (int, error) {
			return client.Project.Query().Where(project.TenantIDEQ(1)).Count(ctx)
		}, 1},
		{"tenant 2 projects", func(ctx context.Context) (int, error) {
			return client.Project.Query().Where(project.TenantIDEQ(2)).Count(ctx)
		}, 1},
	}
	for _, check := range checks {
		count, err := check.count(ctx)
		if err != nil {
			return fmt.Errorf("checking %s: %w", check.name, err)
		}
		if count < check.min {
			return fmt.Errorf("%s count=%d, want >=%d", check.name, count, check.min)
		}
		fmt.Printf("verified %-22s count=%d\n", check.name, count)
	}
	return nil
}
