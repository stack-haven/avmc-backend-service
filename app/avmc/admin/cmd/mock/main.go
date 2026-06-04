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
	ctx := context.Background()
	logger := log.NewStdLogger(os.Stdout)
	bc, err := runtimeconfig.Load(flagconf)
	if err != nil {
		panic(err)
	}
	if err := data.RunSchemaMigration(ctx, bc.Data, logger); err != nil {
		panic(err)
	}
	client := data.NewEntClient(bc.Data, logger)
	defer client.Close()

	if err := seed(ctx, client); err != nil {
		panic(err)
	}
	if err := verify(ctx, client); err != nil {
		panic(err)
	}
	fmt.Printf("admin mock data ready: domains=[1,2] users=[mock_admin,mock_operator,mock_tenant2] password=%s\n", mockPassword)
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

func ensureUser(ctx context.Context, client *gen.Client, domainID uint32, name, password, email string) (*gen.User, error) {
	item, err := client.User.Query().Where(user.DomainIDEQ(domainID), user.Name(name)).Only(ctx)
	if err == nil {
		return item, nil
	}
	if !gen.IsNotFound(err) {
		return nil, err
	}
	return client.User.Create().SetDomainID(domainID).SetName(name).SetPassword(password).SetEmail(email).SetStatus(1).Save(ctx)
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

func ensureRole(ctx context.Context, client *gen.Client, domainID uint32, name string, menuIDs []uint32) error {
	item, err := client.Role.Query().Where(role.DomainIDEQ(domainID), role.Name(name)).Only(ctx)
	if gen.IsNotFound(err) {
		_, err = client.Role.Create().SetDomainID(domainID).SetName(name).SetStatus(1).AddMenuIDs(menuIDs...).Save(ctx)
		return err
	}
	if err != nil {
		return err
	}
	_, err = item.Update().ClearMenus().AddMenuIDs(menuIDs...).Save(ctx)
	return err
}

func ensurePost(ctx context.Context, client *gen.Client, domainID uint32, name string) error {
	exists, err := client.Post.Query().Where(post.DomainIDEQ(domainID), post.Name(name)).Exist(ctx)
	if err != nil || exists {
		return err
	}
	_, err = client.Post.Create().SetDomainID(domainID).SetName(name).SetStatus(1).SetSort(10).SetRemark("mock data").Save(ctx)
	return err
}

func ensureDept(ctx context.Context, client *gen.Client, domainID uint32, name string, parentID, leaderID uint32) (*gen.Dept, error) {
	item, err := client.Dept.Query().Where(dept.DomainIDEQ(domainID), dept.Name(name)).Only(ctx)
	if err == nil {
		return item, nil
	}
	if !gen.IsNotFound(err) {
		return nil, err
	}
	builder := client.Dept.Create().SetDomainID(domainID).SetName(name).SetStatus(1).SetLeaderID(leaderID)
	if parentID > 0 {
		builder.SetParentID(parentID).SetAncestors([]int{int(parentID)})
	}
	return builder.Save(ctx)
}

func ensureProject(ctx context.Context, client *gen.Client, domainID uint32, name, code string, ownerID uint32, memberIDs []uint32) error {
	item, err := client.Project.Query().Where(project.DomainIDEQ(domainID), project.Name(name)).Only(ctx)
	if gen.IsNotFound(err) {
		_, err = client.Project.Create().SetDomainID(domainID).SetName(name).SetCode(code).SetOwnerID(ownerID).SetStatus(1).SetDescription("mock data").AddMemberIDs(memberIDs...).Save(ctx)
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
		count int
		min   int
	}{
		{"domain 1 users", client.User.Query().Where(user.DomainIDEQ(1)).CountX(ctx), 2},
		{"domain 2 users", client.User.Query().Where(user.DomainIDEQ(2)).CountX(ctx), 1},
		{"menus", client.Menu.Query().Where(menu.NameHasPrefix("mock_")).CountX(ctx), 3},
		{"domain 1 roles", client.Role.Query().Where(role.DomainIDEQ(1)).CountX(ctx), 2},
		{"domain 2 roles", client.Role.Query().Where(role.DomainIDEQ(2)).CountX(ctx), 1},
		{"domain 1 posts", client.Post.Query().Where(post.DomainIDEQ(1)).CountX(ctx), 2},
		{"domain 2 posts", client.Post.Query().Where(post.DomainIDEQ(2)).CountX(ctx), 1},
		{"domain 1 departments", client.Dept.Query().Where(dept.DomainIDEQ(1)).CountX(ctx), 2},
		{"domain 2 departments", client.Dept.Query().Where(dept.DomainIDEQ(2)).CountX(ctx), 1},
		{"domain 1 projects", client.Project.Query().Where(project.DomainIDEQ(1)).CountX(ctx), 1},
		{"domain 2 projects", client.Project.Query().Where(project.DomainIDEQ(2)).CountX(ctx), 1},
	}
	for _, check := range checks {
		if check.count < check.min {
			return fmt.Errorf("%s count=%d, want >=%d", check.name, check.count, check.min)
		}
		fmt.Printf("verified %-22s count=%d\n", check.name, check.count)
	}
	return nil
}
