package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	pbCore "backend-service/api/core/service/v1"
	pbAdmin "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/authzpolicy"
	"backend-service/app/platform/admin/internal/conf"
	"backend-service/app/platform/admin/internal/data"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/asynctask"
	"backend-service/app/platform/admin/internal/data/ent/gen/dept"
	"backend-service/app/platform/admin/internal/data/ent/gen/dictionaryitem"
	"backend-service/app/platform/admin/internal/data/ent/gen/dictionarytype"
	"backend-service/app/platform/admin/internal/data/ent/gen/fileaccesslog"
	"backend-service/app/platform/admin/internal/data/ent/gen/fileobject"
	"backend-service/app/platform/admin/internal/data/ent/gen/loginlog"
	"backend-service/app/platform/admin/internal/data/ent/gen/menu"
	"backend-service/app/platform/admin/internal/data/ent/gen/menupermissiongroup"
	"backend-service/app/platform/admin/internal/data/ent/gen/menupermissiongroupversion"
	"backend-service/app/platform/admin/internal/data/ent/gen/notificationmessage"
	"backend-service/app/platform/admin/internal/data/ent/gen/notificationtemplate"
	"backend-service/app/platform/admin/internal/data/ent/gen/operationlog"
	"backend-service/app/platform/admin/internal/data/ent/gen/parameterdefinition"
	"backend-service/app/platform/admin/internal/data/ent/gen/post"
	"backend-service/app/platform/admin/internal/data/ent/gen/project"
	"backend-service/app/platform/admin/internal/data/ent/gen/role"
	"backend-service/app/platform/admin/internal/data/ent/gen/storageprovider"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenant"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenantparameteroverride"
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
	if err := syncPolicies(ctx, bc.Data, client, logger, result); err != nil {
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
		{"legacy mock file objects", "DELETE FROM file_objects WHERE file_name LIKE 'mock\\_%'"},
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
	if _, err := ensureTenant(ctx, client, 1, "演示平台租户", "demo-platform", "拥有完整系统管理能力的演示租户", true); err != nil {
		return nil, err
	}
	if _, err := ensureTenant(ctx, client, 2, "演示业务租户", "demo-business", "只开放基础能力的演示租户", false); err != nil {
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
		item, err := ensureTenant(ctx, client, spec.id, spec.name, spec.code, spec.remark, false)
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
	parameterMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemParameter", Title: "参数配置", Path: "/system/parameter", Component: "/system/parameter/list", Icon: "mdi:tune-variant", Type: 2, Sort: 95,
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
	asyncTaskMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemAsyncTask", Title: "异步任务", Path: "/system/async-task", Component: "/system/async-task/list", Icon: "mdi:progress-clock", Type: 2, Sort: 130,
	}, systemMenu.ID)
	if err != nil {
		return nil, err
	}
	notificationMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemNotification", Title: "通知中心", Path: "/system/notification", Component: "/system/notification/list", Icon: "mdi:bell-outline", Type: 2, Sort: 135,
	}, systemMenu.ID)
	if err != nil {
		return nil, err
	}
	fileCenterMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemFileCenter", Title: "文件中心", Path: "/system/file-center", Component: "/system/file-center/list", Icon: "mdi:file-cloud-outline", Type: 2, Sort: 140,
	}, systemMenu.ID)
	if err != nil {
		return nil, err
	}
	storageProviderMenu, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemStorageProvider", Title: "存储渠道", Path: "/system/storage-provider", Component: "/system/storage-provider/list", Icon: "mdi:database-cog-outline", Type: 2, Sort: 145,
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
	projectListButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemProjectList", Title: "项目查看", Path: "/system/project:list", Type: 3, Sort: 10, AuthCode: "/platform.admin.v1.ProjectService/ListProjects",
	}, projectMenu.ID)
	if err != nil {
		return nil, err
	}
	roleListButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemRoleList", Title: "角色查看", Path: "/system/role:list", Type: 3, Sort: 10, AuthCode: "/platform.admin.v1.RoleService/ListRoles",
	}, roleMenu.ID)
	if err != nil {
		return nil, err
	}
	deptListButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemDeptList", Title: "部门查看", Path: "/system/dept:list", Type: 3, Sort: 10, AuthCode: "/platform.admin.v1.DeptService/ListDepts",
	}, deptMenu.ID)
	if err != nil {
		return nil, err
	}
	userListButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemUserList", Title: "用户查看", Path: "/system/user:list", Type: 3, Sort: 20, AuthCode: "/platform.admin.v1.UserService/ListUsers",
	}, userMenu.ID)
	if err != nil {
		return nil, err
	}
	parameterCurrentButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemParameterCurrent", Title: "租户参数查看", Path: "/system/parameter:list", Type: 3, Sort: 20, AuthCode: "/platform.admin.v1.ParameterService/ListCurrentTenantParameters",
	}, parameterMenu.ID)
	if err != nil {
		return nil, err
	}
	parameterManageButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemParameterManage", Title: "平台参数定义管理", Path: "/system/parameter:manage", Type: 3, Sort: 10, AuthCode: "/platform.admin.v1.ParameterService/ListParameterDefinitions",
	}, parameterMenu.ID)
	if err != nil {
		return nil, err
	}
	fileListButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemFileCenterList", Title: "文件查看", Path: "/system/file-center:list", Type: 3, Sort: 10, AuthCode: "/platform.admin.v1.FileCenterService/ListFileObjects",
	}, fileCenterMenu.ID)
	if err != nil {
		return nil, err
	}
	fileDetailButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemFileCenterDetail", Title: "文件详情", Path: "/system/file-center:detail", Type: 3, Sort: 15, AuthCode: "/platform.admin.v1.FileCenterService/GetFileObject",
	}, fileCenterMenu.ID)
	if err != nil {
		return nil, err
	}
	fileUploadButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemFileCenterUpload", Title: "文件上传", Path: "/system/file-center:upload", Type: 3, Sort: 20, AuthCode: "/platform.admin.v1.FileCenterService/CreateFileUploadSession",
	}, fileCenterMenu.ID)
	if err != nil {
		return nil, err
	}
	fileContentUploadButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemFileCenterContentUpload", Title: "内容上传", Path: "/system/file-center:content-upload", Type: 3, Sort: 22, AuthCode: "/platform.admin.v1.FileCenterService/UploadFileContent",
	}, fileCenterMenu.ID)
	if err != nil {
		return nil, err
	}
	fileConfirmButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemFileCenterConfirm", Title: "确认上传", Path: "/system/file-center:confirm", Type: 3, Sort: 25, AuthCode: "/platform.admin.v1.FileCenterService/ConfirmFileUpload",
	}, fileCenterMenu.ID)
	if err != nil {
		return nil, err
	}
	fileDownloadButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemFileCenterDownload", Title: "文件下载", Path: "/system/file-center:download", Type: 3, Sort: 30, AuthCode: "/platform.admin.v1.FileCenterService/PresignFileDownload",
	}, fileCenterMenu.ID)
	if err != nil {
		return nil, err
	}
	fileAccessLogButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemFileCenterAccessLog", Title: "访问记录", Path: "/system/file-center:access-log", Type: 3, Sort: 35, AuthCode: "/platform.admin.v1.FileCenterService/ListFileAccessLogs",
	}, fileCenterMenu.ID)
	if err != nil {
		return nil, err
	}
	fileDeleteButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemFileCenterDelete", Title: "文件删除", Path: "/system/file-center:delete", Type: 3, Sort: 40, AuthCode: "/platform.admin.v1.FileCenterService/DeleteFileObject",
	}, fileCenterMenu.ID)
	if err != nil {
		return nil, err
	}
	notificationTemplateButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemNotificationTemplate", Title: "模板管理", Path: "/system/notification:template", Type: 3, Sort: 10, AuthCode: "/platform.admin.v1.NotificationService/ListNotificationTemplates",
	}, notificationMenu.ID)
	if err != nil {
		return nil, err
	}
	notificationMessageButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemNotificationMessage", Title: "通知记录", Path: "/system/notification:message", Type: 3, Sort: 20, AuthCode: "/platform.admin.v1.NotificationService/ListNotificationMessages",
	}, notificationMenu.ID)
	if err != nil {
		return nil, err
	}
	notificationSendButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemNotificationSend", Title: "发送站内信", Path: "/system/notification:send", Type: 3, Sort: 30, AuthCode: "/platform.admin.v1.NotificationService/SendInAppNotification",
	}, notificationMenu.ID)
	if err != nil {
		return nil, err
	}
	storageProviderListButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemStorageProviderList", Title: "存储渠道查看", Path: "/system/storage-provider:list", Type: 3, Sort: 10, AuthCode: "/platform.admin.v1.StorageProviderService/ListStorageProviders",
	}, storageProviderMenu.ID)
	if err != nil {
		return nil, err
	}
	storageProviderManageButton, err := ensureMenu(ctx, client, mockMenuSpec{
		Name: "SystemStorageProviderManage", Title: "存储渠道管理", Path: "/system/storage-provider:manage", Type: 3, Sort: 20, AuthCode: "/platform.admin.v1.StorageProviderService/CreateStorageProvider",
	}, storageProviderMenu.ID)
	if err != nil {
		return nil, err
	}

	basicPermissionMenuIDs := []uint32{projectListButton.ID, roleListButton.ID, deptListButton.ID, userListButton.ID, parameterCurrentButton.ID}
	fileCenterMenuIDs := []uint32{fileCenterMenu.ID, fileListButton.ID, fileDetailButton.ID, fileUploadButton.ID, fileContentUploadButton.ID, fileConfirmButton.ID, fileDownloadButton.ID, fileAccessLogButton.ID, fileDeleteButton.ID}
	notificationMenuIDs := []uint32{notificationMenu.ID, notificationTemplateButton.ID, notificationMessageButton.ID, notificationSendButton.ID}
	storageProviderMenuIDs := []uint32{storageProviderMenu.ID, storageProviderListButton.ID, storageProviderManageButton.ID}
	fullMenuIDs := []uint32{systemMenu.ID, projectMenu.ID, tenantMenu.ID, roleMenu.ID, menuMenu.ID, groupMenu.ID, tenantPermissionMenu.ID, deptMenu.ID, userMenu.ID, dictionaryMenu.ID, parameterMenu.ID, operationLogMenu.ID, loginLogMenu.ID, sessionMenu.ID, asyncTaskMenu.ID, userCreateButton.ID, parameterManageButton.ID}
	fullMenuIDs = append(fullMenuIDs, basicPermissionMenuIDs...)
	fullMenuIDs = append(fullMenuIDs, fileCenterMenuIDs...)
	fullMenuIDs = append(fullMenuIDs, notificationMenuIDs...)
	fullMenuIDs = append(fullMenuIDs, storageProviderMenuIDs...)
	basicMenuIDs := []uint32{systemMenu.ID, projectMenu.ID, roleMenu.ID, deptMenu.ID, userMenu.ID, parameterMenu.ID}
	basicMenuIDs = append(basicMenuIDs, basicPermissionMenuIDs...)

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

	superRole, err := ensureRole(ctx, client, 1, "mock_super_admin", fullMenuIDs, true)
	if err != nil {
		return nil, err
	}
	operatorRole, err := ensureRole(ctx, client, 1, "mock_operator_role", basicMenuIDs, false)
	if err != nil {
		return nil, err
	}
	tenantTwoRole, err := ensureRole(ctx, client, 2, "mock_tenant2_role", basicMenuIDs, true)
	if err != nil {
		return nil, err
	}
	tenantTwoMemberRole, err := ensureRole(ctx, client, 2, "mock_tenant2_member_role", basicMenuIDs, false)
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
		if err := assignUserRoles(ctx, item, tenantTwoMemberRole.ID); err != nil {
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
	engineeringDept, err := ensureDept(ctx, client, 1, "mock_engineering", rootDept.ID, operator.ID)
	if err != nil {
		return nil, err
	}
	tenantTwoDept, err := ensureDept(ctx, client, 2, "mock_tenant2_dept", 0, tenantTwo.ID)
	if err != nil {
		return nil, err
	}
	if _, err := admin.Update().SetDeptID(rootDept.ID).Save(ctx); err != nil {
		return nil, err
	}
	if _, err := operator.Update().SetDeptID(engineeringDept.ID).Save(ctx); err != nil {
		return nil, err
	}
	if _, err := jack.Update().SetDeptID(engineeringDept.ID).Save(ctx); err != nil {
		return nil, err
	}
	if _, err := tenantTwo.Update().SetDeptID(tenantTwoDept.ID).Save(ctx); err != nil {
		return nil, err
	}
	if _, err := client.User.Update().
		Where(user.TenantIDEQ(1), user.NameHasPrefix("mock_member_")).
		SetDeptID(engineeringDept.ID).
		Save(ctx); err != nil {
		return nil, err
	}
	if _, err := client.User.Update().
		Where(user.TenantIDEQ(2), user.NameHasPrefix("mock_tenant2_member_")).
		SetDeptID(tenantTwoDept.ID).
		Save(ctx); err != nil {
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
	if err := seedParameters(ctx, client, admin.ID, tenantTwo.ID); err != nil {
		return nil, err
	}
	defaultStorageProvider, err := seedStorageProviders(ctx, client)
	if err != nil {
		return nil, err
	}
	if err := seedFileObjects(ctx, client, defaultStorageProvider, admin.ID, tenantTwo.ID); err != nil {
		return nil, err
	}
	if err := seedFileAccessLogs(ctx, client, admin.ID, tenantTwo.ID); err != nil {
		return nil, err
	}
	if err := seedNotifications(ctx, client, admin.ID, vben.ID, tenantTwo.ID); err != nil {
		return nil, err
	}
	if err := seedAsyncTasks(ctx, client, admin.ID); err != nil {
		return nil, err
	}
	if err := seedOperationLogs(ctx, client, admin.ID, tenantTwo.ID); err != nil {
		return nil, err
	}
	if err := seedLoginLogs(ctx, client, admin.ID, tenantTwo.ID); err != nil {
		return nil, err
	}
	return &mockSeedResult{
		TenantOneUsers: []uint32{admin.ID, vben.ID, webAdmin.ID},
		TenantTwoUsers: []uint32{tenantTwo.ID},
	}, nil
}

func syncPolicies(ctx context.Context, cfg *conf.Data, client *gen.Client, logger log.Logger, result *mockSeedResult) error {
	if cfg == nil || cfg.Database == nil {
		return nil
	}
	authorizer, err := data.NewAuthorizer(cfg, client, &data.Data{}, logger)
	if err != nil {
		return err
	}
	defer authorizer.Close()
	if err := authzpolicy.SyncPlatformAdmin(ctx, authorizer, "super_admin", "1", subjectIDs(result.TenantOneUsers)); err != nil {
		return err
	}
	if err := authzpolicy.SyncSuperAdmin(ctx, authorizer, "super_admin", "2", subjectIDs(result.TenantTwoUsers)); err != nil {
		return err
	}
	platformUser := authz.Subject(strconv.FormatUint(uint64(result.TenantOneUsers[0]), 10))
	if allowed, err := authorizer.Enforce(ctx, platformUser, authz.Object(pbAdmin.OperationTenantServiceListTenants), "GET", "1"); err != nil || !allowed {
		return fmt.Errorf("platform policy verification failed: allowed=%t err=%v", allowed, err)
	}
	tenantUser := authz.Subject(strconv.FormatUint(uint64(result.TenantTwoUsers[0]), 10))
	if allowed, _ := authorizer.Enforce(ctx, tenantUser, authz.Object(pbAdmin.OperationTenantServiceListTenants), "GET", "2"); allowed {
		return fmt.Errorf("business tenant unexpectedly has platform control policy")
	}
	return nil
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
	if err := rdb.Incr(ctx, "platform:admin:parameter:global_version").Err(); err != nil {
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
		parameterKey := fmt.Sprintf("platform:admin:parameter:tenant:%d:version", tenantID)
		if err := rdb.Incr(ctx, parameterKey).Err(); err != nil {
			return err
		}
	}
	return nil
}

func ensureTenant(ctx context.Context, client *gen.Client, id uint32, name, code, remark string, isPlatform bool) (*gen.Tenant, error) {
	item, err := client.Tenant.Query().Where(tenant.IDEQ(id)).Only(ctx)
	if err == nil {
		return item.Update().
			SetName(name).
			SetCode(code).
			SetStatus(1).
			SetRemark(remark).
			SetIsPlatform(isPlatform).
			Save(ctx)
	}
	if !gen.IsNotFound(err) {
		return nil, err
	}
	return client.Tenant.Create().
		SetID(id).
		SetName(name).
		SetCode(code).
		SetIsPlatform(isPlatform).
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
	apiPermissions, featureFlags, resourceQuotas := mockPackageCapabilities(code)
	item, err := client.MenuPermissionGroup.Query().Where(menupermissiongroup.Code(code)).Only(ctx)
	if gen.IsNotFound(err) {
		item, err = client.MenuPermissionGroup.Create().
			SetName(name).
			SetCode(code).
			SetStatus(1).
			SetIsSystem(system).
			SetDescription("mock data").
			SetAPIPermissions(apiPermissions).
			SetFeatureFlags(featureFlags).
			SetResourceQuotas(resourceQuotas).
			AddMenuIDs(menuIDs...).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	} else {
		item, err = item.Update().
			SetName(name).
			SetStatus(1).
			SetIsSystem(system).
			SetDescription("mock data").
			SetAPIPermissions(apiPermissions).
			SetFeatureFlags(featureFlags).
			SetResourceQuotas(resourceQuotas).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	}
	return ensureMenuPermissionGroupVersion(ctx, client, item, menuIDs, apiPermissions, featureFlags, resourceQuotas)
}

func ensureMenuPermissionGroupVersion(ctx context.Context, client *gen.Client, group *gen.MenuPermissionGroup, menuIDs []uint32, apiPermissions []string, featureFlags map[string]bool, resourceQuotas map[string]int64) (*gen.MenuPermissionGroup, error) {
	currentMenus, err := group.QueryMenus().IDs(ctx)
	if err != nil {
		return nil, err
	}
	_, err = group.QueryCurrentVersion().Only(ctx)
	if err == nil &&
		sameIDs(currentMenus, menuIDs) &&
		sameStrings(group.APIPermissions, apiPermissions) &&
		reflect.DeepEqual(group.FeatureFlags, featureFlags) &&
		reflect.DeepEqual(group.ResourceQuotas, resourceQuotas) {
		return group, nil
	}
	if err != nil && !gen.IsNotFound(err) {
		return nil, err
	}
	nextVersion := int32(1)
	if latest, latestErr := client.MenuPermissionGroupVersion.Query().
		Where(menupermissiongroupversion.GroupIDEQ(group.ID)).
		Order(gen.Desc(menupermissiongroupversion.FieldVersion)).
		First(ctx); latestErr == nil {
		nextVersion = latest.Version + 1
	} else if !gen.IsNotFound(latestErr) {
		return nil, latestErr
	}
	tx, err := client.Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if _, err = tx.MenuPermissionGroupVersion.Update().
		Where(menupermissiongroupversion.GroupIDEQ(group.ID)).
		SetState(int32(pbCore.MenuPermissionGroupVersionState_MENU_PERMISSION_GROUP_VERSION_STATE_SUPERSEDED)).
		Save(ctx); err != nil {
		return nil, err
	}
	now := time.Now()
	version, err := tx.MenuPermissionGroupVersion.Create().
		SetGroupID(group.ID).
		SetVersion(nextVersion).
		SetState(int32(pbCore.MenuPermissionGroupVersionState_MENU_PERMISSION_GROUP_VERSION_STATE_PUBLISHED)).
		SetChangeSummary("mock data sync").
		SetEffectiveAt(now).
		SetPublishedAt(now).
		SetAPIPermissions(apiPermissions).
		SetFeatureFlags(featureFlags).
		SetResourceQuotas(resourceQuotas).
		AddMenuIDs(menuIDs...).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	if _, err = tx.MenuPermissionGroup.UpdateOneID(group.ID).
		SetCurrentVersionID(version.ID).
		ClearMenus().
		SetAPIPermissions(apiPermissions).
		SetFeatureFlags(featureFlags).
		SetResourceQuotas(resourceQuotas).
		AddMenuIDs(menuIDs...).
		Save(ctx); err != nil {
		return nil, err
	}
	if _, err = tx.TenantPermissionGroup.Update().
		Where(
			tenantpermissiongroup.GroupIDEQ(group.ID),
			tenantpermissiongroup.AutoUpgradeEQ(true),
		).
		SetVersionID(version.ID).
		Save(ctx); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return client.MenuPermissionGroup.Get(ctx, group.ID)
}

func mockPackageCapabilities(code string) ([]string, map[string]bool, map[string]int64) {
	switch code {
	case "demo-full-admin":
		return []string{
				"platform.admin.audit.read",
				"platform.admin.async_task.manage",
				"platform.admin.file.manage",
				"platform.admin.notification.manage",
				"platform.admin.tenant.manage",
				"platform.admin.webhook.manage",
			},
			map[string]bool{
				"advanced_reports": true,
				"file_center":      true,
				"notification":     true,
				"webhook_center":   true,
			},
			map[string]int64{
				"api_calls_per_day": 100000,
				"files":             10000,
				"projects":          100,
				"storage.bytes":     107374182400,
				"storage_mb":        102400,
				"webhooks":          50,
			}
	default:
		return []string{
				"platform.admin.audit.read",
				"platform.admin.tenant.read",
			},
			map[string]bool{
				"advanced_reports": false,
				"file_center":      false,
				"notification":     false,
				"webhook_center":   false,
			},
			map[string]int64{
				"api_calls_per_day": 5000,
				"files":             0,
				"projects":          10,
				"storage.bytes":     0,
				"storage_mb":        5120,
				"webhooks":          0,
			}
	}
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
		group, err := client.MenuPermissionGroup.Get(ctx, groupID)
		if err != nil {
			return err
		}
		keep[groupID] = struct{}{}
		item, err := client.TenantPermissionGroup.Query().
			Where(tenantpermissiongroup.TenantIDEQ(tenantID), tenantpermissiongroup.GroupIDEQ(groupID)).
			Only(ctx)
		if gen.IsNotFound(err) {
			if _, err := client.TenantPermissionGroup.Create().
				SetTenantID(tenantID).
				SetGroupID(groupID).
				SetEnabled(true).
				SetAutoUpgrade(true).
				SetNillableVersionID(group.CurrentVersionID).
				SetBoundBy(operatorID).
				Save(ctx); err != nil {
				return err
			}
			continue
		}
		if err != nil {
			return err
		}
		if _, err := item.Update().
			SetEnabled(true).
			SetAutoUpgrade(true).
			SetNillableVersionID(group.CurrentVersionID).
			SetBoundBy(operatorID).
			Save(ctx); err != nil {
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

func sameIDs(left, right []uint32) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[uint32]struct{}, len(left))
	for _, id := range left {
		seen[id] = struct{}{}
	}
	for _, id := range right {
		if _, ok := seen[id]; !ok {
			return false
		}
	}
	return true
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	seen := make(map[string]struct{}, len(left))
	for _, item := range left {
		seen[item] = struct{}{}
	}
	for _, item := range right {
		if _, ok := seen[item]; !ok {
			return false
		}
	}
	return true
}

func ensureRole(ctx context.Context, client *gen.Client, tenantID uint32, name string, menuIDs []uint32, isTenantAdmin bool) (*gen.Role, error) {
	dataScope := int32(4)
	if isTenantAdmin {
		dataScope = 1
	}
	item, err := client.Role.Query().Where(role.TenantIDEQ(tenantID), role.Name(name)).Only(ctx)
	if gen.IsNotFound(err) {
		return client.Role.Create().SetTenantID(tenantID).SetName(name).SetStatus(1).SetDataScope(dataScope).SetIsTenantAdmin(isTenantAdmin).AddMenuIDs(menuIDs...).Save(ctx)
	}
	if err != nil {
		return nil, err
	}
	return item.Update().SetStatus(1).SetDataScope(dataScope).SetIsTenantAdmin(isTenantAdmin).ClearMenus().AddMenuIDs(menuIDs...).Save(ctx)
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

func seedStorageProviders(ctx context.Context, client *gen.Client) (*gen.StorageProvider, error) {
	local, err := ensureStorageProvider(ctx, client, "mock-local-default", "本地默认存储", "local", "", "", "", "", "", false, true, "http://localhost:8000/admin/v1/files", "tenant-files", "/private/tmp/avmc-platform-files", true)
	if err != nil {
		return nil, err
	}
	if _, err = ensureStorageProvider(ctx, client, "mock-s3-compatible", "S3 兼容存储示例", "s3-compatible", "http://127.0.0.1:9000", "us-east-1", "mock-access-key", "mock-secret-key", "", false, true, "http://127.0.0.1:9000", "tenant-files", "", false); err != nil {
		return nil, err
	}
	return local, nil
}

func ensureStorageProvider(ctx context.Context, client *gen.Client, code, name, providerType, endpoint, region, accessKey, secretKey, sessionToken string, useSSL, forcePathStyle bool, publicBaseURL, defaultBucket, localBasePath string, isDefault bool) (*gen.StorageProvider, error) {
	if isDefault {
		if _, err := client.StorageProvider.Update().SetIsDefault(false).Save(ctx); err != nil {
			return nil, err
		}
	}
	item, err := client.StorageProvider.Query().Where(storageprovider.CodeEQ(code)).Only(ctx)
	if gen.IsNotFound(err) {
		return client.StorageProvider.Create().
			SetCode(code).
			SetName(name).
			SetType(providerType).
			SetEndpoint(endpoint).
			SetRegion(region).
			SetAccessKey(accessKey).
			SetSecretKey(secretKey).
			SetSessionToken(sessionToken).
			SetUseSsl(useSSL).
			SetForcePathStyle(forcePathStyle).
			SetPublicBaseURL(publicBaseURL).
			SetDefaultBucket(defaultBucket).
			SetLocalBasePath(localBasePath).
			SetStatus(1).
			SetIsDefault(isDefault).
			SetHealthStatus("healthy").
			SetLastCheckedAt(time.Now()).
			SetRemark("mock data").
			Save(ctx)
	}
	if err != nil {
		return nil, err
	}
	return item.Update().
		SetName(name).
		SetType(providerType).
		SetEndpoint(endpoint).
		SetRegion(region).
		SetAccessKey(accessKey).
		SetSecretKey(secretKey).
		SetSessionToken(sessionToken).
		SetUseSsl(useSSL).
		SetForcePathStyle(forcePathStyle).
		SetPublicBaseURL(publicBaseURL).
		SetDefaultBucket(defaultBucket).
		SetLocalBasePath(localBasePath).
		SetStatus(1).
		SetIsDefault(isDefault).
		SetHealthStatus("healthy").
		SetLastCheckedAt(time.Now()).
		SetRemark("mock data").
		Save(ctx)
}

func seedFileObjects(ctx context.Context, client *gen.Client, provider *gen.StorageProvider, tenantOneOperator, tenantTwoOperator uint32) error {
	type fileObjectSpec struct {
		tenantID     uint32
		fileName     string
		contentType  string
		size         int64
		sha256       string
		businessType string
		businessID   string
		visibility   string
		status       int32
		objectKey    string
		createdBy    uint32
	}
	specs := []fileObjectSpec{
		{
			tenantID: 1, fileName: "mock_contract_2026.pdf", contentType: "application/pdf", size: 842144,
			sha256: strings.Repeat("a", 64), businessType: "contract", businessID: "contract-2026-001", visibility: "private", status: 2,
			objectKey: "tenants/1/mock/contracts/mock_contract_2026.pdf", createdBy: tenantOneOperator,
		},
		{
			tenantID: 1, fileName: "mock_product_import.xlsx", contentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", size: 231424,
			sha256: strings.Repeat("b", 64), businessType: "import", businessID: "import-product-001", visibility: "private", status: 2,
			objectKey: "tenants/1/mock/imports/mock_product_import.xlsx", createdBy: tenantOneOperator,
		},
		{
			tenantID: 1, fileName: "mock_public_logo.png", contentType: "image/png", size: 73412,
			sha256: strings.Repeat("c", 64), businessType: "branding", businessID: "brand-logo", visibility: "public", status: 2,
			objectKey: "tenants/1/mock/public/mock_public_logo.png", createdBy: tenantOneOperator,
		},
		{
			tenantID: 1, fileName: "mock_pending_upload.csv", contentType: "text/csv", size: 0,
			sha256: "", businessType: "import", businessID: "pending-import-001", visibility: "private", status: 1,
			objectKey: "tenants/1/mock/uploads/mock_pending_upload.csv", createdBy: tenantOneOperator,
		},
		{
			tenantID: 2, fileName: "mock_tenant2_invoice.pdf", contentType: "application/pdf", size: 512880,
			sha256: strings.Repeat("d", 64), businessType: "invoice", businessID: "invoice-t2-001", visibility: "private", status: 2,
			objectKey: "tenants/2/mock/invoices/mock_tenant2_invoice.pdf", createdBy: tenantTwoOperator,
		},
		{
			tenantID: 2, fileName: "mock_tenant2_archive.zip", contentType: "application/zip", size: 2748779,
			sha256: strings.Repeat("e", 64), businessType: "archive", businessID: "archive-t2-001", visibility: "private", status: 2,
			objectKey: "tenants/2/mock/archive/mock_tenant2_archive.zip", createdBy: tenantTwoOperator,
		},
	}
	now := time.Now()
	for _, spec := range specs {
		if err := ensureFileObject(ctx, client, provider, spec.tenantID, spec.fileName, spec.contentType, spec.size, spec.sha256, spec.businessType, spec.businessID, spec.visibility, spec.status, spec.objectKey, spec.createdBy, now); err != nil {
			return err
		}
	}
	return nil
}

func ensureFileObject(ctx context.Context, client *gen.Client, provider *gen.StorageProvider, tenantID uint32, fileName, contentType string, size int64, sha256, businessType, businessID, visibility string, status int32, objectKey string, createdBy uint32, now time.Time) error {
	providerID := uint32(0)
	providerCode := "mock-local-default"
	providerType := "local"
	if provider != nil {
		providerID = provider.ID
		providerCode = provider.Code
		providerType = provider.Type
	}
	item, err := client.FileObject.Query().
		Where(fileobject.TenantIDEQ(tenantID), fileobject.ObjectKeyEQ(objectKey)).
		Only(ctx)
	expiresAt := now.Add(24 * time.Hour)
	confirmedAt := now.Add(-time.Duration(tenantID) * time.Hour)
	if gen.IsNotFound(err) {
		builder := client.FileObject.Create().
			SetTenantID(tenantID).
			SetFileName(fileName).
			SetContentType(contentType).
			SetSize(size).
			SetSha256(sha256).
			SetEtag(fmt.Sprintf("mock-etag-%d-%s", tenantID, strings.TrimPrefix(fileName, "mock_"))).
			SetProvider(providerType).
			SetProviderID(providerID).
			SetProviderCode(providerCode).
			SetBucket("tenant-files").
			SetObjectKey(objectKey).
			SetBusinessType(businessType).
			SetBusinessID(businessID).
			SetVisibility(visibility).
			SetStatus(status).
			SetUploadExpiresAt(expiresAt).
			SetCreatedBy(createdBy)
		if status == 2 {
			builder.SetConfirmedAt(confirmedAt)
		}
		_, err = builder.Save(ctx)
		return err
	}
	if err != nil {
		return err
	}
	update := item.Update().
		SetFileName(fileName).
		SetContentType(contentType).
		SetSize(size).
		SetSha256(sha256).
		SetEtag(fmt.Sprintf("mock-etag-%d-%s", tenantID, strings.TrimPrefix(fileName, "mock_"))).
		SetProvider(providerType).
		SetProviderID(providerID).
		SetProviderCode(providerCode).
		SetBucket("tenant-files").
		SetBusinessType(businessType).
		SetBusinessID(businessID).
		SetVisibility(visibility).
		SetStatus(status).
		SetUploadExpiresAt(expiresAt).
		SetCreatedBy(createdBy)
	if status == 2 {
		update.SetConfirmedAt(confirmedAt)
	} else {
		update.ClearConfirmedAt()
	}
	_, err = update.Save(ctx)
	return err
}

func seedFileAccessLogs(ctx context.Context, client *gen.Client, tenantOneOperator, tenantTwoOperator uint32) error {
	files, err := client.FileObject.Query().
		Where(fileobject.FileNameHasPrefix("mock_")).
		Order(gen.Asc(fileobject.FieldTenantID), gen.Asc(fileobject.FieldID)).
		All(ctx)
	if err != nil {
		return err
	}
	for _, file := range files {
		operatorID := tenantOneOperator
		operatorName := "admin"
		if file.TenantID == 2 {
			operatorID = tenantTwoOperator
			operatorName = "mock_tenant2"
		}
		if err := ensureFileAccessLog(ctx, client, file, operatorID, operatorName, "download", "success", "mock download"); err != nil {
			return err
		}
		if file.Status != nil && *file.Status == 2 {
			if err := ensureFileAccessLog(ctx, client, file, operatorID, operatorName, "preview", "success", "mock preview"); err != nil {
				return err
			}
		}
	}
	return nil
}

func ensureFileAccessLog(ctx context.Context, client *gen.Client, file *gen.FileObject, operatorID uint32, operatorName string, action string, result string, message string) error {
	if file == nil {
		return nil
	}
	existing, err := client.FileAccessLog.Query().
		Where(
			fileaccesslog.TenantIDEQ(file.TenantID),
			fileaccesslog.FileIDEQ(file.ID),
			fileaccesslog.ActionEQ(action),
			fileaccesslog.MessageEQ(message),
		).
		Only(ctx)
	if gen.IsNotFound(err) {
		_, err = client.FileAccessLog.Create().
			SetTenantID(file.TenantID).
			SetFileID(file.ID).
			SetFileName(file.FileName).
			SetAction(action).
			SetOperatorID(operatorID).
			SetOperatorName(operatorName).
			SetClientIP("127.0.0.1").
			SetUserAgent("mock-seed").
			SetResult(result).
			SetMessage(message).
			Save(ctx)
		return err
	}
	if err != nil {
		return err
	}
	_, err = existing.Update().
		SetFileName(file.FileName).
		SetOperatorID(operatorID).
		SetOperatorName(operatorName).
		SetClientIP("127.0.0.1").
		SetUserAgent("mock-seed").
		SetResult(result).
		SetMessage(message).
		Save(ctx)
	return err
}

func seedParameters(ctx context.Context, client *gen.Client, tenantOneOperator, tenantTwoOperator uint32) error {
	type parameterSpec struct {
		key               string
		name              string
		valueType         pbCore.ParameterValueType
		defaultValue      string
		tenantOverridable bool
		description       string
	}
	specs := []parameterSpec{
		{"system.page_size", "默认分页大小", pbCore.ParameterValueType_PARAMETER_VALUE_TYPE_INTEGER, "20", true, "后台列表默认分页大小"},
		{"system.locale", "默认语言", pbCore.ParameterValueType_PARAMETER_VALUE_TYPE_STRING, "zh-CN", true, "租户默认语言"},
		{"feature.notification_enabled", "通知功能开关", pbCore.ParameterValueType_PARAMETER_VALUE_TYPE_BOOLEAN, "true", true, "是否启用通知能力"},
		{"upload.max_file_size_mb", "文件上传上限", pbCore.ParameterValueType_PARAMETER_VALUE_TYPE_INTEGER, "20", true, "单文件上传大小上限（MB）"},
		{"security.session_idle_minutes", "会话空闲时间", pbCore.ParameterValueType_PARAMETER_VALUE_TYPE_INTEGER, "30", false, "平台统一会话安全参数"},
		{"ui.date_format", "日期格式", pbCore.ParameterValueType_PARAMETER_VALUE_TYPE_STRING, "YYYY-MM-DD", true, "租户界面日期显示格式"},
		{"workflow.options", "工作流通用选项", pbCore.ParameterValueType_PARAMETER_VALUE_TYPE_JSON, `{"autoSave":true,"maxSteps":20}`, true, "通用工作流运行参数"},
		{"export.max_rows", "导出最大行数", pbCore.ParameterValueType_PARAMETER_VALUE_TYPE_INTEGER, "10000", true, "单次导出最大数据行数"},
	}
	definitions := make(map[string]*gen.ParameterDefinition, len(specs))
	for i, spec := range specs {
		item, err := client.ParameterDefinition.Query().
			Where(parameterdefinition.KeyEQ(spec.key)).
			Only(ctx)
		if gen.IsNotFound(err) {
			item, err = client.ParameterDefinition.Create().
				SetKey(spec.key).
				SetName(spec.name).
				SetValueType(int32(spec.valueType)).
				SetDefaultValue(spec.defaultValue).
				SetTenantOverridable(spec.tenantOverridable).
				SetDescription(spec.description).
				SetStatus(1).
				SetSort(int32((i + 1) * 10)).
				Save(ctx)
		} else if err == nil {
			item, err = item.Update().
				SetName(spec.name).
				SetValueType(int32(spec.valueType)).
				SetDefaultValue(spec.defaultValue).
				SetTenantOverridable(spec.tenantOverridable).
				SetDescription(spec.description).
				SetStatus(1).
				SetSort(int32((i + 1) * 10)).
				Save(ctx)
		}
		if err != nil {
			return err
		}
		definitions[spec.key] = item
	}
	overrides := []struct {
		tenantID uint32
		operator uint32
		key      string
		value    string
	}{
		{1, tenantOneOperator, "system.page_size", "50"},
		{1, tenantOneOperator, "export.max_rows", "25000"},
		{2, tenantTwoOperator, "system.page_size", "30"},
		{2, tenantTwoOperator, "system.locale", "en-US"},
		{2, tenantTwoOperator, "feature.notification_enabled", "false"},
	}
	for _, override := range overrides {
		definition := definitions[override.key]
		item, err := client.TenantParameterOverride.Query().
			Where(
				tenantparameteroverride.TenantIDEQ(override.tenantID),
				tenantparameteroverride.DefinitionIDEQ(definition.ID),
			).
			Only(ctx)
		if gen.IsNotFound(err) {
			_, err = client.TenantParameterOverride.Create().
				SetTenantID(override.tenantID).
				SetDefinitionID(definition.ID).
				SetValue(override.value).
				SetUpdatedBy(override.operator).
				Save(ctx)
		} else if err == nil {
			_, err = item.Update().
				SetValue(override.value).
				SetUpdatedBy(override.operator).
				Save(ctx)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func seedNotifications(ctx context.Context, client *gen.Client, tenantOneAdmin, tenantOneUser, tenantTwoUser uint32) error {
	templates := []struct {
		tenantID uint32
		code     string
		name     string
		title    string
		content  string
		remark   string
	}{
		{1, "system.welcome", "系统欢迎通知", "欢迎使用 {{tenantName}}", "你好 {{userName}}，你的租户工作台已完成初始化。", "租户开通后发送给管理员"},
		{1, "quota.warning", "资源额度预警", "{{resourceName}} 使用量预警", "{{resourceName}} 当前使用量已达到 {{usagePercent}}，请及时扩容或清理。", "资源额度接近上限时发送"},
		{2, "system.welcome", "租户二欢迎通知", "欢迎使用租户二工作台", "你的租户二演示环境已准备完成。", "用于验证跨租户隔离"},
	}
	for _, tpl := range templates {
		item, err := client.NotificationTemplate.Query().
			Where(notificationtemplate.TenantIDEQ(tpl.tenantID), notificationtemplate.CodeEQ(tpl.code)).
			Only(ctx)
		if gen.IsNotFound(err) {
			_, err = client.NotificationTemplate.Create().
				SetTenantID(tpl.tenantID).
				SetCode(tpl.code).
				SetName(tpl.name).
				SetChannel(int32(pbCore.NotificationChannel_NOTIFICATION_CHANNEL_IN_APP)).
				SetTitle(tpl.title).
				SetContent(tpl.content).
				SetVariableSchema(`{"type":"object"}`).
				SetLocale("zh-CN").
				SetStatus(1).
				SetRemark(tpl.remark).
				Save(ctx)
		} else if err == nil {
			_, err = item.Update().
				SetName(tpl.name).
				SetChannel(int32(pbCore.NotificationChannel_NOTIFICATION_CHANNEL_IN_APP)).
				SetTitle(tpl.title).
				SetContent(tpl.content).
				SetVariableSchema(`{"type":"object"}`).
				SetLocale("zh-CN").
				SetStatus(1).
				SetRemark(tpl.remark).
				Save(ctx)
		}
		if err != nil {
			return err
		}
	}

	messages := []struct {
		tenantID    uint32
		recipientID uint32
		title       string
		content     string
		status      pbCore.NotificationMessageStatus
		business    string
	}{
		{1, tenantOneAdmin, "租户开通完成", "演示租户已完成初始化，套餐和基础角色已绑定。", pbCore.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_UNREAD, "tenant"},
		{1, tenantOneAdmin, "文件中心额度提醒", "当前文件中心已启用 files 和 storage.bytes 额度闭环。", pbCore.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_UNREAD, "quota"},
		{1, tenantOneUser, "待处理任务", "你有一条模拟异步任务需要关注。", pbCore.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_READ, "async_task"},
		{2, tenantTwoUser, "租户二初始化完成", "租户二基础演示数据已准备完成。", pbCore.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_UNREAD, "tenant"},
	}
	for _, msg := range messages {
		item, err := client.NotificationMessage.Query().
			Where(
				notificationmessage.TenantIDEQ(msg.tenantID),
				notificationmessage.RecipientUserIDEQ(msg.recipientID),
				notificationmessage.TitleEQ(msg.title),
			).
			Only(ctx)
		if gen.IsNotFound(err) {
			builder := client.NotificationMessage.Create().
				SetTenantID(msg.tenantID).
				SetRecipientUserID(msg.recipientID).
				SetChannel(int32(pbCore.NotificationChannel_NOTIFICATION_CHANNEL_IN_APP)).
				SetTitle(msg.title).
				SetContent(msg.content).
				SetMessageStatus(int32(msg.status)).
				SetPriority(0).
				SetBusinessType(msg.business).
				SetBusinessID("mock").
				SetSenderUserID(tenantOneAdmin).
				SetSenderName("admin")
			if msg.status == pbCore.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_READ {
				builder.SetReadAt(time.Now().Add(-time.Hour))
			}
			_, err = builder.Save(ctx)
		} else if err == nil {
			update := item.Update().
				SetChannel(int32(pbCore.NotificationChannel_NOTIFICATION_CHANNEL_IN_APP)).
				SetContent(msg.content).
				SetMessageStatus(int32(msg.status)).
				SetBusinessType(msg.business).
				SetBusinessID("mock").
				SetSenderUserID(tenantOneAdmin).
				SetSenderName("admin")
			if msg.status == pbCore.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_READ {
				update.SetReadAt(time.Now().Add(-time.Hour))
			} else {
				update.ClearReadAt()
			}
			_, err = update.Save(ctx)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func seedAsyncTasks(ctx context.Context, client *gen.Client, operatorID uint32) error {
	type taskSpec struct {
		key            string
		tenantID       *uint32
		taskType       string
		queue          string
		payload        string
		payloadSummary string
		status         pbCore.AsyncTaskStatus
		attempts       int32
		scheduledAt    time.Time
		completedAt    *time.Time
		resultSummary  string
		lastError      string
	}
	now := time.Now()
	tenantOne := uint32(1)
	completed := now.Add(-2 * time.Hour)
	canceled := now.Add(-time.Hour)
	specs := []taskSpec{
		{
			key: "mock:async-task:succeeded", status: pbCore.AsyncTaskStatus_ASYNC_TASK_STATUS_SUCCEEDED,
			attempts: 1, scheduledAt: now.Add(-3 * time.Hour), completedAt: &completed,
			resultSummary: "已清理 0 条过期终态任务",
		},
		{
			key: "mock:async-task:failed", tenantID: &tenantOne, status: pbCore.AsyncTaskStatus_ASYNC_TASK_STATUS_FAILED,
			attempts: 3, scheduledAt: now.Add(-4 * time.Hour), completedAt: &completed,
			lastError: "模拟下游服务暂时不可用",
		},
		{
			key: "mock:async-task:pending", tenantID: &tenantOne, status: pbCore.AsyncTaskStatus_ASYNC_TASK_STATUS_PENDING,
			scheduledAt: now.Add(24 * time.Hour),
		},
		{
			key: "mock:async-task:canceled", status: pbCore.AsyncTaskStatus_ASYNC_TASK_STATUS_CANCELED,
			scheduledAt: now.Add(-2 * time.Hour), completedAt: &canceled,
		},
		{
			key: "mock:notification:in-app:welcome", tenantID: &tenantOne,
			taskType: "notification.in_app.send", queue: "notification",
			payload:        `{"tenantId":1,"recipientUserIds":[1],"templateCode":"system.welcome","variables":"{\"tenantName\":\"演示租户\",\"userName\":\"admin\"}","businessType":"tenant","businessId":"mock"}`,
			payloadSummary: "站内信接收人 1 个",
			status:         pbCore.AsyncTaskStatus_ASYNC_TASK_STATUS_SUCCEEDED,
			attempts:       1, scheduledAt: now.Add(-90 * time.Minute), completedAt: &completed,
			resultSummary: "已生成 1 条站内信",
		},
	}
	for _, spec := range specs {
		if spec.taskType == "" {
			spec.taskType = "system.task_retention_cleanup"
		}
		if spec.queue == "" {
			spec.queue = "maintenance"
		}
		if spec.payload == "" {
			spec.payload = `{"retentionDays":30,"batchSize":500}`
		}
		if spec.payloadSummary == "" {
			spec.payloadSummary = "清理 30 天前的终态任务，单批最多 500 条"
		}
		item, err := client.AsyncTask.Query().
			Where(asynctask.IdempotencyKeyEQ(spec.key)).
			Only(ctx)
		if gen.IsNotFound(err) {
			builder := client.AsyncTask.Create().
				SetTaskType(spec.taskType).
				SetQueue(spec.queue).
				SetStatus(int32(spec.status)).
				SetPriority(-10).
				SetPayload(spec.payload).
				SetPayloadSummary(spec.payloadSummary).
				SetIdempotencyKey(spec.key).
				SetAttempts(spec.attempts).
				SetMaxAttempts(3).
				SetScheduledAt(spec.scheduledAt).
				SetCreatedBy(operatorID).
				SetResultSummary(spec.resultSummary).
				SetLastError(spec.lastError)
			if spec.tenantID != nil {
				builder.SetTenantID(*spec.tenantID)
			}
			if spec.completedAt != nil {
				builder.SetCompletedAt(*spec.completedAt)
			}
			_, err = builder.Save(ctx)
		} else if err == nil {
			update := item.Update().
				SetTaskType(spec.taskType).
				SetQueue(spec.queue).
				SetStatus(int32(spec.status)).
				SetPayload(spec.payload).
				SetPayloadSummary(spec.payloadSummary).
				SetAttempts(spec.attempts).
				SetScheduledAt(spec.scheduledAt).
				SetResultSummary(spec.resultSummary).
				SetLastError(spec.lastError).
				SetLeaseOwner("").
				ClearLeaseExpiresAt()
			if spec.completedAt != nil {
				update.SetCompletedAt(*spec.completedAt)
			} else {
				update.ClearCompletedAt()
			}
			_, err = update.Save(ctx)
		}
		if err != nil {
			return err
		}
	}
	return nil
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
				SetUserAgent("Platform Mock Client/1.0").
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
		{"tenant admin roles", func(ctx context.Context) (int, error) {
			return client.Role.Query().Where(role.IsTenantAdminEQ(true)).Count(ctx)
		}, 2},
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
		{"parameter definitions", func(ctx context.Context) (int, error) {
			return client.ParameterDefinition.Query().Where(parameterdefinition.DeletedAtIsNil()).Count(ctx)
		}, 8},
		{"parameter overrides", func(ctx context.Context) (int, error) {
			return client.TenantParameterOverride.Query().Count(ctx)
		}, 5},
		{"async tasks", func(ctx context.Context) (int, error) {
			return client.AsyncTask.Query().Count(ctx)
		}, 5},
		{"tenant 1 notification templates", func(ctx context.Context) (int, error) {
			return client.NotificationTemplate.Query().Where(notificationtemplate.TenantIDEQ(1)).Count(ctx)
		}, 2},
		{"tenant 1 notification messages", func(ctx context.Context) (int, error) {
			return client.NotificationMessage.Query().Where(notificationmessage.TenantIDEQ(1)).Count(ctx)
		}, 3},
		{"tenant 1 unread notification messages", func(ctx context.Context) (int, error) {
			return client.NotificationMessage.Query().Where(
				notificationmessage.TenantIDEQ(1),
				notificationmessage.MessageStatusEQ(int32(pbCore.NotificationMessageStatus_NOTIFICATION_MESSAGE_STATUS_UNREAD)),
			).Count(ctx)
		}, 2},
		{"tenant 2 notification messages", func(ctx context.Context) (int, error) {
			return client.NotificationMessage.Query().Where(notificationmessage.TenantIDEQ(2)).Count(ctx)
		}, 1},
		{"tenant 1 file objects", func(ctx context.Context) (int, error) {
			return client.FileObject.Query().Where(fileobject.TenantIDEQ(1), fileobject.FileNameHasPrefix("mock_")).Count(ctx)
		}, 4},
		{"tenant 2 file objects", func(ctx context.Context) (int, error) {
			return client.FileObject.Query().Where(fileobject.TenantIDEQ(2), fileobject.FileNameHasPrefix("mock_")).Count(ctx)
		}, 2},
		{"tenant 1 file access logs", func(ctx context.Context) (int, error) {
			return client.FileAccessLog.Query().Where(fileaccesslog.TenantIDEQ(1)).Count(ctx)
		}, 4},
		{"tenant 2 file access logs", func(ctx context.Context) (int, error) {
			return client.FileAccessLog.Query().Where(fileaccesslog.TenantIDEQ(2)).Count(ctx)
		}, 2},
		{"storage providers", func(ctx context.Context) (int, error) {
			return client.StorageProvider.Query().Where(storageprovider.CodeIn("mock-local-default", "mock-s3-compatible")).Count(ctx)
		}, 2},
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
	platformTenants, err := client.Tenant.Query().
		Where(tenant.IsPlatformEQ(true)).
		IDs(ctx)
	if err != nil {
		return fmt.Errorf("checking platform tenant marker: %w", err)
	}
	if len(platformTenants) != 1 || platformTenants[0] != 1 {
		return fmt.Errorf("platform tenants=%v, want [1]", platformTenants)
	}
	fmt.Printf("verified %-22s ids=%v\n", "platform tenant", platformTenants)
	return nil
}
