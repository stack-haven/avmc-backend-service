package zanzibar

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"backend-service/pkg/auth/authz"
)

// ZanzibarAuthorizer 实现基于Zanzibar的授权器
type ZanzibarAuthorizer struct {
	options *authz.Options
	// 关系数据存储
	relationships map[string]map[string]map[string]bool // object#relation -> subject -> bool
	// 关系配置
	relationConfigs map[string]map[string][]string // objectType -> relation -> []parentRelation
	mutex           sync.RWMutex
}

// ZanzibarProvider 实现Zanzibar授权提供者
type ZanzibarProvider struct {
	options *authz.Options
}

// ZanzibarOptions 定义Zanzibar特定的选项
type ZanzibarOptions struct {
	// RelationConfigs 关系配置，定义对象类型的关系及其父关系
	// 例如: {"document": {"viewer": ["editor", "owner"], "editor": ["owner"]}}
	RelationConfigs map[string]map[string][]string
	// DefaultDomain 默认域
	DefaultDomain string
}

// NewZanzibarProvider 创建新的Zanzibar授权提供者
func NewZanzibarProvider(options *authz.Options) *ZanzibarProvider {
	return &ZanzibarProvider{options: options}
}

// CreateAuthorizer 创建Zanzibar授权器
func (p *ZanzibarProvider) CreateAuthorizer() (authz.Authorizer, error) {
	// 获取Zanzibar特定选项
	zanzibarOpts, ok := p.options.ProviderOptions.(*ZanzibarOptions)
	if !ok || zanzibarOpts == nil {
		return nil, authz.NewAuthzError(authz.ErrCodeInvalidOptions, "invalid or missing Zanzibar options")
	}

	// 创建授权器
	authorizer := &ZanzibarAuthorizer{
		options:         p.options,
		relationships:   make(map[string]map[string]map[string]bool),
		relationConfigs: zanzibarOpts.RelationConfigs,
	}

	return authorizer, nil
}

// Name 返回提供者名称
func (p *ZanzibarProvider) Name() string {
	return "zanzibar"
}

// Enforce 执行授权策略
func (a *ZanzibarAuthorizer) Enforce(ctx context.Context, subject authz.Subject, object authz.Object, action authz.Action, domain authz.Domain) (bool, error) {
	// 从上下文中获取授权信息
	if subject == "" {
		var ok bool
		subject, ok = authz.SubjectFromContext(ctx)
		if !ok || subject == "" {
			return false, authz.NewAuthzError(authz.ErrCodeInvalidSubject, "missing subject")
		}
	}

	if object == "" {
		var ok bool
		object, ok = authz.ObjectFromContext(ctx)
		if !ok || object == "" {
			return false, authz.NewAuthzError(authz.ErrCodeInvalidObject, "missing object")
		}
	}

	if action == "" {
		var ok bool
		action, ok = authz.ActionFromContext(ctx)
		if !ok || action == "" {
			return false, authz.NewAuthzError(authz.ErrCodeInvalidAction, "missing action")
		}
	}

	if domain == "" {
		var ok bool
		domain, ok = authz.DomainFromContext(ctx)
		if !ok || domain == "" {
			// 使用默认域
			zanzibarOpts, ok := a.options.ProviderOptions.(*ZanzibarOptions)
			if !ok || zanzibarOpts == nil {
				return false, authz.NewAuthzError(authz.ErrCodeInvalidOptions, "invalid or missing Zanzibar options")
			}
			domain = authz.Domain(zanzibarOpts.DefaultDomain)
		}
	}

	// 在Zanzibar中，action通常映射到relation
	relation := string(action)

	// 检查权限
	return a.checkPermission(string(subject), string(object), relation, string(domain))
}

// BatchEnforce 批量执行授权策略
func (a *ZanzibarAuthorizer) BatchEnforce(ctx context.Context, subjects []authz.Subject, objects []authz.Object, actions []authz.Action, domains []authz.Domain) ([]bool, error) {
	if len(subjects) != len(objects) || len(subjects) != len(actions) || (len(domains) > 0 && len(subjects) != len(domains)) {
		return nil, authz.NewAuthzError(authz.ErrCodeInvalidRequest, "subjects, objects, actions, and domains must have the same length")
	}

	results := make([]bool, len(subjects))
	var err error

	for i := range subjects {
		var domain authz.Domain
		if len(domains) > 0 {
			domain = domains[i]
		}

		results[i], err = a.Enforce(ctx, subjects[i], objects[i], actions[i], domain)
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}

// AddPolicy 添加授权策略
func (a *ZanzibarAuthorizer) AddPolicy(ctx context.Context, subject authz.Subject, object authz.Object, action authz.Action, domain authz.Domain) error {
	// 在Zanzibar中，添加策略意味着添加关系
	return a.addRelation(string(subject), string(object), string(action), string(domain))
}

// RemovePolicy 移除授权策略
func (a *ZanzibarAuthorizer) RemovePolicy(ctx context.Context, subject authz.Subject, object authz.Object, action authz.Action, domain authz.Domain) error {
	// 在Zanzibar中，移除策略意味着移除关系
	return a.removeRelation(string(subject), string(object), string(action), string(domain))
}

// BatchAddPolicies 批量添加授权策略
func (a *ZanzibarAuthorizer) BatchAddPolicies(ctx context.Context, subjects []authz.Subject, objects []authz.Object, actions []authz.Action, domains []authz.Domain) error {
	if len(subjects) != len(objects) || len(subjects) != len(actions) || (len(domains) > 0 && len(subjects) != len(domains)) {
		return authz.NewAuthzError(authz.ErrCodeInvalidRequest, "subjects, objects, actions, and domains must have the same length")
	}

	for i := range subjects {
		var domain authz.Domain
		if len(domains) > 0 {
			domain = domains[i]
		}

		err := a.AddPolicy(ctx, subjects[i], objects[i], actions[i], domain)
		if err != nil {
			return err
		}
	}

	return nil
}

// BatchRemovePolicies 批量移除授权策略
func (a *ZanzibarAuthorizer) BatchRemovePolicies(ctx context.Context, subjects []authz.Subject, objects []authz.Object, actions []authz.Action, domains []authz.Domain) error {
	if len(subjects) != len(objects) || len(subjects) != len(actions) || (len(domains) > 0 && len(subjects) != len(domains)) {
		return authz.NewAuthzError(authz.ErrCodeInvalidRequest, "subjects, objects, actions, and domains must have the same length")
	}

	for i := range subjects {
		var domain authz.Domain
		if len(domains) > 0 {
			domain = domains[i]
		}

		err := a.RemovePolicy(ctx, subjects[i], objects[i], actions[i], domain)
		if err != nil {
			return err
		}
	}

	return nil
}

// GetSubjects 获取所有主体
func (a *ZanzibarAuthorizer) GetSubjects(ctx context.Context) ([]authz.Subject, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	subjectMap := make(map[string]bool)

	// 收集所有唯一的主体
	for _, relations := range a.relationships {
		for subject := range relations {
			subjectMap[subject] = true
		}
	}

	// 转换为切片
	subjects := make([]authz.Subject, 0, len(subjectMap))
	for subject := range subjectMap {
		subjects = append(subjects, authz.Subject(subject))
	}

	return subjects, nil
}

// GetObjects 获取所有对象
func (a *ZanzibarAuthorizer) GetObjects(ctx context.Context) ([]authz.Object, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	objectMap := make(map[string]bool)

	// 收集所有唯一的对象
	for objectRelation := range a.relationships {
		parts := strings.Split(objectRelation, "#")
		if len(parts) > 0 {
			objectMap[parts[0]] = true
		}
	}

	// 转换为切片
	objects := make([]authz.Object, 0, len(objectMap))
	for object := range objectMap {
		objects = append(objects, authz.Object(object))
	}

	return objects, nil
}

// GetActions 获取所有操作
func (a *ZanzibarAuthorizer) GetActions(ctx context.Context) ([]authz.Action, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	actionMap := make(map[string]bool)

	// 收集所有唯一的操作（关系）
	for objectRelation := range a.relationships {
		parts := strings.Split(objectRelation, "#")
		if len(parts) > 1 {
			actionMap[parts[1]] = true
		}
	}

	// 转换为切片
	actions := make([]authz.Action, 0, len(actionMap))
	for action := range actionMap {
		actions = append(actions, authz.Action(action))
	}

	return actions, nil
}

// GetDomains 获取所有域
func (a *ZanzibarAuthorizer) GetDomains(ctx context.Context) ([]authz.Domain, error) {
	// 在此简单实现中，我们不跟踪域
	// 在实际实现中，可能需要从关系中提取域信息
	zanzibarOpts, ok := a.options.ProviderOptions.(*ZanzibarOptions)
	if !ok || zanzibarOpts == nil {
		return nil, authz.NewAuthzError(authz.ErrCodeInvalidOptions, "invalid or missing Zanzibar options")
	}

	return []authz.Domain{authz.Domain(zanzibarOpts.DefaultDomain)}, nil
}

// GetPolicies 获取所有策略
func (a *ZanzibarAuthorizer) GetPolicies(ctx context.Context) ([]authz.Policy, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	policies := []authz.Policy{}

	// 从关系中提取策略
	for objectRelation, subjects := range a.relationships {
		parts := strings.Split(objectRelation, "#")
		if len(parts) < 2 {
			continue
		}

		object := parts[0]
		action := parts[1]

		for subject, allowed := range subjects {
			if allowed {
				policies = append(policies, authz.Policy{
					Subject: authz.Subject(subject),
					Object:  authz.Object(object),
					Action:  authz.Action(action),
					Effect:  authz.EffectAllow,
				})
			}
		}
	}

	return policies, nil
}

// GetRoles 获取所有角色
func (a *ZanzibarAuthorizer) GetRoles(ctx context.Context) ([]string, error) {
	// Zanzibar不直接支持角色概念，但我们可以将某些主体视为角色
	// 在此简单实现中，我们返回空列表
	return []string{}, nil
}

// GetUsersForRole 获取角色的所有用户
func (a *ZanzibarAuthorizer) GetUsersForRole(ctx context.Context, role string) ([]string, error) {
	// Zanzibar不直接支持角色概念
	return nil, authz.NewAuthzError(authz.ErrCodeUnsupportedOperation, "role management not supported by Zanzibar authorizer")
}

// GetRolesForUser 获取用户的所有角色
func (a *ZanzibarAuthorizer) GetRolesForUser(ctx context.Context, user string) ([]string, error) {
	// Zanzibar不直接支持角色概念
	return nil, authz.NewAuthzError(authz.ErrCodeUnsupportedOperation, "role management not supported by Zanzibar authorizer")
}

// AddRoleForUser 为用户添加角色
func (a *ZanzibarAuthorizer) AddRoleForUser(ctx context.Context, user string, role string) error {
	// Zanzibar不直接支持角色概念
	return authz.NewAuthzError(authz.ErrCodeUnsupportedOperation, "role management not supported by Zanzibar authorizer")
}

// RemoveRoleForUser 移除用户的角色
func (a *ZanzibarAuthorizer) RemoveRoleForUser(ctx context.Context, user string, role string) error {
	// Zanzibar不直接支持角色概念
	return authz.NewAuthzError(authz.ErrCodeUnsupportedOperation, "role management not supported by Zanzibar authorizer")
}

// checkPermission 检查主体是否有权限执行对象上的关系
func (a *ZanzibarAuthorizer) checkPermission(subject, object, relation, domain string) (bool, error) {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	// 构建对象关系键
	objectRelationKey := fmt.Sprintf("%s#%s", object, relation)

	// 直接检查关系
	if subjectMap, ok := a.relationships[objectRelationKey]; ok {
		if allowed, ok := subjectMap[subject]; ok && allowed {
			return true, nil
		}
	}

	// 检查对象类型和关系
	objectType := getObjectType(object)
	if objectType == "" {
		return false, nil
	}

	// 检查关系配置
	if relationConfig, ok := a.relationConfigs[objectType]; ok {
		if parentRelations, ok := relationConfig[relation]; ok {
			// 检查父关系
			for _, parentRelation := range parentRelations {
				parentObjectRelationKey := fmt.Sprintf("%s#%s", object, parentRelation)
				if subjectMap, ok := a.relationships[parentObjectRelationKey]; ok {
					if allowed, ok := subjectMap[subject]; ok && allowed {
						return true, nil
					}
				}
			}
		}
	}

	return false, nil
}

// addRelation 添加关系
func (a *ZanzibarAuthorizer) addRelation(subject, object, relation, domain string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 构建对象关系键
	objectRelationKey := fmt.Sprintf("%s#%s", object, relation)

	// 确保映射存在
	if _, ok := a.relationships[objectRelationKey]; !ok {
		a.relationships[objectRelationKey] = make(map[string]map[string]bool)
	}

	if _, ok := a.relationships[objectRelationKey][subject]; !ok {
		a.relationships[objectRelationKey][subject] = make(map[string]bool)
	}

	// 添加关系
	a.relationships[objectRelationKey][subject] = true

	return nil
}

// removeRelation 移除关系
func (a *ZanzibarAuthorizer) removeRelation(subject, object, relation, domain string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	// 构建对象关系键
	objectRelationKey := fmt.Sprintf("%s#%s", object, relation)

	// 检查映射是否存在
	if subjectMap, ok := a.relationships[objectRelationKey]; ok {
		delete(subjectMap, subject)
		// 如果主体映射为空，删除对象关系映射
		if len(subjectMap) == 0 {
			delete(a.relationships, objectRelationKey)
		}
	}

	return nil
}

// getObjectType 从对象ID中提取对象类型
func getObjectType(objectID string) string {
	parts := strings.Split(objectID, ":")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}
