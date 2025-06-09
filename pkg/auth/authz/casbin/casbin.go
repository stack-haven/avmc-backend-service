package casbin

import (
	"context"
	"fmt"

	stdcasbin "github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/casbin/casbin/v2/persist"
	fileadapter "github.com/casbin/casbin/v2/persist/file-adapter"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	_ "github.com/go-sql-driver/mysql"

	"backend-service/pkg/auth/authz"
)

// 默认的RBAC模型定义
const defaultRBACModel = `
[request_definition]
r = sub, obj, act, dom

[policy_definition]
p = sub, obj, act, dom, eft

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow)) && !some(where (p.eft == deny))

[matchers]
m = g(r.sub, p.sub, r.dom) && r.obj == p.obj && r.act == p.act && r.dom == p.dom
`

// CasbinAuthorizer Casbin授权器实现
type CasbinAuthorizer struct {
	// options 配置选项
	options *authz.Options
	// enforcer Casbin执行器
	enforcer stdcasbin.IEnforcer
}

// CasbinProvider Casbin授权提供者
type CasbinProvider struct{}

// Name 获取提供者名称
func (p *CasbinProvider) Name() string {
	return "casbin"
}

// NewAuthorizer 创建新的授权器实例
func (p *CasbinProvider) NewAuthorizer(ctx context.Context, opts ...authz.Option) (authz.Authorizer, error) {
	// 使用默认选项
	options := authz.DefaultOptions()
	auth := new(CasbinAuthorizer)
	auth.options = options

	// 应用选项
	for _, opt := range opts {
		opt(auth.options)
	}

	// 初始化授权器
	if err := auth.Init(ctx, opts...); err != nil {
		return nil, err
	}

	return auth, nil
}

// Init 初始化授权器
func (a *CasbinAuthorizer) Init(ctx context.Context, opts ...authz.Option) error {
	var m model.Model
	var err error

	// 应用选项
	for _, opt := range opts {
		opt(a.options)
	}

	// 加载模型
	switch a.options.ModelFormat {
	case authz.ModelFormatText:
		modelText := a.options.ModelText
		if modelText == "" {
			modelText = defaultRBACModel
		}
		m, err = model.NewModelFromString(modelText)
		if err != nil {
			return authz.NewAuthzError(
				authz.ErrCodeInitializationFailed,
				"failed to load model from string",
				err,
			)
		}
	case authz.ModelFormatFile:
		if a.options.ModelFile == "" {
			return authz.NewAuthzError(
				authz.ErrCodeInvalidConfiguration,
				"model file path is required for file model format",
				nil,
			)
		}
		m, err = model.NewModelFromFile(a.options.ModelFile)
		if err != nil {
			return authz.NewAuthzError(
				authz.ErrCodeInitializationFailed,
				"failed to load model from file",
				err,
			)
		}
	default:
		return authz.NewAuthzError(
			authz.ErrCodeInvalidConfiguration,
			"unsupported model format",
			nil,
		)
	}

	// 加载适配器
	var adapter persist.Adapter
	switch a.options.AdapterType {
	case authz.AdapterFile:
		if a.options.AdapterDSN == "" {
			return authz.NewAuthzError(
				authz.ErrCodeInvalidConfiguration,
				"adapter DSN (policy file path) is required for file adapter",
				nil,
			)
		}
		adapter = fileadapter.NewAdapter(a.options.AdapterDSN)
	case authz.AdapterMemory:
	case authz.AdapterMySQL:
		println("MySQL adapter", a.options.AdapterDSN)
		if a.options.AdapterDSN == "" {
			return authz.NewAuthzError(
				authz.ErrCodeInvalidConfiguration,
				"adapter DSN (MySQL connection string) is required for MySQL adapter",
				nil,
			)
		}
		adapter, _ = gormadapter.NewAdapter("mysql", a.options.AdapterDSN, true) // Your driver and data source.
	default:
		// 其他适配器类型需要在实际应用中实现
		return authz.NewAuthzError(
			authz.ErrCodeInvalidConfiguration,
			fmt.Sprintf("unsupported adapter type: %s", a.options.AdapterType),
			nil,
		)
	}

	// 创建执行器
	if adapter != nil {
		a.enforcer, err = stdcasbin.NewEnforcer(m, adapter)
	} else {
		a.enforcer, err = stdcasbin.NewEnforcer(m)
	}

	if err != nil {
		return authz.NewAuthzError(
			authz.ErrCodeInitializationFailed,
			"failed to create enforcer",
			err,
		)
	}

	// 配置执行器
	// a.enforcer.SetAutoSave(a.options.AutoSave)

	// 启用日志
	if a.options.EnableLog {
		a.enforcer.EnableLog(true)
	}

	// 启用自动通知观察者
	if a.options.EnableWatcher && a.options.AutoNotifyWatcher {
		a.enforcer.EnableAutoNotifyWatcher(true)
	}

	return nil
}

// Enforce 执行授权检查
func (a *CasbinAuthorizer) Enforce(ctx context.Context, sub authz.Subject, obj authz.Object, act authz.Action, domain authz.Domain) (bool, error) {
	// 检查参数
	if sub == "" {
		return false, authz.NewAuthzError(authz.ErrCodeInvalidSubject, "subject is required", nil)
	}
	if obj == "" {
		return false, authz.NewAuthzError(authz.ErrCodeInvalidObject, "object is required", nil)
	}
	if act == "" {
		return false, authz.NewAuthzError(authz.ErrCodeInvalidAction, "action is required", nil)
	}

	// 执行授权检查
	result, err := a.enforcer.Enforce(string(sub), string(obj), string(act), string(domain))
	if err != nil {
		return false, authz.NewAuthzError(authz.ErrCodeEnforceFailed, "enforce check failed", err)
	}

	// 如果未授权，返回权限被拒绝错误
	if !result {
		return false, authz.NewAuthzError(
			authz.ErrCodePermissionDenied,
			fmt.Sprintf("permission denied for %s to %s on %s in domain %s", sub, act, obj, domain),
			nil,
		)
	}

	return result, nil
}

// BatchEnforce 批量执行授权检查
func (a *CasbinAuthorizer) BatchEnforce(ctx context.Context, subjects []authz.Subject, objects []authz.Object, actions []authz.Action, domains []authz.Domain) ([]bool, error) {
	// 检查参数长度一致性
	if len(subjects) != len(objects) || len(subjects) != len(actions) || len(subjects) != len(domains) {
		return nil, authz.NewAuthzError(
			authz.ErrCodeBatchEnforceFailed,
			"subjects, objects, actions, and domains must have the same length",
			nil,
		)
	}

	// 构建请求
	requests := make([][]interface{}, len(subjects))
	for i := range subjects {
		requests[i] = []interface{}{string(subjects[i]), string(objects[i]), string(actions[i]), string(domains[i])}
	}

	// 执行批量授权检查
	results, err := a.enforcer.BatchEnforce(requests)
	if err != nil {
		return nil, authz.NewAuthzError(authz.ErrCodeBatchEnforceFailed, "batch enforce check failed", err)
	}

	return results, nil
}

// AddPolicy 添加策略
func (a *CasbinAuthorizer) AddPolicy(ctx context.Context, policy authz.Policy) (bool, error) {
	// 检查策略
	if policy.Subject == "" {
		return false, authz.NewAuthzError(authz.ErrCodeInvalidPolicy, "subject is required in policy", nil)
	}
	if policy.Object == "" {
		return false, authz.NewAuthzError(authz.ErrCodeInvalidPolicy, "object is required in policy", nil)
	}
	if policy.Action == "" {
		return false, authz.NewAuthzError(authz.ErrCodeInvalidPolicy, "action is required in policy", nil)
	}

	// 添加策略
	effect := "allow"
	if policy.Effect == authz.EffectDeny {
		effect = "deny"
	}

	added, err := a.enforcer.AddPolicy(string(policy.Subject), string(policy.Object), string(policy.Action), string(policy.Domain), effect)
	if err != nil {
		return false, authz.NewAuthzError(authz.ErrCodeAddPolicyFailed, "add policy failed", err)
	}

	return added, nil
}

// RemovePolicy 移除策略
func (a *CasbinAuthorizer) RemovePolicy(ctx context.Context, policy authz.Policy) (bool, error) {
	// 检查策略
	if policy.Subject == "" {
		return false, authz.NewAuthzError(authz.ErrCodeInvalidPolicy, "subject is required in policy", nil)
	}
	if policy.Object == "" {
		return false, authz.NewAuthzError(authz.ErrCodeInvalidPolicy, "object is required in policy", nil)
	}
	if policy.Action == "" {
		return false, authz.NewAuthzError(authz.ErrCodeInvalidPolicy, "action is required in policy", nil)
	}

	// 移除策略
	effect := "allow"
	if policy.Effect == authz.EffectDeny {
		effect = "deny"
	}

	removed, err := a.enforcer.RemovePolicy(string(policy.Subject), string(policy.Object), string(policy.Action), string(policy.Domain), effect)
	if err != nil {
		return false, authz.NewAuthzError(authz.ErrCodeRemovePolicyFailed, "remove policy failed", err)
	}

	return removed, nil
}

// AddPolicies 批量添加策略
func (a *CasbinAuthorizer) AddPolicies(ctx context.Context, policies []authz.Policy) (bool, error) {
	// 检查策略
	if len(policies) == 0 {
		return false, nil
	}

	// 转换策略格式
	rules := make([][]string, len(policies))
	for i, policy := range policies {
		effect := "allow"
		if policy.Effect == authz.EffectDeny {
			effect = "deny"
		}
		rules[i] = []string{string(policy.Subject), string(policy.Object), string(policy.Action), string(policy.Domain), effect}
	}

	// 批量添加策略
	added, err := a.enforcer.AddPolicies(rules)
	if err != nil {
		return false, authz.NewAuthzError(authz.ErrCodeAddPoliciesFailed, "add policies failed", err)
	}

	return added, nil
}

// RemovePolicies 批量移除策略
func (a *CasbinAuthorizer) RemovePolicies(ctx context.Context, policies []authz.Policy) (bool, error) {
	// 检查策略
	if len(policies) == 0 {
		return false, nil
	}

	// 转换策略格式
	rules := make([][]string, len(policies))
	for i, policy := range policies {
		effect := "allow"
		if policy.Effect == authz.EffectDeny {
			effect = "deny"
		}
		rules[i] = []string{string(policy.Subject), string(policy.Object), string(policy.Action), string(policy.Domain), effect}
	}

	// 批量移除策略
	removed, err := a.enforcer.RemovePolicies(rules)
	if err != nil {
		return false, authz.NewAuthzError(authz.ErrCodeRemovePoliciesFailed, "remove policies failed", err)
	}

	return removed, nil
}

// GetAllSubjects 获取所有主体
func (a *CasbinAuthorizer) GetAllSubjects(ctx context.Context) ([]authz.Subject, error) {
	// 获取所有主体
	subjects, _ := a.enforcer.GetAllSubjects()

	// 转换为授权主体类型
	result := make([]authz.Subject, len(subjects))
	for i, sub := range subjects {
		result[i] = authz.Subject(sub)
	}

	return result, nil
}

// GetAllObjects 获取所有对象
func (a *CasbinAuthorizer) GetAllObjects(ctx context.Context) ([]authz.Object, error) {
	// 获取所有对象
	objects, _ := a.enforcer.GetAllObjects()

	// 转换为授权对象类型
	result := make([]authz.Object, len(objects))
	for i, obj := range objects {
		result[i] = authz.Object(obj)
	}

	return result, nil
}

// GetAllActions 获取所有操作
func (a *CasbinAuthorizer) GetAllActions(ctx context.Context) ([]authz.Action, error) {
	// 获取所有操作
	actions, _ := a.enforcer.GetAllActions()

	// 转换为授权操作类型
	result := make([]authz.Action, len(actions))
	for i, act := range actions {
		result[i] = authz.Action(act)
	}

	return result, nil
}

// GetAllDomains 获取所有域
func (a *CasbinAuthorizer) GetAllDomains(ctx context.Context) ([]authz.Domain, error) {
	// 获取所有域
	domains, _ := a.enforcer.GetAllDomains()

	// 转换为授权域类型
	result := make([]authz.Domain, len(domains))
	for i, dom := range domains {
		result[i] = authz.Domain(dom)
	}

	return result, nil
}

// GetAllRoles 获取所有角色
func (a *CasbinAuthorizer) GetAllRoles(ctx context.Context) ([]authz.Subject, error) {
	// 获取所有角色
	roles, _ := a.enforcer.GetAllRoles()

	// 转换为授权主体类型
	result := make([]authz.Subject, len(roles))
	for i, role := range roles {
		result[i] = authz.Subject(role)
	}

	return result, nil
}

// GetRolesForUser 获取用户的角色
func (a *CasbinAuthorizer) GetRolesForUser(ctx context.Context, user authz.Subject, domain authz.Domain) ([]authz.Subject, error) {
	// 获取用户的角色
	roles, err := a.enforcer.GetRolesForUser(string(user), string(domain))
	if err != nil {
		return nil, authz.NewAuthzError(authz.ErrCodeGetRolesForUserFailed, "get roles for user failed", err)
	}

	// 转换为授权主体类型
	result := make([]authz.Subject, len(roles))
	for i, role := range roles {
		result[i] = authz.Subject(role)
	}

	return result, nil
}

// GetUsersForRole 获取角色的用户
func (a *CasbinAuthorizer) GetUsersForRole(ctx context.Context, role authz.Subject, domain authz.Domain) ([]authz.Subject, error) {
	// 获取角色的用户
	users, err := a.enforcer.GetUsersForRole(string(role), string(domain))
	if err != nil {
		return nil, authz.NewAuthzError(authz.ErrCodeGetUsersForRoleFailed, "get users for role failed", err)
	}

	// 转换为授权主体类型
	result := make([]authz.Subject, len(users))
	for i, user := range users {
		result[i] = authz.Subject(user)
	}

	return result, nil
}

// HasRoleForUser 检查用户是否拥有角色
func (a *CasbinAuthorizer) HasRoleForUser(ctx context.Context, user authz.Subject, role authz.Subject, domain authz.Domain) (bool, error) {
	// 检查用户是否拥有角色
	has, err := a.enforcer.HasRoleForUser(string(user), string(role), string(domain))
	if err != nil {
		return false, authz.NewAuthzError(authz.ErrCodeHasRoleForUserFailed, "has role for user failed", err)
	}

	return has, nil
}

// AddRoleForUser 为用户添加角色
func (a *CasbinAuthorizer) AddRoleForUser(ctx context.Context, user authz.Subject, role authz.Subject, domain authz.Domain) (bool, error) {
	// 为用户添加角色
	added, err := a.enforcer.AddRoleForUser(string(user), string(role), string(domain))
	if err != nil {
		return false, authz.NewAuthzError(authz.ErrCodeAddRoleForUserFailed, "add role for user failed", err)
	}

	return added, nil
}

// DeleteRoleForUser 删除用户的角色
func (a *CasbinAuthorizer) DeleteRoleForUser(ctx context.Context, user authz.Subject, role authz.Subject, domain authz.Domain) (bool, error) {
	// 删除用户的角色
	deleted, err := a.enforcer.DeleteRoleForUser(string(user), string(role), string(domain))
	if err != nil {
		return false, authz.NewAuthzError(authz.ErrCodeDeleteRoleForUserFailed, "delete role for user failed", err)
	}

	return deleted, nil
}

// Name 获取授权器名称
func (a *CasbinAuthorizer) Name() string {
	return "casbin"
}

// Close 关闭授权器，释放资源
func (a *CasbinAuthorizer) Close() error {
	// 无需特殊清理
	return nil
}

// NewProvider 创建新的Casbin授权提供者
func NewProvider() authz.AuthzProvider {
	return &CasbinProvider{}
}
