//go:build mock

package main

import (
	"context"

	v1 "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/app/platform/service/internal/data/ent/gen/menu"
)

// platformMenus 技术中台管理后台菜单树。
var platformMenus = []menuSeed{
	// ── 个人中心（隐藏菜单：不在侧边栏展示，由用户下拉菜单进入；全部角色可见）──
	{Parent: "", Name: "ProfileCenter", Title: "个人中心", Path: "/profile", Component: "/system/profile/index", Icon: "mdi:account-circle-outline", Type: 2, Sort: 1},

	// ── 仪表盘 ──────────────────────────────────────────
	{Parent: "", Name: "Dashboard", Title: "仪表盘", Path: "/dashboard", Component: "BasicLayout", Icon: "ion:grid-outline", Type: 1, Sort: 10},
	{Parent: "Dashboard", Name: "DashboardWorkbench", Title: "工作台", Path: "/dashboard/workbench", Component: "/dashboard/workspace/index", Icon: "mdi:monitor-dashboard", Type: 2, Sort: 10},
	{Parent: "Dashboard", Name: "DashboardAnalytics", Title: "分析页", Path: "/dashboard/analytics", Component: "/dashboard/analytics/index", Icon: "mdi:chart-box-outline", Type: 2, Sort: 20},

	// ── 租户管理 ────────────────────────────────────────
	{Parent: "", Name: "TenantManagement", Title: "租户管理", Path: "/tenant", Component: "BasicLayout", Icon: "mdi:office-building-cog-outline", Type: 1, Sort: 20},
	{Parent: "TenantManagement", Name: "TenantList", Title: "租户列表", Path: "/tenant/list", Component: "/system/tenant/list", Icon: "mdi:office-building-outline", Type: 2, Sort: 10},
	{Parent: "TenantManagement", Name: "TenantPackage", Title: "租户套餐配置", Path: "/tenant/package", Component: "/system/tenant-menu-permission-group/list", Icon: "mdi:package-variant-closed", Type: 2, Sort: 20},

	// ── 组织架构 ────────────────────────────────────────
	{Parent: "", Name: "Organization", Title: "组织架构", Path: "/org", Component: "BasicLayout", Icon: "ion:people-outline", Type: 1, Sort: 30},
	{Parent: "Organization", Name: "UserManagement", Title: "用户与部门", Path: "/org/user", Component: "/system/user/list", Icon: "mdi:account-multiple-outline", Type: 2, Sort: 10},
	{Parent: "Organization", Name: "DeptManagement", Title: "部门管理", Path: "/org/dept", Component: "/system/user/list", Icon: "charm:organisation", Type: 2, Sort: 20},
	{Parent: "Organization", Name: "PostManagement", Title: "岗位管理", Path: "/org/post", Component: "/system/post/list", Icon: "mdi:badge-account-outline", Type: 2, Sort: 30},
	{Parent: "Organization", Name: "RoleManagement", Title: "角色管理", Path: "/org/role", Component: "/system/role/list", Icon: "mdi:shield-account-outline", Type: 2, Sort: 40},
	{Parent: "Organization", Name: "ProjectManagement", Title: "项目管理", Path: "/org/project", Component: "/system/project/list", Icon: "mdi:folder-cog-outline", Type: 2, Sort: 50},

	// ── 权限与安全（G2/G3 重组：菜单授权 + 安全审计归拢）──────────────
	{Parent: "", Name: "PermissionSecurity", Title: "权限与安全", Path: "/perm", Component: "BasicLayout", Icon: "ion:shield-checkmark-outline", Type: 1, Sort: 40},
	{Parent: "PermissionSecurity", Name: "MenuManagement", Title: "菜单管理", Path: "/perm/menu", Component: "/system/menu/list", Icon: "mdi:menu", Type: 2, Sort: 10},
	{Parent: "PermissionSecurity", Name: "OperationLog", Title: "操作审计", Path: "/perm/operation-log", Component: "/system/operation-log/list", Icon: "mdi:clipboard-text-clock-outline", Type: 2, Sort: 20},
	{Parent: "PermissionSecurity", Name: "LoginLog", Title: "登录日志", Path: "/perm/login-log", Component: "/system/login-log/list", Icon: "mdi:login-variant", Type: 2, Sort: 30},
	{Parent: "PermissionSecurity", Name: "Session", Title: "会话管理", Path: "/perm/session", Component: "/system/session/list", Icon: "mdi:account-clock-outline", Type: 2, Sort: 40},

	// ── 文件中心 ────────────────────────────────────────
	{Parent: "", Name: "FileCenter", Title: "文件中心", Path: "/file", Component: "BasicLayout", Icon: "ion:folder-open-outline", Type: 1, Sort: 50},
	{Parent: "FileCenter", Name: "FileList", Title: "文件列表", Path: "/file/list", Component: "/system/file-center/list", Icon: "mdi:file-document-outline", Type: 2, Sort: 10},
	{Parent: "FileCenter", Name: "StorageProvider", Title: "存储渠道", Path: "/file/storage", Component: "/system/storage-provider/list", Icon: "mdi:server-network-outline", Type: 2, Sort: 20},

	// ── 通知中心 ────────────────────────────────────────
	{Parent: "", Name: "NotificationCenter", Title: "通知中心", Path: "/notif", Component: "BasicLayout", Icon: "ion:notifications-outline", Type: 1, Sort: 60},
	{Parent: "NotificationCenter", Name: "NotifTemplate", Title: "通知模板", Path: "/notif/template", Component: "/system/notification/template", Icon: "mdi:email-outline", Type: 2, Sort: 10},
	{Parent: "NotificationCenter", Name: "NotifRecord", Title: "通知记录", Path: "/notif/record", Component: "/system/notification/record", Icon: "mdi:bell-outline", Type: 2, Sort: 20},
	{Parent: "NotificationCenter", Name: "NotifProvider", Title: "通知渠道配置", Path: "/notif/provider", Component: "/system/notification-provider/list", Icon: "mdi:message-cog-outline", Type: 2, Sort: 30},

	// ── 数据治理（G3 拆分后的系统管理，仅保留字典/参数）──────────────
	{Parent: "", Name: "SystemManagement", Title: "数据治理", Path: "/system", Component: "BasicLayout", Icon: "ion:settings-outline", Type: 1, Sort: 996},
	{Parent: "SystemManagement", Name: "Dictionary", Title: "字典管理", Path: "/system/dictionary", Component: "/system/dictionary/list", Icon: "mdi:book-cog-outline", Type: 2, Sort: 10},
	{Parent: "SystemManagement", Name: "Parameter", Title: "参数配置", Path: "/system/parameter", Component: "/system/parameter/list", Icon: "mdi:tune-variant", Type: 2, Sort: 20},

	// ── 集成与任务（G3 新目录）──────────────────────────────
	{Parent: "", Name: "IntegrationTasks", Title: "集成与任务", Path: "/integration", Component: "BasicLayout", Icon: "ion:git-network-outline", Type: 1, Sort: 997},
	{Parent: "IntegrationTasks", Name: "AsyncTask", Title: "异步任务", Path: "/integration/async-task", Component: "/system/async-task/list", Icon: "mdi:progress-clock", Type: 2, Sort: 10},
	{Parent: "IntegrationTasks", Name: "Webhook", Title: "Webhook管理", Path: "/integration/webhook", Component: "/system/webhook/list", Icon: "mdi:webhook", Type: 2, Sort: 20},

	// ── 开发者（G5 新目录：API 文档 + 健康总览）──────────────────
	// IframeSrc 为 dev 地址；生产环境由部署层覆盖同名菜单或调整 seed。
	{Parent: "", Name: "Developer", Title: "开发者", Path: "/developer", Component: "BasicLayout", Icon: "ion:code-slash-outline", Type: 1, Sort: 998},
	{Parent: "Developer", Name: "ApiDocs", Title: "API 文档", Path: "/developer/api-docs", Component: "IFrameView", IframeSrc: "http://127.0.0.1:8000/docs/swagger-ui/", Icon: "mdi:file-document-edit-outline", Type: 2, Sort: 10},
	{Parent: "Developer", Name: "HealthOverview", Title: "健康总览", Path: "/developer/health", Component: "/system/health/index", Icon: "mdi:heart-pulse", Type: 2, Sort: 20},
}

// seedMenus 幂等维护平台菜单树，返回 name→ID 映射。
func seedMenus(ctx context.Context, c *gen.Client) (map[string]uint32, error) {
	menuMap, err := seedMenuTree(ctx, c, platformMenus)
	if err != nil {
		return nil, err
	}

	// 部门管理页合并入“用户与部门”工作台：隐藏菜单、重定向。
	deptLegacy, err := c.Menu.Query().Where(menu.NameEQ("DeptManagement")).Only(ctx)
	if err == nil {
		c.Menu.UpdateOneID(deptLegacy.ID).SetHideInMenu(true).SetRedirect("/org/user").Exec(ctx)
	}
	// 个人中心：隐藏菜单（由用户下拉入口进入，不占侧边栏）。
	profileMenu, err := c.Menu.Query().Where(menu.NameEQ("ProfileCenter")).Only(ctx)
	if err == nil {
		c.Menu.UpdateOneID(profileMenu.ID).SetHideInMenu(true).Exec(ctx)
	}
	return menuMap, nil
}

// platformButtons 技术中台管理后台的权限按钮清单。
//
// 每个按钮的 Operation 直接引用 api 生成的 v1.OperationXxx 常量（对应 HTTP 接口），
// 编译期即可校验接口名是否存在；接口重命名/删除会在此处编译报错，天然防漂移。
//
// 后续新增接口按钮：在对应分组下追加一行 buttonSpec 即可，重新运行 mock 会增量 upsert。
var platformButtons = []buttonSpec{
	// ── 租户管理 ────────────────────────────────────────
	{Parent: "TenantList", Name: "TenantListQuery", Title: "查询", Operation: v1.OperationTenantServiceListTenants, Sort: 10},
	{Parent: "TenantList", Name: "TenantCreate", Title: "新增", Operation: v1.OperationTenantServiceCreateTenant, Sort: 20},
	{Parent: "TenantList", Name: "TenantEdit", Title: "编辑", Operation: v1.OperationTenantServiceUpdateTenant, Sort: 30},
	{Parent: "TenantList", Name: "TenantDelete", Title: "删除", Operation: v1.OperationTenantServiceDeleteTenant, Sort: 40},
	{Parent: "TenantList", Name: "TenantLifecycle", Title: "生命周期", Operation: v1.OperationTenantServiceUpdateTenantLifecycle, Sort: 50},
	{Parent: "TenantList", Name: "TenantAdminQuery", Title: "管理员查询", Operation: v1.OperationTenantServiceListTenantAdmins, Sort: 60},
	{Parent: "TenantList", Name: "TenantAdminEdit", Title: "管理员资料", Operation: v1.OperationTenantServiceUpdateTenantAdmin, Sort: 70},
	{Parent: "TenantList", Name: "TenantAdminPassword", Title: "管理员密码重置", Operation: v1.OperationTenantServiceResetTenantAdminPassword, Sort: 80},

	{Parent: "TenantPackage", Name: "TPQuery", Title: "查询", Operation: v1.OperationTenantMenuPermissionGroupServiceListTenantMenuPermissionGroups, Sort: 10},
	{Parent: "TenantPackage", Name: "TPCreate", Title: "新增", Operation: v1.OperationTenantMenuPermissionGroupServiceCreateTenantMenuPermissionGroup, Sort: 20},
	{Parent: "TenantPackage", Name: "TPEdit", Title: "编辑", Operation: v1.OperationTenantMenuPermissionGroupServiceUpdateTenantMenuPermissionGroup, Sort: 30},
	{Parent: "TenantPackage", Name: "TPDelete", Title: "删除", Operation: v1.OperationTenantMenuPermissionGroupServiceDeleteTenantMenuPermissionGroup, Sort: 40},
	{Parent: "TenantPackage", Name: "TPStatus", Title: "启停", Operation: v1.OperationTenantMenuPermissionGroupServiceUpdateTenantMenuPermissionGroupStatus, Sort: 50},
	{Parent: "TenantPackage", Name: "TPVersions", Title: "版本列表", Operation: v1.OperationTenantMenuPermissionGroupServiceListTenantMenuPermissionGroupVersions, Sort: 60},
	{Parent: "TenantPackage", Name: "TPPublish", Title: "发布版本", Operation: v1.OperationTenantMenuPermissionGroupServicePublishTenantMenuPermissionGroupVersion, Sort: 70},
	{Parent: "TenantPackage", Name: "TPRollback", Title: "回滚版本", Operation: v1.OperationTenantMenuPermissionGroupServiceRollbackTenantMenuPermissionGroupVersion, Sort: 80},

	// ── 组织架构 · 用户与部门 ─────────────────────────────
	{Parent: "UserManagement", Name: "UserQuery", Title: "查询", Operation: v1.OperationUserServiceListUsers, Sort: 10},
	{Parent: "UserManagement", Name: "UserCreate", Title: "新增", Operation: v1.OperationUserServiceCreateUser, Sort: 20},
	{Parent: "UserManagement", Name: "UserEdit", Title: "编辑", Operation: v1.OperationUserServiceUpdateUser, Sort: 30},
	{Parent: "UserManagement", Name: "UserDelete", Title: "删除", Operation: v1.OperationUserServiceDeleteUser, Sort: 40},
	{Parent: "UserManagement", Name: "UserStatus", Title: "启停", Operation: v1.OperationUserServiceUpdateUserByStatus, Sort: 50},
	{Parent: "UserManagement", Name: "DeptQuery", Title: "部门查询", Operation: v1.OperationDeptServiceListDepts, Sort: 60},
	{Parent: "UserManagement", Name: "DeptCreate", Title: "部门新增", Operation: v1.OperationDeptServiceCreateDept, Sort: 70},
	{Parent: "UserManagement", Name: "DeptEdit", Title: "部门编辑", Operation: v1.OperationDeptServiceUpdateDept, Sort: 80},
	{Parent: "UserManagement", Name: "DeptDelete", Title: "部门删除", Operation: v1.OperationDeptServiceDeleteDept, Sort: 90},
	{Parent: "UserManagement", Name: "DeptStatus", Title: "部门启停", Operation: v1.OperationDeptServiceUpdateDeptByStatus, Sort: 100},
	{Parent: "UserManagement", Name: "DeptTreeQuery", Title: "部门树查询", Operation: v1.OperationDeptServiceListDeptsTree, Sort: 110},
	{Parent: "UserManagement", Name: "DeptDeleteImpact", Title: "部门删除检查", Operation: v1.OperationDeptServiceGetDeptDeleteImpact, Sort: 120},
	{Parent: "UserManagement", Name: "DeptTransferDelete", Title: "人员转移并删除部门", Operation: v1.OperationDeptServiceTransferAndDeleteDept, Sort: 130},
	{Parent: "UserManagement", Name: "RoleSimpleQuery", Title: "角色简单列表", Operation: v1.OperationRoleServiceListRoleSimple, Sort: 140},

	{Parent: "PostManagement", Name: "PostQuery", Title: "查询", Operation: v1.OperationPostServiceListPosts, Sort: 10},
	{Parent: "PostManagement", Name: "PostCreate", Title: "新增", Operation: v1.OperationPostServiceCreatePost, Sort: 20},
	{Parent: "PostManagement", Name: "PostEdit", Title: "编辑", Operation: v1.OperationPostServiceUpdatePost, Sort: 30},
	{Parent: "PostManagement", Name: "PostDelete", Title: "删除", Operation: v1.OperationPostServiceDeletePost, Sort: 40},
	{Parent: "PostManagement", Name: "PostStatus", Title: "启停", Operation: v1.OperationPostServiceUpdatePostByStatus, Sort: 50},

	{Parent: "RoleManagement", Name: "RoleQuery", Title: "查询", Operation: v1.OperationRoleServiceListRoles, Sort: 10},
	{Parent: "RoleManagement", Name: "RoleCreate", Title: "新增", Operation: v1.OperationRoleServiceCreateRole, Sort: 20},
	{Parent: "RoleManagement", Name: "RoleEdit", Title: "编辑", Operation: v1.OperationRoleServiceUpdateRole, Sort: 30},
	{Parent: "RoleManagement", Name: "RoleDelete", Title: "删除", Operation: v1.OperationRoleServiceDeleteRole, Sort: 40},
	{Parent: "RoleManagement", Name: "RoleStatus", Title: "启停", Operation: v1.OperationRoleServiceUpdateRoleByStatus, Sort: 50},
	{Parent: "RoleManagement", Name: "RoleSimpleQuery", Title: "角色简单列表", Operation: v1.OperationRoleServiceListRoleSimple, Sort: 60},

	{Parent: "ProjectManagement", Name: "ProjectQuery", Title: "查询", Operation: v1.OperationProjectServiceListProjects, Sort: 10},
	{Parent: "ProjectManagement", Name: "ProjectCreate", Title: "新增", Operation: v1.OperationProjectServiceCreateProject, Sort: 20},
	{Parent: "ProjectManagement", Name: "ProjectEdit", Title: "编辑", Operation: v1.OperationProjectServiceUpdateProject, Sort: 30},
	{Parent: "ProjectManagement", Name: "ProjectDelete", Title: "删除", Operation: v1.OperationProjectServiceDeleteProject, Sort: 40},
	{Parent: "ProjectManagement", Name: "ProjectStatus", Title: "启停", Operation: v1.OperationProjectServiceUpdateProjectByStatus, Sort: 50},

	// ── 菜单与权限 ──────────────────────────────────────
	{Parent: "MenuManagement", Name: "MenuTree", Title: "树查询", Operation: v1.OperationMenuServiceListMenusTree, Sort: 9},
	{Parent: "MenuManagement", Name: "MenuQuery", Title: "查询", Operation: v1.OperationMenuServiceListMenus, Sort: 10},
	{Parent: "MenuManagement", Name: "MenuCreate", Title: "新增", Operation: v1.OperationMenuServiceCreateMenu, Sort: 20},
	{Parent: "MenuManagement", Name: "MenuEdit", Title: "编辑", Operation: v1.OperationMenuServiceUpdateMenu, Sort: 30},
	{Parent: "MenuManagement", Name: "MenuDelete", Title: "删除", Operation: v1.OperationMenuServiceDeleteMenu, Sort: 40},
	{Parent: "MenuManagement", Name: "MenuStatus", Title: "启停", Operation: v1.OperationMenuServiceUpdateMenuByStatus, Sort: 50},

	// ── 文件中心 ────────────────────────────────────────
	{Parent: "FileList", Name: "FileQuery", Title: "查询", Operation: v1.OperationFileCenterServiceListFileObjects, Sort: 10},
	{Parent: "FileList", Name: "FileUpload", Title: "上传", Operation: v1.OperationFileCenterServiceCreateFileUploadSession, Sort: 20},
	{Parent: "FileList", Name: "FileDelete", Title: "删除", Operation: v1.OperationFileCenterServiceDeleteFileObject, Sort: 30},
	{Parent: "FileList", Name: "FileDownload", Title: "下载", Operation: v1.OperationFileCenterServicePresignFileDownload, Sort: 40},

	{Parent: "StorageProvider", Name: "StorageQuery", Title: "查询", Operation: v1.OperationStorageProviderServiceListStorageProviders, Sort: 10},
	{Parent: "StorageProvider", Name: "StorageCreate", Title: "新增", Operation: v1.OperationStorageProviderServiceCreateStorageProvider, Sort: 20},
	{Parent: "StorageProvider", Name: "StorageEdit", Title: "编辑", Operation: v1.OperationStorageProviderServiceUpdateStorageProvider, Sort: 30},
	{Parent: "StorageProvider", Name: "StorageDelete", Title: "删除", Operation: v1.OperationStorageProviderServiceDeleteStorageProvider, Sort: 40},
	{Parent: "StorageProvider", Name: "StorageDefault", Title: "设为默认", Operation: v1.OperationStorageProviderServiceSetDefaultStorageProvider, Sort: 50},

	// ── 通知中心 ────────────────────────────────────────
	{Parent: "NotifTemplate", Name: "NTQuery", Title: "查询", Operation: v1.OperationNotificationServiceListNotificationTemplates, Sort: 10},
	{Parent: "NotifTemplate", Name: "NTCreate", Title: "新增", Operation: v1.OperationNotificationServiceCreateNotificationTemplate, Sort: 20},
	{Parent: "NotifTemplate", Name: "NTEdit", Title: "编辑", Operation: v1.OperationNotificationServiceUpdateNotificationTemplate, Sort: 30},
	{Parent: "NotifTemplate", Name: "NTDelete", Title: "删除", Operation: v1.OperationNotificationServiceDeleteNotificationTemplate, Sort: 40},

	{Parent: "NotifRecord", Name: "NMsgQuery", Title: "查询", Operation: v1.OperationNotificationServiceListNotificationMessages, Sort: 10},
	{Parent: "NotifRecord", Name: "NMsgSend", Title: "发送", Operation: v1.OperationNotificationServiceSendInAppNotification, Sort: 20},

	{Parent: "NotifProvider", Name: "NPQuery", Title: "查询", Operation: v1.OperationNotificationProviderServiceListNotificationProviders, Sort: 10},
	{Parent: "NotifProvider", Name: "NPCreate", Title: "新增", Operation: v1.OperationNotificationProviderServiceCreateNotificationProvider, Sort: 20},
	{Parent: "NotifProvider", Name: "NPEdit", Title: "编辑", Operation: v1.OperationNotificationProviderServiceUpdateNotificationProvider, Sort: 30},
	{Parent: "NotifProvider", Name: "NPDelete", Title: "删除", Operation: v1.OperationNotificationProviderServiceDeleteNotificationProvider, Sort: 40},
	{Parent: "NotifProvider", Name: "NPDefault", Title: "设默认", Operation: v1.OperationNotificationProviderServiceSetDefaultNotificationProvider, Sort: 50},
	{Parent: "NotifProvider", Name: "NPTest", Title: "测试", Operation: v1.OperationNotificationProviderServiceTestNotificationProvider, Sort: 60},

	// ── 系统管理 ────────────────────────────────────────
	{Parent: "Dictionary", Name: "DictQuery", Title: "查询", Operation: v1.OperationDictionaryServiceListDictionaryTypes, Sort: 10},
	{Parent: "Dictionary", Name: "DictCreate", Title: "新增", Operation: v1.OperationDictionaryServiceCreateDictionaryType, Sort: 20},
	{Parent: "Dictionary", Name: "DictEdit", Title: "编辑", Operation: v1.OperationDictionaryServiceUpdateDictionaryType, Sort: 30},
	{Parent: "Dictionary", Name: "DictDelete", Title: "删除", Operation: v1.OperationDictionaryServiceDeleteDictionaryType, Sort: 40},

	{Parent: "Parameter", Name: "ParamQuery", Title: "查询", Operation: v1.OperationParameterServiceListParameterDefinitions, Sort: 10},
	{Parent: "Parameter", Name: "ParamCreate", Title: "新增", Operation: v1.OperationParameterServiceCreateParameterDefinition, Sort: 20},
	{Parent: "Parameter", Name: "ParamEdit", Title: "编辑", Operation: v1.OperationParameterServiceUpdateParameterDefinition, Sort: 30},
	{Parent: "Parameter", Name: "ParamDelete", Title: "删除", Operation: v1.OperationParameterServiceDeleteParameterDefinition, Sort: 40},
	{Parent: "Parameter", Name: "ParamDetail", Title: "定义详情", Operation: v1.OperationParameterServiceGetParameterDefinition, Sort: 45},
	// 租户参数运行时（/parameters/current 系列）：此前无按钮，所有角色均 403（Q1 根因）。
	{Parent: "Parameter", Name: "ParamCurrentQuery", Title: "查看本租户参数", Operation: v1.OperationParameterServiceListCurrentTenantParameters, Sort: 50},
	{Parent: "Parameter", Name: "ParamCurrentSet", Title: "设置本租户参数", Operation: v1.OperationParameterServiceSetCurrentTenantParameter, Sort: 55},
	{Parent: "Parameter", Name: "ParamCurrentReset", Title: "重置本租户参数", Operation: v1.OperationParameterServiceResetCurrentTenantParameter, Sort: 60},
	// 平台代管指定租户参数（/tenants/{id}/parameters 系列，仅超管可达）。
	{Parent: "Parameter", Name: "ParamTenantQuery", Title: "查看指定租户参数", Operation: v1.OperationParameterServiceListTenantParameters, Sort: 65},
	{Parent: "Parameter", Name: "ParamTenantSet", Title: "设置指定租户参数", Operation: v1.OperationParameterServiceSetTenantParameter, Sort: 70},
	{Parent: "Parameter", Name: "ParamTenantReset", Title: "重置指定租户参数", Operation: v1.OperationParameterServiceResetTenantParameter, Sort: 75},

	{Parent: "OperationLog", Name: "OLQuery", Title: "查询", Operation: v1.OperationOperationLogServiceListOperationLogs, Sort: 10},
	{Parent: "LoginLog", Name: "LLQuery", Title: "查询", Operation: v1.OperationLoginLogServiceListLoginLogs, Sort: 10},

	{Parent: "Session", Name: "SessionQuery", Title: "查询", Operation: v1.OperationSessionServiceListSessions, Sort: 10},
	{Parent: "Session", Name: "SessionRevoke", Title: "踢下线", Operation: v1.OperationSessionServiceRevokeSession, Sort: 20},

	{Parent: "AsyncTask", Name: "ATQuery", Title: "查询", Operation: v1.OperationAsyncTaskServiceListAsyncTasks, Sort: 10},
	{Parent: "AsyncTask", Name: "ATCancel", Title: "取消", Operation: v1.OperationAsyncTaskServiceCancelAsyncTask, Sort: 20},
	{Parent: "AsyncTask", Name: "ATRetry", Title: "重试", Operation: v1.OperationAsyncTaskServiceRetryAsyncTask, Sort: 30},

	{Parent: "Webhook", Name: "WHQuery", Title: "查询", Operation: v1.OperationWebhookServiceListWebhookSubscriptions, Sort: 10},
	{Parent: "Webhook", Name: "WHCreate", Title: "新增", Operation: v1.OperationWebhookServiceCreateWebhookSubscription, Sort: 20},
	{Parent: "Webhook", Name: "WHEdit", Title: "编辑", Operation: v1.OperationWebhookServiceUpdateWebhookSubscription, Sort: 30},
	{Parent: "Webhook", Name: "WHDelete", Title: "删除", Operation: v1.OperationWebhookServiceDeleteWebhookSubscription, Sort: 40},
	{Parent: "Webhook", Name: "WHRetry", Title: "重试", Operation: v1.OperationWebhookServiceRetryWebhookDelivery, Sort: 50},

	// ── 详情与从属操作补齐 ────────────────────────────────
	// 2026-10-24 系统性审计：这些 operation 在 authzpolicy 中受保护，但 seed 无对应
	// 按钮（= 无 Casbin 策略），导致包括超管在内的所有角色 403。审计脚本见
	// 4-8 §七 Q6；防止再犯的 CI 门禁见 scripts/check-operation-coverage.sh。
	// DeviceService 四个 operation 无对应页面（前端未消费），暂不建按钮，见忽略清单。
	{Parent: "TenantList", Name: "TenantDetail", Title: "详情", Operation: v1.OperationTenantServiceGetTenant, Sort: 15},
	{Parent: "TenantPackage", Name: "TPDetail", Title: "详情", Operation: v1.OperationTenantMenuPermissionGroupServiceGetTenantMenuPermissionGroup, Sort: 15},

	{Parent: "UserManagement", Name: "UserDetail", Title: "用户详情", Operation: v1.OperationUserServiceGetUser, Sort: 15},
	{Parent: "UserManagement", Name: "UserSimpleQuery", Title: "用户简单列表", Operation: v1.OperationUserServiceListUsersSimple, Sort: 150},
	{Parent: "UserManagement", Name: "DeptDetail", Title: "部门详情", Operation: v1.OperationDeptServiceGetDept, Sort: 125},

	{Parent: "PostManagement", Name: "PostDetail", Title: "详情", Operation: v1.OperationPostServiceGetPost, Sort: 15},
	{Parent: "RoleManagement", Name: "RoleDetail", Title: "详情", Operation: v1.OperationRoleServiceGetRole, Sort: 15},
	{Parent: "ProjectManagement", Name: "ProjectDetail", Title: "详情", Operation: v1.OperationProjectServiceGetProject, Sort: 15},

	{Parent: "MenuManagement", Name: "MenuDetail", Title: "详情", Operation: v1.OperationMenuServiceGetMenu, Sort: 15},
	{Parent: "MenuManagement", Name: "MenuAll", Title: "全量列表", Operation: v1.OperationMenuServiceListMenusAll, Sort: 17},

	{Parent: "FileList", Name: "FileDetail", Title: "详情", Operation: v1.OperationFileCenterServiceGetFileObject, Sort: 15},
	{Parent: "FileList", Name: "FileAccessLog", Title: "访问日志", Operation: v1.OperationFileCenterServiceListFileAccessLogs, Sort: 50},
	{Parent: "FileList", Name: "FilePartList", Title: "分片列表", Operation: v1.OperationFileCenterServiceListFileParts, Sort: 60},
	{Parent: "FileList", Name: "FilePartUpload", Title: "分片上传", Operation: v1.OperationFileCenterServiceUploadFilePart, Sort: 70},
	{Parent: "FileList", Name: "FileComplete", Title: "完成上传", Operation: v1.OperationFileCenterServiceCompleteFileUpload, Sort: 80},
	{Parent: "FileList", Name: "FileAbort", Title: "中止上传", Operation: v1.OperationFileCenterServiceAbortFileUpload, Sort: 90},
	{Parent: "FileList", Name: "FileReplace", Title: "替换内容", Operation: v1.OperationFileCenterServiceReplaceFileContent, Sort: 100},
	{Parent: "FileList", Name: "FileEdit", Title: "编辑元数据", Operation: v1.OperationFileCenterServiceUpdateFileObject, Sort: 110},

	{Parent: "StorageProvider", Name: "StorageDetail", Title: "详情", Operation: v1.OperationStorageProviderServiceGetStorageProvider, Sort: 15},
	{Parent: "StorageProvider", Name: "StorageTest", Title: "测试连通", Operation: v1.OperationStorageProviderServiceTestStorageProvider, Sort: 60},

	{Parent: "NotifTemplate", Name: "NTDetail", Title: "详情", Operation: v1.OperationNotificationServiceGetNotificationTemplate, Sort: 15},
	{Parent: "NotifRecord", Name: "NMsgDetail", Title: "详情", Operation: v1.OperationNotificationServiceGetNotificationMessage, Sort: 15},
	{Parent: "NotifRecord", Name: "NMsgSendGeneral", Title: "发送(通用)", Operation: v1.OperationNotificationServiceSendNotification, Sort: 30},
	{Parent: "NotifProvider", Name: "NPDetail", Title: "详情", Operation: v1.OperationNotificationProviderServiceGetNotificationProvider, Sort: 15},

	{Parent: "Dictionary", Name: "DictTypeDetail", Title: "类型详情", Operation: v1.OperationDictionaryServiceGetDictionaryType, Sort: 15},
	{Parent: "Dictionary", Name: "DictItemQuery", Title: "条目查询", Operation: v1.OperationDictionaryServiceListDictionaryItems, Sort: 50},
	{Parent: "Dictionary", Name: "DictItemCreate", Title: "条目新增", Operation: v1.OperationDictionaryServiceCreateDictionaryItem, Sort: 60},
	{Parent: "Dictionary", Name: "DictItemEdit", Title: "条目编辑", Operation: v1.OperationDictionaryServiceUpdateDictionaryItem, Sort: 70},
	{Parent: "Dictionary", Name: "DictItemDelete", Title: "条目删除", Operation: v1.OperationDictionaryServiceDeleteDictionaryItem, Sort: 80},

	{Parent: "OperationLog", Name: "OLDetail", Title: "详情", Operation: v1.OperationOperationLogServiceGetOperationLog, Sort: 15},
	{Parent: "LoginLog", Name: "LLDetail", Title: "详情", Operation: v1.OperationLoginLogServiceGetLoginLog, Sort: 15},

	{Parent: "AsyncTask", Name: "ATStats", Title: "统计", Operation: v1.OperationAsyncTaskServiceGetAsyncTaskStats, Sort: 5},
	{Parent: "AsyncTask", Name: "ATDetail", Title: "详情", Operation: v1.OperationAsyncTaskServiceGetAsyncTask, Sort: 15},

	{Parent: "Webhook", Name: "WHDetail", Title: "详情", Operation: v1.OperationWebhookServiceGetWebhookSubscription, Sort: 15},
	{Parent: "Webhook", Name: "WHLogQuery", Title: "投递日志", Operation: v1.OperationWebhookServiceListWebhookDeliveryLogs, Sort: 60},
	{Parent: "Webhook", Name: "WHLogDetail", Title: "投递日志详情", Operation: v1.OperationWebhookServiceGetWebhookDeliveryLog, Sort: 70},
}

// seedButtons 幂等维护平台权限按钮。
func seedButtons(ctx context.Context, c *gen.Client, menuMap map[string]uint32) error {
	return seedButtonList(ctx, c, platformButtons, menuMap)
}
