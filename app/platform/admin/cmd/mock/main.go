package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/authzpolicy"
	"backend-service/app/platform/admin/internal/conf"
	"backend-service/app/platform/admin/internal/data"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/dept"
	"backend-service/app/platform/admin/internal/data/ent/gen/dictionaryitem"
	"backend-service/app/platform/admin/internal/data/ent/gen/dictionarytype"
	"backend-service/app/platform/admin/internal/data/ent/gen/loginlog"
	"backend-service/app/platform/admin/internal/data/ent/gen/menu"
	"backend-service/app/platform/admin/internal/data/ent/gen/menupermissiongroup"
	"backend-service/app/platform/admin/internal/data/ent/gen/operationlog"
	"backend-service/app/platform/admin/internal/data/ent/gen/post"
	"backend-service/app/platform/admin/internal/data/ent/gen/project"
	"backend-service/app/platform/admin/internal/data/ent/gen/role"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenant"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenantpermissiongroup"
	"backend-service/app/platform/admin/internal/data/ent/gen/user"
	entviewer "backend-service/app/platform/admin/internal/data/ent/viewer"
	"backend-service/app/platform/admin/internal/runtimeconfig"
	"backend-service/pkg/auth/authz"
	"backend-service/pkg/utils/crypto"

	"github.com/go-kratos/kratos/v2/log"
	_ "github.com/go-sql-driver/mysql"
)

const mockPassword = "123456"

var (
	flagconf            string
	flagMigrate         bool
	flagResetLegacyMock bool
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf ./configs")
	flag.BoolVar(&flagMigrate, "migrate", false, "run schema migration before seeding mock data")
	flag.BoolVar(&flagResetLegacyMock, "reset-legacy-mock", false, "remove legacy tenant_id=0 mock data before migration")
}

func main() {
	flag.Parse()
	if err := run(context.Background(), log.NewStdLogger(os.Stdout)); err != nil {
		fmt.Fprintf(os.Stderr, "admin mock failed: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger log.Logger) error {
	ctx = entviewer.NewSystemContext(ctx)
	bc, err := runtimeconfig.Load(flagconf)
	if err != nil {
		return err
	}
	if flagResetLegacyMock {
		if bc.Data == nil || bc.Data.Database == nil {
			return fmt.Errorf("database config is required for legacy mock cleanup")
		}
		if err := cleanupLegacyMock(ctx, bc.Data.Database.Source); err != nil {
			return err
		}
	}
	if flagMigrate {
		if err := data.RunSchemaMigration(ctx, bc.Data, logger); err != nil {
			return fmt.Errorf("migrating schema: %w", err)
		}
	}
	client, err := data.NewEntClient(bc.Data, logger)
	if err != nil {
		return err
	}
	defer client.Close()

	result, err := seed(ctx, client)
	if err != nil {
		return err
	}
	if err := syncPolicies(ctx, bc.Data, logger, result); err != nil {
		return err
	}
	if err := refreshTenantMenuCacheVersions(ctx, bc.Data, logger, []uint32{1, 2}); err != nil {
		return err
	}
	if err := verify(ctx, client); err != nil {
		return err
	}
	fmt.Printf("admin mock data ready: tenants=[1,2] users=[admin,vben,jack,mock_tenant2] password=%s\n", mockPassword)
	return nil
}

type mockSeedResult struct {
	TenantOneUsers []uint32
	TenantTwoUsers []uint32
}

type mockMenuSpec struct {
	Name      string
	Title     string
	Path      string
	Component string
	Icon      string
	Type      int32
	Sort      int32
	AuthCode  string
}

func cleanupLegacyMock(ctx context.Context, dsn string) error {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer db.Close()

	cleanups := []struct {
		name string
		sql  string
	}{
		{"legacy mock role menus", "DELETE FROM role_menus WHERE role_id IN (SELECT id FROM roles WHERE name LIKE 'mock\\_%') OR menu_id IN (SELECT id FROM menus WHERE name LIKE 'mock\\_%')"},
		{"legacy mock user roles", "DELETE FROM user_roles WHERE user_id IN (SELECT id FROM users WHERE name LIKE 'mock\\_%') OR role_id IN (SELECT id FROM roles WHERE name LIKE 'mock\\_%')"},
		{"legacy mock projects", "DELETE FROM projects WHERE name LIKE 'mock\\_%'"},
		{"legacy mock departments", "DELETE FROM depts WHERE name LIKE 'mock\\_%'"},
		{"legacy mock posts", "DELETE FROM posts WHERE name LIKE 'mock\\_%'"},
		{"legacy mock roles", "DELETE FROM roles WHERE name LIKE 'mock\\_%'"},
		{"legacy mock users", "DELETE FROM users WHERE name LIKE 'mock\\_%'"},
		{"legacy mock menus", "DELETE FROM menus WHERE name LIKE 'mock\\_%'"},
	}
	for _, cleanup := range cleanups {
		result, err := db.ExecContext(ctx, cleanup.sql)
		if err != nil {
			return fmt.Errorf("cleaning %s: %w", cleanup.name, err)
		}
		count, _ := result.RowsAffected()
		fmt.Printf("cleaned %-24s count=%d\n", cleanup.name, count)
	}
	return nil
}

func seed(ctx context.Context, client *gen.Client) (*mockSeedResult, error) {
	hash, err := crypto.HashPassword(mockPassword)
	if err != nil {
		return nil, err
	}
	if _, err := ensureTenant(ctx, client, 1, "演示平台租户", "demo-platform", "拥有完整系统管理能力的演示租户"); err != nil {
		return nil, err
	}
	if _, err := ensureTenant(ctx, client, 2, "演示业务租户", "demo-business", "只开放基础能力的演示租户"); err != nil {
		return nil, err
	}
	extraTenants := []struct {
		id     uint32
		name   string
		code   string
		remark string
	}{
		{3, "华东零售演示租户", "demo-retail-east", "零售业务演示数据"},
		{4, "华南供应链演示租户", "demo-supply-south", "供应链业务演示数据"},
		{5, "北方制造演示租户", "demo-manufacturing", "制造业务演示数据"},
		{6, "教育培训演示租户", "demo-education", "教育行业演示数据"},
		{7, "企业服务演示租户", "demo-enterprise", "企业服务演示数据"},
		{8, "停用状态演示租户", "demo-disabled", "用于观察停用状态"},
	}
	for _, spec := range extraTenants {
		item, err := ensureTenant(ctx, client, spec.id, spec.name, spec.code, spec.remark)
		if err != nil {
			return nil, err
		}
		if spec.id == 8 {
			if _, err := item.Update().
				SetStatus(2).
				SetLifecycleStatus(int32(pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_SUSPENDED)).
				SetSuspendedAt(time.Now()).
				Save(ctx); err != nil {
				return nil, err
			}
		}
	}
	admin, err := ensureUser(ctx, client, 1, "mock_admin", hash, "mock_admin@example.com")
	if err != nil {
		return nil, err
	}
	vben, err := ensureUser(ctx, client, 1, "vben", hash, "vben@example.com")
	if err != nil {
		return nil, err
	}
	webAdmin, err := ensureUser(ctx, client, 1, "admin", hash, "admin@example.com")
	if err != nil {
		return nil, err
	}
	jack, err := ensureUser(ctx, client, 1, "jack", hash, "jack@example.com")
	if err != nil {
		return nil, err
	}
	operator, err := ensureUser(ctx, client, 1, "mock_operator", hash, "mock_operator@example.com")
	if err != nil {
		return nil, err
	}
	tenantTwo, err := ensureUser(ctx, client, 2, "mock_tenant2", hash, "mock_tenant2@example.com")
	if err != nil {
		return nil, err
	}

	systemMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "System", Title: "系统管理", Path: "/system", Component: "BasicLayout", Icon: "ion:settings-outline", Type: 1, Sort: 900,
	}, 0)
	if err != nil {
		return nil, err
	}
	projectMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemProject", Title: "项目管理", Path: "/system/project", Component: "/system/project/list", Icon: "mdi:folder-cog-outline", Type: 2, Sort: 10,
	}, systemMenu.ID)
	if err != nil {
		return nil, err
	}
	tenantMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemTenant", Title: "租户管理", Path: "/system/tenant", Component: "/system/tenant/list", Icon: "mdi:office-building-cog-outline", Type: 2, Sort: 20,
	}, systemMenu.ID)
	if err != nil {
		return nil, err
	}
	roleMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemRole", Title: "角色管理", Path: "/system/role", Component: "/system/role/list", Icon: "mdi:account-group", Type: 2, Sort: 30,
	}, systemMenu.ID)
	if err != nil {
		return nil, err
	}
	menuMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemMenu", Title: "菜单管理", Path: "/system/menu", Component: "/system/menu/list", Icon: "mdi:menu", Type: 2, Sort: 40,
	}, systemMenu.ID)
	if err != nil {
		return nil, err
	}
	groupMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemMenuPermissionGroup", Title: "菜单权限组", Path: "/system/menu-permission-group", Component: "/system/menu-permission-group/list", Icon: "mdi:shield-key-outline", Type: 2, Sort: 50,
	}, systemMenu.ID)
	if err != nil {
		return nil, err
	}
	tenantPermissionMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemTenantPermission", Title: "租户授权", Path: "/system/tenant-permission", Component: "/system/tenant-permission/list", Icon: "mdi:domain", Type: 2, Sort: 60,
	}, systemMenu.ID)
	if err != nil {
		return nil, err
	}
	deptMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemDept", Title: "部门管理", Path: "/system/dept", Component: "/system/dept/list", Icon: "charm:organisation", Type: 2, Sort: 70,
	}, systemMenu.ID)
	if err != nil {
		return nil, err
	}
	userMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemUser", Title: "用户管理", Path: "/system/user", Component: "/system/user/list", Icon: "mdi:account-outline", Type: 2, Sort: 80,
	}, systemMenu.ID)
	if err != nil {
		return nil, err
	}
	dictionaryMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemDictionary", Title: "数据字典", Path: "/system/dictionary", Component: "/system/dictionary/list", Icon: "mdi:book-cog-outline", Type: 2, Sort: 90,
	}, systemMenu.ID)
	if err != nil {
		return nil, err
	}
	operationLogMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemOperationLog", Title: "操作审计", Path: "/system/operation-log", Component: "/system/operation-log/list", Icon: "mdi:clipboard-text-clock-outline", Type: 2, Sort: 100,
	}, systemMenu.ID)
	if err != nil {
		return nil, err
	}
	loginLogMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemLoginLog", Title: "登录日志", Path: "/system/login-log", Component: "/system/login-log/list", Icon: "mdi:shield-account-outline", Type: 2, Sort: 110,
	}, systemMenu.ID)
	if err != nil {
		return nil, err
	}
	sessionMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemSession", Title: "在线会话", Path: "/system/session", Component: "/system/session/list", Icon: "mdi:monitor-account", Type: 2, Sort: 120,
	}, systemMenu.ID)
	if err != nil {
		return nil, err
	}
	userCreateButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemUserCreate", Title: "用户新增", Path: "/system/user:create", Type: 3, Sort: 10, AuthCode: "/platform.admin.v1.UserService/CreateUser",
	}, userMenu.ID)
	if err != nil {
		return nil, err
	}

	fullMenuIDs := []uint32{systemMenu.ID, projectMenu.ID, tenantMenu.ID, roleMenu.ID, menuMenu.ID, groupMenu.ID, tenantPermissionMenu.ID, deptMenu.ID, userMenu.ID, dictionaryMenu.ID, operationLogMenu.ID, loginLogMenu.ID, sessionMenu.ID, userCreateButton.ID}
	basicMenuIDs := []uint32{systemMenu.ID, projectMenu.ID, roleMenu.ID, deptMenu.ID, userMenu.ID}

	fullGroup, err := ensureMenuPermissionGroup(ctx, client, "演示完整管理权限组", "demo-full-admin", true, fullMenuIDs)
	if err != nil {
		return nil, err
	}
	basicGroup, err := ensureMenuPermissionGroup(ctx, client, "演示基础业务权限组", "demo-basic-admin", true, basicMenuIDs)
	if err != nil {
		return nil, err
	}
	if err := ensureTenantPermissionGroups(ctx, client, 1, admin.ID, []uint32{fullGroup.ID}); err != nil {
		return nil, err
	}
	if err := ensureTenantPermissionGroups(ctx, client, 2, tenantTwo.ID, []uint32{basicGroup.ID}); err != nil {
		return nil, err
	}

	superRole, err := ensureRole(ctx, client, 1, "mock_super_admin", fullMenuIDs)
	if err != nil {
		return nil, err
	}
	operatorRole, err := ensureRole(ctx, client, 1, "mock_operator_role", basicMenuIDs)
	if err != nil {
		return nil, err
	}
	tenantTwoRole, err := ensureRole(ctx, client, 2, "mock_tenant2_role", basicMenuIDs)
	if err != nil {
		return nil, err
	}
	if err := assignUserRoles(ctx, admin, superRole.ID); err != nil {
		return nil, err
	}
	if err := assignUserRoles(ctx, vben, superRole.ID); err != nil {
		return nil, err
	}
	if err := assignUserRoles(ctx, webAdmin, superRole.ID); err != nil {
		return nil, err
	}
	if err := assignUserRoles(ctx, operator, operatorRole.ID); err != nil {
		return nil, err
	}
	if err := assignUserRoles(ctx, jack, operatorRole.ID); err != nil {
		return nil, err
	}
	if err := assignUserRoles(ctx, tenantTwo, tenantTwoRole.ID); err != nil {
		return nil, err
	}
	for i := 1; i <= 24; i++ {
		name := fmt.Sprintf("mock_member_%02d", i)
		item, err := ensureUser(ctx, client, 1, name, hash, fmt.Sprintf("%s@example.com", name))
		if err != nil {
			return nil, err
		}
		if err := assignUserRoles(ctx, item, operatorRole.ID); err != nil {
			return nil, err
		}
	}
	for i := 1; i <= 8; i++ {
		name := fmt.Sprintf("mock_tenant2_member_%02d", i)
		item, err := ensureUser(ctx, client, 2, name, hash, fmt.Sprintf("%s@example.com", name))
		if err != nil {
			return nil, err
		}
		if err := assignUserRoles(ctx, item, tenantTwoRole.ID); err != nil {
			return nil, err
		}
	}
	if err := ensurePost(ctx, client, 1, "mock_developer"); err != nil {
		return nil, err
	}
	if err := ensurePost(ctx, client, 1, "mock_manager"); err != nil {
		return nil, err
	}
	if err := ensurePost(ctx, client, 2, "mock_developer"); err != nil {
		return nil, err
	}
	rootDept, err := ensureDept(ctx, client, 1, "mock_headquarters", 0, admin.ID)
	if err != nil {
		return nil, err
	}
	if _, err := ensureDept(ctx, client, 1, "mock_engineering", rootDept.ID, operator.ID); err != nil {
		return nil, err
	}
	if _, err := ensureDept(ctx, client, 2, "mock_tenant2_dept", 0, tenantTwo.ID); err != nil {
		return nil, err
	}
	for i, name := range []string{"mock_product", "mock_quality", "mock_operations", "mock_finance", "mock_marketing", "mock_customer_success"} {
		if _, err := ensureDept(ctx, client, 1, name, rootDept.ID, operator.ID); err != nil {
			return nil, err
		}
		_ = i
	}
	if err := ensureProject(ctx, client, 1, "mock_admin_project", "MOCK-ADMIN", admin.ID, []uint32{admin.ID, operator.ID}); err != nil {
		return nil, err
	}
	if err := ensureProject(ctx, client, 2, "mock_tenant2_project", "MOCK-TENANT2", tenantTwo.ID, []uint32{tenantTwo.ID}); err != nil {
		return nil, err
	}
	for i := 1; i <= 18; i++ {
		if err := ensureProject(ctx, client, 1, fmt.Sprintf("mock_project_%02d", i), fmt.Sprintf("DEMO-%03d", i), admin.ID, []uint32{admin.ID, operator.ID}); err != nil {
			return nil, err
		}
	}
	for i := 1; i <= 6; i++ {
		if err := ensureProject(ctx, client, 2, fmt.Sprintf("mock_tenant2_project_%02d", i), fmt.Sprintf("T2-%03d", i), tenantTwo.ID, []uint32{tenantTwo.ID}); err != nil {
			return nil, err
		}
	}
	for _, tenantID := range []uint32{3, 4, 5, 6, 7, 8} {
		groupID := basicGroup.ID
		if tenantID%2 == 1 {
			groupID = fullGroup.ID
		}
		if err := ensureTenantPermissionGroups(ctx, client, tenantID, admin.ID, []uint32{groupID}); err != nil {
			return nil, err
		}
	}
	if err := seedDictionaries(ctx, client); err != nil {
		return nil, err
	}
	if err := seedOperationLogs(ctx, client, admin.ID, tenantTwo.ID); err != nil {
		return nil, err
	}
	if err := seedLoginLogs(ctx, client, admin.ID, tenantTwo.ID); err != nil {
		return nil, err
	}
	return &mockSeedResult{
		TenantOneUsers: []uint32{admin.ID, vben.ID, webAdmin.ID, operator.ID, jack.ID},
		TenantTwoUsers: []uint32{tenantTwo.ID},
	}, nil
}

func syncPolicies(ctx context.Context, cfg *conf.Data, logger log.Logger, result *mockSeedResult) error {
	if cfg == nil || cfg.Database == nil {
		return nil
	}
	authorizer, err := data.NewAuthorizer(cfg, logger)
	if err != nil {
		return err
	}
	defer authorizer.Close()
	if err := authzpolicy.SyncSuperAdmin(ctx, authorizer, "super_admin", "1", subjectIDs(result.TenantOneUsers)); err != nil {
		return err
	}
	return authzpolicy.SyncSuperAdmin(ctx, authorizer, "super_admin", "2", subjectIDs(result.TenantTwoUsers))
}

func subjectIDs(ids []uint32) []authz.Subject {
	subjects := make([]authz.Subject, 0, len(ids))
	for _, id := range ids {
		subjects = append(subjects, authz.Subject(strconv.FormatUint(uint64(id), 10)))
	}
	return subjects
}

func refreshTenantMenuCacheVersions(ctx context.Context, cfg *conf.Data, logger log.Logger, tenantIDs []uint32) error {
	if cfg == nil || cfg.Redis == nil {
		return nil
	}
	rdb, err := data.NewRedisClient(cfg, logger)
	if err != nil {
		return err
	}
	defer rdb.Close()
	if err := rdb.Incr(ctx, "platform:admin:menu:version").Err(); err != nil {
		return err
	}
	for _, tenantID := range tenantIDs {
		if tenantID == 0 {
			continue
		}
		key := fmt.Sprintf("platform:admin:tenant:%d:package_version", tenantID)
		if err := rdb.Incr(ctx, key).Err(); err != nil {
			return err
		}
	}
	return nil
}

func ensureTenant(ctx context.Context, client *gen.Client, id uint32, name, code, remark string) (*gen.Tenant, error) {
	item, err := client.Tenant.Query().Where(tenant.IDEQ(id)).Only(ctx)
	if err == nil {
		return item.Update().SetName(name).SetCode(code).SetStatus(1).SetRemark(remark).Save(ctx)
	}
	if !gen.IsNotFound(err) {
		return nil, err
	}
	return client.Tenant.Create().
		SetID(id).
		SetName(name).
		SetCode(code).
		SetStatus(1).
		SetLifecycleStatus(int32(pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE)).
		SetActivatedAt(time.Now()).
		SetSort(int32(id * 10)).
		SetRemark(remark).
		Save(ctx)
}

func ensureUser(ctx context.Context, client *gen.Client, tenantID uint32, name, password, email string) (*gen.User, error) {
	item, err := client.User.Query().Where(user.TenantIDEQ(tenantID), user.Name(name)).Only(ctx)
	if err == nil {
		return item.Update().SetPassword(password).SetEmail(email).SetStatus(1).Save(ctx)
	}
	if !gen.IsNotFound(err) {
		return nil, err
	}
	return client.User.Create().SetTenantID(tenantID).SetName(name).SetPassword(password).SetEmail(email).SetStatus(1).Save(ctx)
}

func ensureMenu(ctx context.Context, client *gen.Client, spec mockMenuSpec, parentID uint32) (*gen.Menu, error) {
	item, err := client.Menu.Query().Where(menu.Name(spec.Name)).Only(ctx)
	if err == nil {
		builder := item.Update().
			SetTitle(spec.Title).
			SetPath(spec.Path).
			SetComponent(spec.Component).
			SetIcon(spec.Icon).
			SetStatus(1).
			SetType(spec.Type).
			SetSort(spec.Sort).
			SetAuthCode(spec.AuthCode)
		if parentID > 0 {
			builder.SetParentID(parentID)
		}
		return builder.Save(ctx)
	}
	if !gen.IsNotFound(err) {
		return nil, err
	}
	builder := client.Menu.Create().
		SetName(spec.Name).
		SetTitle(spec.Title).
		SetPath(spec.Path).
		SetComponent(spec.Component).
		SetIcon(spec.Icon).
		SetStatus(1).
		SetType(spec.Type).
		SetSort(spec.Sort).
		SetAuthCode(spec.AuthCode)
	if parentID > 0 {
		builder.SetParentID(parentID)
	}
	return builder.Save(ctx)
}

func ensureMenuPermissionGroup(ctx context.Context, client *gen.Client, name, code string, system bool, menuIDs []uint32) (*gen.MenuPermissionGroup, error) {
	item, err := client.MenuPermissionGroup.Query().Where(menupermissiongroup.Code(code)).Only(ctx)
	if gen.IsNotFound(err) {
		return client.MenuPermissionGroup.Create().
			SetName(name).
			SetCode(code).
			SetStatus(1).
			SetIsSystem(system).
			SetDescription("mock data").
			AddMenuIDs(menuIDs...).
			Save(ctx)
	}
	if err != nil {
		return nil, err
	}
	return item.Update().
		SetName(name).
		SetStatus(1).
		SetIsSystem(system).
		SetDescription("mock data").
		ClearMenus().
		AddMenuIDs(menuIDs...).
		Save(ctx)
}

func ensureTenantPermissionGroups(ctx context.Context, client *gen.Client, tenantID, operatorID uint32, groupIDs []uint32) error {
	existing, err := client.TenantPermissionGroup.Query().
		Where(tenantpermissiongroup.TenantIDEQ(tenantID)).
		All(ctx)
	if err != nil {
		return err
	}
	keep := make(map[uint32]struct{}, len(groupIDs))
	for _, groupID := range groupIDs {
		keep[groupID] = struct{}{}
		item, err := client.TenantPermissionGroup.Query().
			Where(tenantpermissiongroup.TenantIDEQ(tenantID), tenantpermissiongroup.GroupIDEQ(groupID)).
			Only(ctx)
		if gen.IsNotFound(err) {
			if _, err := client.TenantPermissionGroup.Create().
				SetTenantID(tenantID).
				SetGroupID(groupID).
				SetEnabled(true).
				SetBoundBy(operatorID).
				Save(ctx); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if _, err := item.Update().SetEnabled(true).SetBoundBy(operatorID).Save(ctx); err != nil {
			return err
		}
	}
	for _, item := range existing {
		if _, ok := keep[item.GroupID]; ok {
			continue
		}
		if _, err := item.Update().SetEnabled(false).SetBoundBy(operatorID).Save(ctx); err != nil {
			return err
		}
	}
	return nil
}

func ensureRole(ctx context.Context, client *gen.Client, tenantID uint32, name string, menuIDs []uint32) (*gen.Role, error) {
	item, err := client.Role.Query().Where(role.TenantIDEQ(tenantID), role.Name(name)).Only(ctx)
	if gen.IsNotFound(err) {
		return client.Role.Create().SetTenantID(tenantID).SetName(name).SetStatus(1).AddMenuIDs(menuIDs...).Save(ctx)
	}
	if err != nil {
		return nil, err
	}
	return item.Update().SetStatus(1).ClearMenus().AddMenuIDs(menuIDs...).Save(ctx)
}

func assignUserRoles(ctx context.Context, item *gen.User, roleIDs ...uint32) error {
	_, err := item.Update().SetStatus(1).ClearRoles().AddRoleIDs(roleIDs...).Save(ctx)
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

type mockDictionarySpec struct {
	name   string
	code   string
	values []string
}

func seedDictionaries(ctx context.Context, client *gen.Client) error {
	tenantOne := []mockDictionarySpec{
		{"订单状态", "order_status", []string{"待支付", "已支付", "处理中", "已发货", "已完成", "已取消", "退款中", "已退款"}},
		{"用户等级", "user_level", []string{"普通会员", "银卡会员", "金卡会员", "铂金会员", "钻石会员"}},
		{"支付方式", "payment_method", []string{"微信支付", "支付宝", "银行卡", "账户余额", "线下转账"}},
		{"通知渠道", "notification_channel", []string{"站内信", "电子邮件", "短信", "企业微信", "Webhook"}},
		{"工单优先级", "ticket_priority", []string{"低", "普通", "高", "紧急", "阻断"}},
		{"客户来源", "customer_source", []string{"自然访问", "广告投放", "渠道推荐", "客户转介绍", "线下活动"}},
		{"合同状态", "contract_status", []string{"草稿", "审批中", "已生效", "即将到期", "已终止"}},
		{"发票类型", "invoice_type", []string{"增值税普通发票", "增值税专用发票", "电子普通发票"}},
		{"设备状态", "device_status", []string{"在线", "离线", "故障", "维护中", "已停用"}},
		{"数据敏感级别", "data_sensitivity", []string{"公开", "内部", "敏感", "机密"}},
	}
	tenantTwo := []mockDictionarySpec{
		{"订单状态", "order_status", []string{"新建", "确认", "执行中", "完成", "关闭"}},
		{"业务区域", "business_region", []string{"华东", "华南", "华北", "西南", "海外"}},
		{"服务等级", "service_level", []string{"标准", "专业", "企业"}},
		{"审批结果", "approval_result", []string{"待审批", "通过", "驳回", "撤回"}},
	}
	for _, tenant := range []struct {
		id    uint32
		specs []mockDictionarySpec
	}{{1, tenantOne}, {2, tenantTwo}} {
		for typeIndex, spec := range tenant.specs {
			dictionary, err := ensureDictionaryType(ctx, client, tenant.id, spec, int32((typeIndex+1)*10))
			if err != nil {
				return err
			}
			for itemIndex, label := range spec.values {
				if err := ensureDictionaryItem(ctx, client, tenant.id, dictionary.ID, label, fmt.Sprintf("%s_%02d", spec.code, itemIndex+1), int32((itemIndex+1)*10)); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func ensureDictionaryType(ctx context.Context, client *gen.Client, tenantID uint32, spec mockDictionarySpec, sort int32) (*gen.DictionaryType, error) {
	item, err := client.DictionaryType.Query().
		Where(dictionarytype.TenantIDEQ(tenantID), dictionarytype.CodeEQ(spec.code)).
		Only(ctx)
	if gen.IsNotFound(err) {
		return client.DictionaryType.Create().
			SetTenantID(tenantID).
			SetName(spec.name).
			SetCode(spec.code).
			SetStatus(1).
			SetSort(sort).
			SetRemark("后台展示 mock 数据").
			Save(ctx)
	}
	if err != nil {
		return nil, err
	}
	return item.Update().SetName(spec.name).SetStatus(1).SetSort(sort).SetRemark("后台展示 mock 数据").Save(ctx)
}

func ensureDictionaryItem(ctx context.Context, client *gen.Client, tenantID, typeID uint32, label, value string, sort int32) error {
	item, err := client.DictionaryItem.Query().
		Where(dictionaryitem.TenantIDEQ(tenantID), dictionaryitem.TypeIDEQ(typeID), dictionaryitem.ValueEQ(value)).
		Only(ctx)
	color := []string{"blue", "green", "orange", "red", "purple"}[(sort/10-1)%5]
	if gen.IsNotFound(err) {
		_, err = client.DictionaryItem.Create().
			SetTenantID(tenantID).
			SetTypeID(typeID).
			SetLabel(label).
			SetValue(value).
			SetStatus(1).
			SetSort(sort).
			SetColor(color).
			SetRemark("mock item").
			Save(ctx)
		return err
	}
	if err != nil {
		return err
	}
	_, err = item.Update().SetLabel(label).SetStatus(1).SetSort(sort).SetColor(color).Save(ctx)
	return err
}

func seedOperationLogs(ctx context.Context, client *gen.Client, tenantOneOperator, tenantTwoOperator uint32) error {
	if err := seedTenantOperationLogs(ctx, client, 1, tenantOneOperator, 120); err != nil {
		return err
	}
	return seedTenantOperationLogs(ctx, client, 2, tenantTwoOperator, 40)
}

func seedTenantOperationLogs(ctx context.Context, client *gen.Client, tenantID, operatorID uint32, count int) error {
	modules := []string{"tenant", "user", "role", "menu", "dictionary", "project", "department", "package"}
	actions := []string{"create", "update", "delete", "enable", "disable", "export"}
	methods := []string{"POST", "PUT", "DELETE", "PUT", "PUT", "GET"}
	baseTime := time.Now().Add(-time.Duration(count) * time.Minute)
	for i := 1; i <= count; i++ {
		module := modules[(i-1)%len(modules)]
		actionIndex := (i - 1) % len(actions)
		success := i%9 != 0
		traceID := fmt.Sprintf("mock-t%d-%04d", tenantID, i)
		createdAt := baseTime.Add(time.Duration(i) * time.Minute)
		errorMessage := ""
		if !success {
			errorMessage = "演示失败：业务规则校验未通过"
		}
		item, err := client.OperationLog.Query().Where(operationlog.TraceIDEQ(traceID)).Only(ctx)
		if gen.IsNotFound(err) {
			_, err = client.OperationLog.Create().
				SetTenantID(tenantID).
				SetOperatorID(operatorID).
				SetOperatorName(fmt.Sprintf("mock_operator_t%d", tenantID)).
				SetModule(module).
				SetAction(actions[actionIndex]).
				SetResourceType(module).
				SetResourceID(strconv.Itoa(i)).
				SetOperation(fmt.Sprintf("/platform.admin.v1.%sService/%s", module, actions[actionIndex])).
				SetMethod(methods[actionIndex]).
				SetPath(fmt.Sprintf("/admin/v1/%ss/%d", module, i)).
				SetRequestSummary(fmt.Sprintf(`{"mock":true,"sequence":%d}`, i)).
				SetIP(fmt.Sprintf("192.168.%d.%d", tenantID, i%250+1)).
				SetUserAgent("AVMC Mock Client/1.0").
				SetTraceID(traceID).
				SetSuccess(success).
				SetDurationMs(int64(15 + i%480)).
				SetErrorMessage(errorMessage).
				SetCreatedAt(createdAt).
				Save(ctx)
			if err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if _, err := item.Update().
			SetModule(module).
			SetAction(actions[actionIndex]).
			SetSuccess(success).
			SetDurationMs(int64(15 + i%480)).
			SetErrorMessage(errorMessage).
			Save(ctx); err != nil {
			return err
		}
	}
	return nil
}

func seedLoginLogs(ctx context.Context, client *gen.Client, tenantOneUser, tenantTwoUser uint32) error {
	if err := seedTenantLoginLogs(ctx, client, 1, tenantOneUser, "admin", 80); err != nil {
		return err
	}
	return seedTenantLoginLogs(ctx, client, 2, tenantTwoUser, "mock_tenant2", 30)
}

func seedTenantLoginLogs(ctx context.Context, client *gen.Client, tenantID, userID uint32, identity string, count int) error {
	baseTime := time.Now().Add(-time.Duration(count) * 10 * time.Minute)
	for i := 1; i <= count; i++ {
		result := "success"
		reason := ""
		var eventUserID *uint32
		if i%5 == 0 {
			result = "failure"
			reason = "用户名或密码错误"
		} else {
			id := userID
			eventUserID = &id
		}
		if i%17 == 0 {
			result = "locked"
			reason = "登录失败次数过多，请稍后重试"
			eventUserID = nil
		}
		traceID := fmt.Sprintf("mock-login-t%d-%04d", tenantID, i)
		item, err := client.LoginLog.Query().Where(loginlog.TraceIDEQ(traceID)).Only(ctx)
		if gen.IsNotFound(err) {
			_, err = client.LoginLog.Create().
				SetTenantID(tenantID).
				SetNillableUserID(eventUserID).
				SetIdentity(identity).
				SetLoginType("username").
				SetResult(result).
				SetFailureReason(reason).
				SetIP(fmt.Sprintf("172.16.%d.%d", tenantID, i%250+1)).
				SetUserAgent("Mock Browser/1.0").
				SetTraceID(traceID).
				SetCreatedAt(baseTime.Add(time.Duration(i) * 10 * time.Minute)).
				Save(ctx)
			if err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if _, err := item.Update().
			SetNillableUserID(eventUserID).
			SetResult(result).
			SetFailureReason(reason).
			Save(ctx); err != nil {
			return err
		}
	}
	return nil
}

func verify(ctx context.Context, client *gen.Client) error {
	checks := []struct {
		name  string
		count func(context.Context) (int, error)
		min   int
	}{
		{"tenant 1 users", func(ctx context.Context) (int, error) {
			return client.User.Query().Where(user.TenantIDEQ(1)).Count(ctx)
		}, 29},
		{"tenant 2 users", func(ctx context.Context) (int, error) {
			return client.User.Query().Where(user.TenantIDEQ(2)).Count(ctx)
		}, 9},
		{"menus", func(ctx context.Context) (int, error) {
			return client.Menu.Query().Where(menu.NameHasPrefix("System")).Count(ctx)
		}, 9},
		{"tenants", func(ctx context.Context) (int, error) {
			return client.Tenant.Query().Where(tenant.IDIn(1, 2, 3, 4, 5, 6, 7, 8)).Count(ctx)
		}, 8},
		{"permission groups", func(ctx context.Context) (int, error) {
			return client.MenuPermissionGroup.Query().Where(menupermissiongroup.CodeIn("demo-full-admin", "demo-basic-admin")).Count(ctx)
		}, 2},
		{"tenant permission bindings", func(ctx context.Context) (int, error) {
			return client.TenantPermissionGroup.Query().Where(tenantpermissiongroup.TenantIDIn(1, 2, 3, 4, 5, 6, 7, 8), tenantpermissiongroup.EnabledEQ(true)).Count(ctx)
		}, 8},
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
		}, 8},
		{"tenant 2 departments", func(ctx context.Context) (int, error) {
			return client.Dept.Query().Where(dept.TenantIDEQ(2)).Count(ctx)
		}, 1},
		{"tenant 1 projects", func(ctx context.Context) (int, error) {
			return client.Project.Query().Where(project.TenantIDEQ(1)).Count(ctx)
		}, 19},
		{"tenant 2 projects", func(ctx context.Context) (int, error) {
			return client.Project.Query().Where(project.TenantIDEQ(2)).Count(ctx)
		}, 7},
		{"tenant 1 dictionary types", func(ctx context.Context) (int, error) {
			return client.DictionaryType.Query().Where(dictionarytype.TenantIDEQ(1)).Count(ctx)
		}, 10},
		{"tenant 2 dictionary types", func(ctx context.Context) (int, error) {
			return client.DictionaryType.Query().Where(dictionarytype.TenantIDEQ(2)).Count(ctx)
		}, 4},
		{"tenant 1 dictionary items", func(ctx context.Context) (int, error) {
			return client.DictionaryItem.Query().Where(dictionaryitem.TenantIDEQ(1)).Count(ctx)
		}, 48},
		{"tenant 2 dictionary items", func(ctx context.Context) (int, error) {
			return client.DictionaryItem.Query().Where(dictionaryitem.TenantIDEQ(2)).Count(ctx)
		}, 17},
		{"tenant 1 operation logs", func(ctx context.Context) (int, error) {
			return client.OperationLog.Query().Where(operationlog.TenantIDEQ(1)).Count(ctx)
		}, 120},
		{"tenant 2 operation logs", func(ctx context.Context) (int, error) {
			return client.OperationLog.Query().Where(operationlog.TenantIDEQ(2)).Count(ctx)
		}, 40},
		{"tenant 1 login logs", func(ctx context.Context) (int, error) {
			return client.LoginLog.Query().Where(loginlog.TenantIDEQ(1)).Count(ctx)
		}, 80},
		{"tenant 2 login logs", func(ctx context.Context) (int, error) {
			return client.LoginLog.Query().Where(loginlog.TenantIDEQ(2)).Count(ctx)
		}, 30},
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
