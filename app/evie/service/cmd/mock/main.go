//go:build mock

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"backend-service/app/evie/service/internal/data"
	"backend-service/app/evie/service/internal/data/ent/gen"
	"backend-service/app/evie/service/internal/data/ent/gen/dept"
	"backend-service/app/evie/service/internal/data/ent/gen/menu"
	"backend-service/app/evie/service/internal/data/ent/gen/post"
	"backend-service/app/evie/service/internal/data/ent/gen/project"
	"backend-service/app/evie/service/internal/data/ent/gen/role"
	"backend-service/app/evie/service/internal/data/ent/gen/tenantmenupermissiongroup"
	"backend-service/app/evie/service/internal/data/ent/gen/user"
	entviewer "backend-service/app/evie/service/internal/data/ent/viewer"
	"backend-service/app/evie/service/internal/runtimeconfig"
	authzEngine "backend-service/pkg/auth/authz"
	authzCasbin "backend-service/pkg/auth/authz/casbin"
	"backend-service/pkg/utils/crypto"

	"github.com/go-kratos/kratos/v2/log"
	_ "github.com/go-sql-driver/mysql"
)

const mockPassword = "123456"

var flagconf = "../../configs"

type mockMenuSpec struct {
	Name, Title, Path, Component, Icon string
	Type, Sort                         int32
	AuthCode                           string
}

func main() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path")
	flag.Parse()
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "admin mock failed: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	ctx = entviewer.NewSystemContext(ctx)
	bc, err := runtimeconfig.Load(flagconf)
	if err != nil {
		return err
	}
	client, err := data.NewEntClient(bc.Data, log.DefaultLogger)
	if err != nil {
		return err
	}
	defer client.Close()

	authorizer, err := authzCasbin.NewProvider().NewAuthorizer(
		ctx,
		authzEngine.WithAdapterType(authzEngine.AdapterMySQL),
		authzEngine.WithAdapterDSN(bc.Data.Database.Source),
	)
	if err != nil {
		return err
	}
	if err := seed(ctx, client, authorizer); err != nil {
		return err
	}
	if err := verify(ctx, client); err != nil {
		return err
	}
	fmt.Printf("admin mock data ready: users=[admin,vben,jack,tenant2] password=%s\n", mockPassword)
	return nil
}

func seed(ctx context.Context, client *gen.Client, authorizer authzEngine.Authorizer) error {
	hash, err := crypto.HashPassword(mockPassword)
	if err != nil {
		return err
	}

	admin := ensureUser(ctx, client, 1, "admin", hash, "admin@example.com")
	vben := ensureUser(ctx, client, 1, "vben", hash, "vben@example.com")
	jack := ensureUser(ctx, client, 1, "jack", hash, "jack@example.com")
	operator := ensureUser(ctx, client, 1, "operator", hash, "operator@example.com")
	t2u := ensureUser(ctx, client, 2, "tenant2", hash, "tenant2@example.com")

	// ── Menu Hierarchy ─────────────────────────────────
	menuMap := make(map[string]uint32)
	mi := func(spec mockMenuSpec, parentID uint32) *gen.Menu {
		m := menuItem(ctx, client, spec, parentID)
		menuMap[spec.Name] = m.ID
		return m
	}
	md := func(spec mockMenuSpec, parentID uint32) *gen.Menu {
		m := menuDir(ctx, client, spec, parentID)
		menuMap[spec.Name] = m.ID
		return m
	}

	dash := md(mockMenuSpec{"Dashboard", "仪表盘", "/dashboard", "BasicLayout", "ion:grid-outline", 1, 10, ""}, 0)
	mi(mockMenuSpec{"DashboardWorkbench", "工作台", "/dashboard/workbench", "/dashboard/index", "mdi:monitor-dashboard", 2, 10, ""}, dash.ID)

	tn := md(mockMenuSpec{"TenantManagement", "租户管理", "/tenant", "BasicLayout", "mdi:office-building-cog-outline", 1, 20, ""}, 0)
	mi(mockMenuSpec{"TenantList", "租户列表", "/tenant/list", "/system/tenant/list", "mdi:office-building-outline", 2, 10, ""}, tn.ID)
	mi(mockMenuSpec{"TenantPackage", "租户套餐配置", "/tenant/package", "/system/tenant-menu-permission-group/list", "mdi:package-variant-closed", 2, 20, ""}, tn.ID)

	org := md(mockMenuSpec{"Organization", "组织架构", "/org", "BasicLayout", "ion:people-outline", 1, 30, ""}, 0)
	mi(mockMenuSpec{"UserManagement", "用户与部门", "/org/user", "/system/user/list", "mdi:account-multiple-outline", 2, 10, ""}, org.ID)
	deptLegacy := mi(mockMenuSpec{"DeptManagement", "部门管理", "/org/dept", "/system/user/list", "charm:organisation", 2, 20, ""}, org.ID)
	client.Menu.UpdateOneID(deptLegacy.ID).SetHideInMenu(true).SetRedirect("/org/user").Exec(ctx)
	mi(mockMenuSpec{"PostManagement", "岗位管理", "/org/post", "/system/post/list", "mdi:badge-account-outline", 2, 30, ""}, org.ID)
	mi(mockMenuSpec{"RoleManagement", "角色管理", "/org/role", "/system/role/list", "mdi:shield-account-outline", 2, 40, ""}, org.ID)
	mi(mockMenuSpec{"ProjectManagement", "项目管理", "/org/project", "/system/project/list", "mdi:folder-cog-outline", 2, 50, ""}, org.ID)

	perm := md(mockMenuSpec{"PermissionSecurity", "权限与安全", "/perm", "BasicLayout", "ion:shield-checkmark-outline", 1, 40, ""}, 0)
	mi(mockMenuSpec{"MenuManagement", "菜单管理", "/perm/menu", "/system/menu/list", "mdi:menu", 2, 10, ""}, perm.ID)

	file := md(mockMenuSpec{"FileCenter", "文件中心", "/file", "BasicLayout", "ion:folder-open-outline", 1, 50, ""}, 0)
	mi(mockMenuSpec{"FileList", "文件列表", "/file/list", "/system/file-center/list", "mdi:file-document-outline", 2, 10, ""}, file.ID)
	mi(mockMenuSpec{"StorageProvider", "存储渠道", "/file/storage", "/system/storage-provider/list", "mdi:server-network-outline", 2, 20, ""}, file.ID)

	notif := md(mockMenuSpec{"NotificationCenter", "通知中心", "/notif", "BasicLayout", "ion:notifications-outline", 1, 60, ""}, 0)
	mi(mockMenuSpec{"NotifTemplate", "通知模板", "/notif/template", "/system/notification/template", "mdi:email-outline", 2, 10, ""}, notif.ID)
	mi(mockMenuSpec{"NotifRecord", "通知记录", "/notif/record", "/system/notification/record", "mdi:bell-outline", 2, 20, ""}, notif.ID)

	sys := md(mockMenuSpec{"SystemManagement", "系统管理", "/system", "BasicLayout", "ion:settings-outline", 1, 999, ""}, 0)
	mi(mockMenuSpec{"Dictionary", "字典管理", "/system/dictionary", "/system/dictionary/list", "mdi:book-cog-outline", 2, 10, ""}, sys.ID)
	mi(mockMenuSpec{"Parameter", "参数配置", "/system/parameter", "/system/parameter/list", "mdi:tune-variant", 2, 20, ""}, sys.ID)
	mi(mockMenuSpec{"OperationLog", "操作审计", "/system/operation-log", "/system/operation-log/list", "mdi:clipboard-text-clock-outline", 2, 30, ""}, sys.ID)
	mi(mockMenuSpec{"LoginLog", "登录日志", "/system/login-log", "/system/login-log/list", "mdi:login-variant", 2, 40, ""}, sys.ID)
	mi(mockMenuSpec{"Session", "会话管理", "/system/session", "/system/session/list", "mdi:account-clock-outline", 2, 50, ""}, sys.ID)
	mi(mockMenuSpec{"AsyncTask", "异步任务", "/system/async-task", "/system/async-task/list", "mdi:progress-clock", 2, 60, ""}, sys.ID)
	mi(mockMenuSpec{"Webhook", "Webhook管理", "/system/webhook", "/system/webhook/list", "mdi:webhook", 2, 70, ""}, sys.ID)

	// ── Permission Buttons ─────────────────────────────
	bt := func(parent, name, title, code string, s int32) {
		menuBtn(ctx, client, mockMenuSpec{Name: name, Title: title, AuthCode: "v1." + code, Type: 3, Sort: s, Path: "", Component: "", Icon: ""}, menuMap[parent])
	}
	btOperation := func(parent, name, title, operation string, s int32) {
		menuBtn(ctx, client, mockMenuSpec{Name: name, Title: title, AuthCode: operation, Type: 3, Sort: s, Path: "", Component: "", Icon: ""}, menuMap[parent])
		client.Menu.Update().Where(menu.NameEQ(name)).SetAuthCode(operation).Exec(ctx)
	}
	// 租户管理
	bt("TenantList", "TenantListQuery", "查询", "OperationTenantServiceListTenants", 10)
	bt("TenantList", "TenantCreate", "新增", "OperationTenantServiceCreateTenant", 20)
	bt("TenantList", "TenantEdit", "编辑", "OperationTenantServiceUpdateTenant", 30)
	bt("TenantList", "TenantDelete", "删除", "OperationTenantServiceDeleteTenant", 40)
	bt("TenantList", "TenantLifecycle", "生命周期", "OperationTenantServiceUpdateTenantLifecycle", 50)
	btOperation("TenantList", "TenantAdminQuery", "管理员查询", "/platform.admin.v1.TenantService/ListTenantAdmins", 60)
	btOperation("TenantList", "TenantAdminEdit", "管理员资料", "/platform.admin.v1.TenantService/UpdateTenantAdmin", 70)
	btOperation("TenantList", "TenantAdminPassword", "管理员密码重置", "/platform.admin.v1.TenantService/ResetTenantAdminPassword", 80)
	bt("TenantPackage", "TPQuery", "查询", "OperationTenantMenuPermissionGroupServiceListTenantMenuPermissionGroups", 10)
	bt("TenantPackage", "TPCreate", "新增", "OperationTenantMenuPermissionGroupServiceCreateTenantMenuPermissionGroup", 20)
	bt("TenantPackage", "TPEdit", "编辑", "OperationTenantMenuPermissionGroupServiceUpdateTenantMenuPermissionGroup", 30)
	bt("TenantPackage", "TPDelete", "删除", "OperationTenantMenuPermissionGroupServiceDeleteTenantMenuPermissionGroup", 40)
	bt("TenantPackage", "TPStatus", "启停", "OperationTenantMenuPermissionGroupServiceUpdateTenantMenuPermissionGroupStatus", 50)
	bt("TenantPackage", "TPVersions", "版本列表", "OperationTenantMenuPermissionGroupServiceListTenantMenuPermissionGroupVersions", 60)
	bt("TenantPackage", "TPPublish", "发布版本", "OperationTenantMenuPermissionGroupServicePublishTenantMenuPermissionGroupVersion", 70)
	bt("TenantPackage", "TPRollback", "回滚版本", "OperationTenantMenuPermissionGroupServiceRollbackTenantMenuPermissionGroupVersion", 80)
	// 组织架构
	bt("UserManagement", "UserQuery", "查询", "OperationUserServiceListUsers", 10)
	bt("UserManagement", "UserCreate", "新增", "OperationUserServiceCreateUser", 20)
	bt("UserManagement", "UserEdit", "编辑", "OperationUserServiceUpdateUser", 30)
	bt("UserManagement", "UserDelete", "删除", "OperationUserServiceDeleteUser", 40)
	bt("UserManagement", "UserStatus", "启停", "OperationUserServiceUpdateUserByStatus", 50)
	bt("UserManagement", "DeptQuery", "部门查询", "OperationDeptServiceListDepts", 60)
	bt("UserManagement", "DeptCreate", "部门新增", "OperationDeptServiceCreateDept", 70)
	bt("UserManagement", "DeptEdit", "部门编辑", "OperationDeptServiceUpdateDept", 80)
	bt("UserManagement", "DeptDelete", "部门删除", "OperationDeptServiceDeleteDept", 90)
	bt("UserManagement", "DeptStatus", "部门启停", "OperationDeptServiceUpdateDeptByStatus", 100)
	btOperation("UserManagement", "DeptTreeQuery", "部门树查询", "/platform.admin.v1.DeptService/ListDeptsTree", 110)
	btOperation("UserManagement", "DeptDeleteImpact", "部门删除检查", "/platform.admin.v1.DeptService/GetDeptDeleteImpact", 120)
	btOperation("UserManagement", "DeptTransferDelete", "人员转移并删除部门", "/platform.admin.v1.DeptService/TransferAndDeleteDept", 130)
	bt("PostManagement", "PostQuery", "查询", "OperationPostServiceListPosts", 10)
	bt("PostManagement", "PostCreate", "新增", "OperationPostServiceCreatePost", 20)
	bt("PostManagement", "PostEdit", "编辑", "OperationPostServiceUpdatePost", 30)
	bt("PostManagement", "PostDelete", "删除", "OperationPostServiceDeletePost", 40)
	bt("PostManagement", "PostStatus", "启停", "OperationPostServiceUpdatePostByStatus", 50)
	bt("RoleManagement", "RoleQuery", "查询", "OperationRoleServiceListRoles", 10)
	bt("RoleManagement", "RoleCreate", "新增", "OperationRoleServiceCreateRole", 20)
	bt("RoleManagement", "RoleEdit", "编辑", "OperationRoleServiceUpdateRole", 30)
	bt("RoleManagement", "RoleDelete", "删除", "OperationRoleServiceDeleteRole", 40)
	bt("RoleManagement", "RoleStatus", "启停", "OperationRoleServiceUpdateRoleByStatus", 50)
	bt("ProjectManagement", "ProjectQuery", "查询", "OperationProjectServiceListProjects", 10)
	bt("ProjectManagement", "ProjectCreate", "新增", "OperationProjectServiceCreateProject", 20)
	bt("ProjectManagement", "ProjectEdit", "编辑", "OperationProjectServiceUpdateProject", 30)
	bt("ProjectManagement", "ProjectDelete", "删除", "OperationProjectServiceDeleteProject", 40)
	bt("ProjectManagement", "ProjectStatus", "启停", "OperationProjectServiceUpdateProjectByStatus", 50)
	// 权限与安全
	bt("MenuManagement", "MenuQuery", "查询", "OperationMenuServiceListMenus", 10)
	bt("MenuManagement", "MenuCreate", "新增", "OperationMenuServiceCreateMenu", 20)
	bt("MenuManagement", "MenuEdit", "编辑", "OperationMenuServiceUpdateMenu", 30)
	bt("MenuManagement", "MenuDelete", "删除", "OperationMenuServiceDeleteMenu", 40)
	bt("MenuManagement", "MenuStatus", "启停", "OperationMenuServiceUpdateMenuByStatus", 50)
	// 文件中心
	bt("FileList", "FileQuery", "查询", "OperationFileCenterServiceListFileObjects", 10)
	bt("FileList", "FileUpload", "上传", "OperationFileCenterServiceCreateFileUploadSession", 20)
	bt("FileList", "FileDelete", "删除", "OperationFileCenterServiceDeleteFileObject", 30)
	bt("FileList", "FileDownload", "下载", "OperationFileCenterServicePresignFileDownload", 40)
	bt("StorageProvider", "StorageQuery", "查询", "OperationStorageProviderServiceListStorageProviders", 10)
	bt("StorageProvider", "StorageCreate", "新增", "OperationStorageProviderServiceCreateStorageProvider", 20)
	bt("StorageProvider", "StorageEdit", "编辑", "OperationStorageProviderServiceUpdateStorageProvider", 30)
	bt("StorageProvider", "StorageDelete", "删除", "OperationStorageProviderServiceDeleteStorageProvider", 40)
	bt("StorageProvider", "StorageDefault", "设为默认", "OperationStorageProviderServiceSetDefaultStorageProvider", 50)
	// 通知中心
	bt("NotifTemplate", "NTQuery", "查询", "OperationNotificationServiceListNotificationTemplates", 10)
	bt("NotifTemplate", "NTCreate", "新增", "OperationNotificationServiceCreateNotificationTemplate", 20)
	bt("NotifTemplate", "NTEdit", "编辑", "OperationNotificationServiceUpdateNotificationTemplate", 30)
	bt("NotifTemplate", "NTDelete", "删除", "OperationNotificationServiceDeleteNotificationTemplate", 40)
	bt("NotifRecord", "NMsgQuery", "查询", "OperationNotificationServiceListNotificationMessages", 10)
	bt("NotifRecord", "NMsgSend", "发送", "OperationNotificationServiceSendInAppNotification", 20)
	// 系统管理
	bt("Dictionary", "DictQuery", "查询", "OperationDictionaryServiceListDictionaryTypes", 10)
	bt("Dictionary", "DictCreate", "新增", "OperationDictionaryServiceCreateDictionaryType", 20)
	bt("Dictionary", "DictEdit", "编辑", "OperationDictionaryServiceUpdateDictionaryType", 30)
	bt("Dictionary", "DictDelete", "删除", "OperationDictionaryServiceDeleteDictionaryType", 40)
	bt("Parameter", "ParamQuery", "查询", "OperationParameterServiceListParameterDefinitions", 10)
	bt("Parameter", "ParamCreate", "新增", "OperationParameterServiceCreateParameterDefinition", 20)
	bt("Parameter", "ParamEdit", "编辑", "OperationParameterServiceUpdateParameterDefinition", 30)
	bt("Parameter", "ParamDelete", "删除", "OperationParameterServiceDeleteParameterDefinition", 40)
	bt("OperationLog", "OLQuery", "查询", "OperationOperationLogServiceListOperationLogs", 10)
	bt("LoginLog", "LLQuery", "查询", "OperationLoginLogServiceListLoginLogs", 10)
	bt("Session", "SessionQuery", "查询", "OperationSessionServiceListSessions", 10)
	bt("Session", "SessionRevoke", "踢下线", "OperationSessionServiceRevokeSession", 20)
	bt("AsyncTask", "ATQuery", "查询", "OperationAsyncTaskServiceListAsyncTasks", 10)
	bt("AsyncTask", "ATCancel", "取消", "OperationAsyncTaskServiceCancelAsyncTask", 20)
	bt("AsyncTask", "ATRetry", "重试", "OperationAsyncTaskServiceRetryAsyncTask", 30)
	bt("Webhook", "WHQuery", "查询", "OperationWebhookServiceListWebhookSubscriptions", 10)
	bt("Webhook", "WHCreate", "新增", "OperationWebhookServiceCreateWebhookSubscription", 20)
	bt("Webhook", "WHEdit", "编辑", "OperationWebhookServiceUpdateWebhookSubscription", 30)
	bt("Webhook", "WHDelete", "删除", "OperationWebhookServiceDeleteWebhookSubscription", 40)
	bt("Webhook", "WHRetry", "重试", "OperationWebhookServiceRetryWebhookDelivery", 50)

	// ── Packages ───────────────────────────────────────
	allIDs, _ := client.Menu.Query().IDs(ctx)
	ensurePkg(ctx, client, "全功能管理员套餐", "full-admin", true, allIDs)

	basicPaths := []string{"/tenant", "/org", "/perm", "/file", "/notif", "/dashboard"}
	basicDirs, _ := client.Menu.Query().Where(menu.PathIn(basicPaths...)).IDs(ctx)
	basicSet := make(map[uint32]bool)
	for _, id := range basicDirs {
		basicSet[id] = true
	}
	allMenus, _ := client.Menu.Query().All(ctx)
	basicIDs := make([]uint32, 0)
	for _, m := range allMenus {
		if basicSet[*m.ParentID] || basicSet[m.ID] {
			basicIDs = append(basicIDs, m.ID)
		}
	}
	basicIDs = append(basicIDs, basicDirs...)
	ensurePkg(ctx, client, "基础业务套餐", "basic-business", true, basicIDs)

	// ── Roles ──────────────────────────────────────────
	sa := ensureRole(ctx, client, 1, "超级管理员", allIDs, true)
	normalRole := ensureRole(ctx, client, 1, "普通用户", basicIDs, false)
	ensureRole(ctx, client, 2, "租户管理员", basicIDs, true)
	client.User.UpdateOneID(admin.ID).AddRoleIDs(sa.ID).Exec(ctx)
	client.User.UpdateOneID(vben.ID).AddRoleIDs(sa.ID).Exec(ctx)
	client.User.UpdateOneID(jack.ID).AddRoleIDs(sa.ID).Exec(ctx)
	t2Role, _ := client.Role.Query().Where(role.TenantIDEQ(2), role.IsTenantAdminEQ(true)).Only(ctx)
	if t2Role != nil {
		client.User.UpdateOneID(t2u.ID).AddRoleIDs(t2Role.ID).Exec(ctx)
	}
	if err := data.SyncRolePolicies(ctx, client, authorizer, 1, sa.ID); err != nil {
		return err
	}
	if err := data.SyncRolePolicies(ctx, client, authorizer, 1, normalRole.ID); err != nil {
		return err
	}
	if t2Role != nil {
		if err := data.SyncRolePolicies(ctx, client, authorizer, 2, t2Role.ID); err != nil {
			return err
		}
	}

	// ── Departments ────────────────────────────────────
	t1d, _ := ensureDept(ctx, client, 1, "总公司", 0, admin.ID)
	t1tech, _ := ensureDept(ctx, client, 1, "技术部", t1d.ID, vben.ID)
	t2d, _ := ensureDept(ctx, client, 2, "客户企业", 0, t2u.ID)
	client.User.UpdateOneID(admin.ID).SetDeptID(t1d.ID).Exec(ctx)
	client.User.UpdateOneID(jack.ID).SetDeptID(t1d.ID).Exec(ctx)
	client.User.UpdateOneID(vben.ID).SetDeptID(t1tech.ID).Exec(ctx)
	client.User.UpdateOneID(operator.ID).SetDeptID(t1tech.ID).Exec(ctx)
	client.User.UpdateOneID(t2u.ID).SetDeptID(t2d.ID).Exec(ctx)

	// ── Posts / Projects ───────────────────────────────
	for _, nm := range []string{"技术总监", "运营经理", "开发工程师"} {
		ensurePost(ctx, client, 1, nm)
	}
	ensurePost(ctx, client, 2, "管理员")
	ensureProject(ctx, client, 1, "GEO内容工程", "geo-engine", admin.ID, []uint32{admin.ID, vben.ID})
	ensureProject(ctx, client, 2, "客户项目A", "customer-a", t2u.ID, []uint32{t2u.ID})

	return nil
}

// ── Helpers ─────────────────────────────────────────────

func menuDir(ctx context.Context, c *gen.Client, s mockMenuSpec, pid uint32) *gen.Menu {
	return menuX(ctx, c, s, pid, 1)
}
func menuItem(ctx context.Context, c *gen.Client, s mockMenuSpec, pid uint32) *gen.Menu {
	return menuX(ctx, c, s, pid, 2)
}
func menuX(ctx context.Context, c *gen.Client, s mockMenuSpec, pid uint32, typ int32) *gen.Menu {
	ex, err := c.Menu.Query().Where(menu.NameEQ(s.Name)).Only(ctx)
	if err != nil {
		return c.Menu.Create().SetName(s.Name).SetTitle(s.Title).SetPath(s.Path).
			SetComponent(s.Component).SetIcon(s.Icon).SetType(typ).SetSort(s.Sort).
			SetNillableAuthCode(&s.AuthCode).SetNillableParentID(&pid).SaveX(ctx)
	}
	c.Menu.UpdateOneID(ex.ID).SetTitle(s.Title).SetPath(s.Path).
		SetComponent(s.Component).SetIcon(s.Icon).SetType(typ).SetSort(s.Sort).
		SetNillableAuthCode(&s.AuthCode).SetNillableParentID(&pid).Exec(ctx)
	return ex
}

func menuBtn(ctx context.Context, c *gen.Client, s mockMenuSpec, pid uint32) {
	ex, err := c.Menu.Query().Where(menu.NameEQ(s.Name)).Only(ctx)
	if err != nil {
		c.Menu.Create().SetName(s.Name).SetTitle(s.Title).SetPath(s.Path).
			SetComponent(s.Component).SetIcon(s.Icon).SetType(3).SetSort(s.Sort).
			SetNillableAuthCode(&s.AuthCode).SetNillableParentID(&pid).SaveX(ctx)
		return
	}
	c.Menu.UpdateOneID(ex.ID).
		SetTitle(s.Title).
		SetSort(s.Sort).
		SetNillableAuthCode(&s.AuthCode).
		SetNillableParentID(&pid).
		Exec(ctx)
}

func ensurePkg(ctx context.Context, c *gen.Client, name, code string, isSystem bool, menuIDs []uint32) *gen.TenantMenuPermissionGroup {
	ex, err := c.TenantMenuPermissionGroup.Query().Where(tenantmenupermissiongroup.CodeEQ(code)).Only(ctx)
	if err != nil {
		pkg := c.TenantMenuPermissionGroup.Create().SetName(name).SetCode(code).SetNillableIsSystem(&isSystem).AddMenuIDs(menuIDs...).SaveX(ctx)
		ver := c.TenantMenuPermissionGroupVersion.Create().SetGroupID(pkg.ID).SetVersion(1).SetState(1).AddMenuIDs(menuIDs...).SaveX(ctx)
		c.TenantMenuPermissionGroup.UpdateOneID(pkg.ID).SetCurrentVersionID(ver.ID).Exec(ctx)
		return pkg
	}
	existingIDs, _ := ex.QueryMenus().IDs(ctx)
	if missing := missingIDs(existingIDs, menuIDs); len(missing) > 0 {
		c.TenantMenuPermissionGroup.UpdateOneID(ex.ID).AddMenuIDs(missing...).Exec(ctx)
	}
	return ex
}

func ensureUser(ctx context.Context, c *gen.Client, tid uint32, name, hash, email string) *gen.User {
	u, err := c.User.Query().Where(user.TenantIDEQ(tid), user.NameEQ(name)).Only(ctx)
	if err != nil {
		u = c.User.Create().SetTenantID(tid).SetName(name).SetPassword(hash).SetEmail(email).SetStatus(1).SaveX(ctx)
	}
	return u
}

func ensureRole(ctx context.Context, c *gen.Client, tid uint32, name string, menuIDs []uint32, isTA bool) *gen.Role {
	r, err := c.Role.Query().Where(role.TenantIDEQ(tid), role.NameEQ(name)).Only(ctx)
	if err != nil {
		r = c.Role.Create().SetTenantID(tid).SetName(name).SetDataScope(1).SetNillableIsTenantAdmin(&isTA).SetStatus(1).AddMenuIDs(menuIDs...).SaveX(ctx)
	} else {
		existingIDs, _ := r.QueryMenus().IDs(ctx)
		if missing := missingIDs(existingIDs, menuIDs); len(missing) > 0 {
			c.Role.UpdateOneID(r.ID).AddMenuIDs(missing...).Exec(ctx)
		}
	}
	return r
}

func missingIDs(existing, desired []uint32) []uint32 {
	seen := make(map[uint32]struct{}, len(existing))
	for _, id := range existing {
		seen[id] = struct{}{}
	}
	missing := make([]uint32, 0)
	for _, id := range desired {
		if _, ok := seen[id]; !ok {
			missing = append(missing, id)
		}
	}
	return missing
}

func ensureDept(ctx context.Context, c *gen.Client, tid uint32, name string, pid, lid uint32) (*gen.Dept, error) {
	ex, err := c.Dept.Query().Where(dept.TenantIDEQ(tid), dept.NameEQ(name)).Only(ctx)
	if err != nil {
		return c.Dept.Create().SetTenantID(tid).SetName(name).SetParentID(pid).SetNillableLeaderID(&lid).SetStatus(1).Save(ctx)
	}
	return ex, nil
}

func ensurePost(ctx context.Context, c *gen.Client, tid uint32, name string) {
	_, err := c.Post.Query().Where(post.TenantIDEQ(tid), post.NameEQ(name)).Only(ctx)
	if err != nil {
		c.Post.Create().SetTenantID(tid).SetName(name).SetStatus(1).SaveX(ctx)
	}
}

func ensureProject(ctx context.Context, c *gen.Client, tid uint32, name, code string, oid uint32, mids []uint32) {
	_, err := c.Project.Query().Where(project.TenantIDEQ(tid), project.CodeEQ(code)).Only(ctx)
	if err != nil {
		c.Project.Create().SetTenantID(tid).SetName(name).SetCode(code).SetOwnerID(oid).AddMemberIDs(mids...).SetStatus(1).SaveX(ctx)
	}
}

func verify(ctx context.Context, client *gen.Client) error {
	type check struct {
		name string
		fn   func(context.Context) (int, error)
		min  int
	}
	checks := []check{
		{"menus", func(ctx context.Context) (int, error) { return client.Menu.Query().Count(ctx) }, 105},
		{"tenant 1 users", func(ctx context.Context) (int, error) {
			return client.User.Query().Where(user.TenantIDEQ(1)).Count(ctx)
		}, 3},
		{"tenant 2 users", func(ctx context.Context) (int, error) {
			return client.User.Query().Where(user.TenantIDEQ(2)).Count(ctx)
		}, 1},
		{"packages", func(ctx context.Context) (int, error) { return client.TenantMenuPermissionGroup.Query().Count(ctx) }, 2},
		{"tenant 1 roles", func(ctx context.Context) (int, error) {
			return client.Role.Query().Where(role.TenantIDEQ(1)).Count(ctx)
		}, 1},
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
