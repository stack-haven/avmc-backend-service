package opa

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"backend-service/pkg/auth/authz"

	"github.com/open-policy-agent/opa/rego"
)

var _ authz.Authorizer = (*OPAAuthorizer)(nil)

// OPAAuthorizer 实现基于Open Policy Agent的授权器
type OPAAuthorizer struct {
	options       *authz.Options
	preparedQuery *rego.PreparedEvalQuery
	httpClient    *http.Client
	opaURL        string
	policyPath    string
}

// OPAProvider 实现Open Policy Agent授权提供者
type OPAProvider struct {
	options *authz.Options
}

// OPAOptions 定义OPA特定的选项
type OPAOptions struct {
	// URL OPA服务器URL，用于远程模式
	URL string
	// PolicyPath 策略路径，用于远程模式
	PolicyPath string
	// QueryPath 查询路径，用于远程模式
	QueryPath string
	// Policy Rego策略，用于本地模式
	Policy string
	// Query Rego查询，用于本地模式
	Query string
	// LocalMode 是否使用本地模式
	LocalMode bool
}

// NewOPAProvider 创建新的OPA授权提供者
func NewOPAProvider(options *authz.Options) *OPAProvider {
	return &OPAProvider{options: options}
}

// Name 获取授权器名称
func (a *OPAAuthorizer) Name() string {
	return "opa"
}

func (p *OPAAuthorizer) Init(ctx context.Context, opts ...authz.Option) error {
	return nil
}

// CreateAuthorizer 创建OPA授权器
func (p *OPAProvider) CreateAuthorizer() (authz.Authorizer, error) {
	// 获取OPA特定选项
	opaOpts, ok := p.options.ProviderOptions.(*OPAOptions)
	if !ok || opaOpts == nil {
		return nil, authz.NewAuthzError(authz.ErrCodeInitializationFailed, "invalid or missing OPA options", nil)
	}

	// 创建授权器
	authorizer := &OPAAuthorizer{
		options:    p.options,
		httpClient: &http.Client{},
	}

	// 根据模式初始化
	if opaOpts.LocalMode {
		// 本地模式：使用内嵌的Rego引擎
		if opaOpts.Policy == "" {
			return nil, authz.NewAuthzError(authz.ErrCodeInitializationFailed, "missing Rego policy for local mode", nil)
		}
		if opaOpts.Query == "" {
			return nil, authz.NewAuthzError(authz.ErrCodeInitializationFailed, "missing Rego query for local mode", nil)
		}

		// 准备查询
		query, err := rego.New(
			rego.Query(opaOpts.Query),
			rego.Module("policy.rego", opaOpts.Policy),
		).PrepareForEval(context.Background())
		if err != nil {
			return nil, authz.NewAuthzError(authz.ErrCodeInitializationFailed, fmt.Sprintf("failed to prepare Rego query: %v", err), nil)
		}

		authorizer.preparedQuery = &query
		authorizer.options.Mode = authz.ModeLocal
	} else {
		// 远程模式：使用OPA服务器
		if opaOpts.URL == "" {
			return nil, authz.NewAuthzError(authz.ErrCodeInitializationFailed, "missing OPA server URL for remote mode", nil)
		}

		authorizer.options.RemoteURL = strings.TrimSuffix(opaOpts.URL, "/")
		authorizer.options.Mode = authz.ModeRemote
		authorizer.opaURL = strings.TrimSuffix(opaOpts.URL, "/")
		authorizer.policyPath = opaOpts.PolicyPath
	}

	return authorizer, nil
}

// Name 返回提供者名称
func (p *OPAProvider) Name() string {
	return "opa"
}

// Enforce 执行授权检查
func (a *OPAAuthorizer) Enforce(ctx context.Context, sub authz.Subject, obj authz.Object, act authz.Action, dom authz.Domain) (bool, error) {
	// 构建输入数据
	input := map[string]interface{}{
		"subject": string(sub),
		"object":  string(obj),
		"action":  string(act),
		"domain":  string(dom),
	}

	// 根据模式执行授权检查
	if a.options.Mode == authz.ModeLocal {
		return a.enforceLocal(ctx, input)
	}
	return a.enforceRemote(ctx, input)
}

// enforceLocal 使用本地Rego引擎执行授权检查
func (a *OPAAuthorizer) enforceLocal(ctx context.Context, input map[string]interface{}) (bool, error) {
	if a.options.Mode == authz.ModeLocal && a.preparedQuery != nil {
		// 执行查询
		results, err := a.preparedQuery.Eval(ctx, rego.EvalInput(input))
		if err != nil {
			return false, authz.NewAuthzError(
				authz.ErrCodeEnforceFailed,
				"failed to evaluate policy",
				err,
			)
		}

		// 解析结果
		if len(results) == 0 || len(results[0].Expressions) == 0 {
			return false, nil
		}

		// 提取布尔结果
		allowed, ok := results[0].Expressions[0].Value.(bool)
		if !ok {
			return false, authz.NewAuthzError(
				authz.ErrCodeEnforceFailed,
				"policy did not return a boolean result",
				nil,
			)
		}

		return allowed, nil
	}

	return false, authz.NewAuthzError(
		authz.ErrCodeInitializationFailed,
		"Rego query not prepared",
		nil,
	)
}

// enforceRemote 使用远程OPA服务器执行授权检查
func (a *OPAAuthorizer) enforceRemote(ctx context.Context, input map[string]interface{}) (bool, error) {
	// 构建请求体
	reqBody, err := json.Marshal(map[string]interface{}{
		"input": input,
	})
	if err != nil {
		return false, authz.NewAuthzError(
			authz.ErrCodeEnforceFailed,
			"failed to marshal input",
			err,
		)
	}

	// 创建请求
	url := fmt.Sprintf("%s/allow", a.options.RemoteURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return false, authz.NewAuthzError(
			authz.ErrCodeEnforceFailed,
			"failed to create request",
			err,
		)
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return false, authz.NewAuthzError(
			authz.ErrCodeEnforceFailed,
			"failed to send request",
			err,
		)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, authz.NewAuthzError(
			authz.ErrCodeEnforceFailed,
			fmt.Sprintf("unexpected status code: %d - %s", resp.StatusCode, string(body)),
			nil,
		)
	}

	// 解析响应
	var result struct {
		Result bool `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return false, authz.NewAuthzError(
			authz.ErrCodeEnforceFailed,
			"failed to decode response",
			err,
		)
	}

	return result.Result, nil
}

// BatchEnforce 批量执行授权检查
func (a *OPAAuthorizer) BatchEnforce(ctx context.Context, subjects []authz.Subject, objects []authz.Object, actions []authz.Action, domains []authz.Domain) ([]bool, error) {
	// 检查参数长度一致性
	if len(subjects) != len(objects) || len(subjects) != len(actions) || len(subjects) != len(domains) {
		return nil, authz.NewAuthzError(
			authz.ErrCodeBatchEnforceFailed,
			"subjects, objects, actions, and domains must have the same length",
			nil,
		)
	}

	// 构建请求
	requests := make([]map[string]interface{}, len(subjects))
	for i := range subjects {
		requests[i] = map[string]interface{}{
			"subject": string(subjects[i]),
			"object":  string(objects[i]),
			"action":  string(actions[i]),
			"domain":  string(domains[i]),
		}
	}

	results := make([]bool, len(requests))

	// 根据模式执行批量授权检查
	if a.options.Mode == authz.ModeLocal {
		// 本地模式：批量处理
		if a.preparedQuery != nil {
			// 批量执行查询
			for i, input := range requests {
				allowed, err := a.enforceLocal(ctx, input)
				if err != nil {
					return nil, err
				}
				results[i] = allowed
			}
			return results, nil
		}
		return nil, authz.NewAuthzError(
			authz.ErrCodeInitializationFailed,
			"Rego query not prepared for batch operation",
			nil,
		)
	} else {
		// 远程模式：批量处理
		// 构建请求体
		reqBody, err := json.Marshal(map[string]interface{}{
			"input": requests,
		})
		if err != nil {
			return nil, authz.NewAuthzError(
				authz.ErrCodeBatchEnforceFailed,
				"failed to marshal batch input",
				err,
			)
		}

		// 创建请求
		url := fmt.Sprintf("%s/batch", a.options.RemoteURL)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
		if err != nil {
			return nil, authz.NewAuthzError(
				authz.ErrCodeBatchEnforceFailed,
				"failed to create batch request",
				err,
			)
		}

		req.Header.Set("Content-Type", "application/json")

		// 发送请求
		resp, err := a.httpClient.Do(req)
		if err != nil {
			return nil, authz.NewAuthzError(
				authz.ErrCodeBatchEnforceFailed,
				"failed to send batch request",
				err,
			)
		}
		defer resp.Body.Close()

		// 检查响应状态
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return nil, authz.NewAuthzError(
				authz.ErrCodeBatchEnforceFailed,
				fmt.Sprintf("unexpected status code: %d - %s", resp.StatusCode, string(body)),
				nil,
			)
		}

		// 解析响应
		var result struct {
			Results []bool `json:"results"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, authz.NewAuthzError(
				authz.ErrCodeBatchEnforceFailed,
				"failed to decode batch response",
				err,
			)
		}

		return result.Results, nil
	}
}

// AddPolicy 添加策略
func (a *OPAAuthorizer) AddPolicy(ctx context.Context, policy authz.Policy) (bool, error) {
	return a.updatePolicy(ctx, "add", policy)
}

// RemovePolicy 移除策略
func (a *OPAAuthorizer) RemovePolicy(ctx context.Context, policy authz.Policy) (bool, error) {
	return a.updatePolicy(ctx, "remove", policy)
}

// updatePolicy 更新策略
func (a *OPAAuthorizer) updatePolicy(ctx context.Context, operation string, policy authz.Policy) (bool, error) {
	// 本地模式不支持动态更新策略
	if a.options.Mode == authz.ModeLocal {
		return false, authz.NewAuthzError(
			authz.ErrCodeInvalidConfiguration,
			"policy updates not supported in local mode",
			nil,
		)
	}

	// 构建策略更新
	policyUpdate := map[string]interface{}{
		"operation": operation,
		"policy": map[string]interface{}{
			"subject": string(policy.Subject),
			"object":  string(policy.Object),
			"action":  string(policy.Action),
			"domain":  string(policy.Domain),
			"effect":  string(policy.Effect),
		},
	}

	// 构建请求体
	reqBody, err := json.Marshal(policyUpdate)
	if err != nil {
		return false, authz.NewAuthzError(
			authz.ErrCodeUpdatePoliciesFailed,
			"failed to marshal policy update",
			err,
		)
	}

	// 创建请求
	url := fmt.Sprintf("%s/policies", a.options.RemoteURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return false, authz.NewAuthzError(
			authz.ErrCodeUpdatePoliciesFailed,
			"failed to create policy update request",
			err,
		)
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return false, authz.NewAuthzError(
			authz.ErrCodeUpdatePoliciesFailed,
			"failed to send policy update request",
			err,
		)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return false, authz.NewAuthzError(
			authz.ErrCodeUpdatePoliciesFailed,
			fmt.Sprintf("unexpected status code: %d - %s", resp.StatusCode, string(body)),
			nil,
		)
	}

	return true, nil
}

// AddPolicies 批量添加策略
func (a *OPAAuthorizer) AddPolicies(ctx context.Context, policies []authz.Policy) (bool, error) {
	err := a.updatePolicies(ctx, "add", policies)
	if err != nil {
		return false, err
	}
	return true, nil
}

// RemovePolicies 批量移除策略
func (a *OPAAuthorizer) RemovePolicies(ctx context.Context, policies []authz.Policy) (bool, error) {
	err := a.updatePolicies(ctx, "remove", policies)
	if err != nil {
		return false, err
	}
	return true, nil
}

// updatePolicies 批量更新策略
func (a *OPAAuthorizer) updatePolicies(ctx context.Context, operation string, policies []authz.Policy) error {
	// 本地模式不支持动态更新策略
	if a.options.Mode == authz.ModeLocal {
		return authz.NewAuthzError(authz.ErrCodeInvalidConfiguration, "policy updates not supported in local mode", nil)
	}

	// 构建批量策略更新
	policyUpdates := make([]map[string]interface{}, len(policies))
	for i, policy := range policies {
		policyUpdates[i] = map[string]interface{}{
			"subject": string(policy.Subject),
			"object":  string(policy.Object),
			"action":  string(policy.Action),
			"domain":  string(policy.Domain),
		}
	}

	// 构建请求体
	reqBody, err := json.Marshal(map[string]interface{}{
		"operation": operation,
		"policies":  policyUpdates,
	})
	if err != nil {
		return authz.NewAuthzError(authz.ErrCodeUpdatePoliciesFailed, fmt.Sprintf("failed to marshal batch policy update: %v", err), nil)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", a.opaURL+a.policyPath+"/batch", bytes.NewBuffer(reqBody))
	if err != nil {
		return authz.NewAuthzError(authz.ErrCodeUpdatePoliciesFailed, fmt.Sprintf("failed to create batch policy update request: %v", err), nil)
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return authz.NewAuthzError(authz.ErrCodeUpdatePoliciesFailed, fmt.Sprintf("failed to send batch policy update request: %v", err), nil)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return authz.NewAuthzError(authz.ErrCodeUpdatePoliciesFailed, fmt.Sprintf("OPA server returned error for batch policy update: %s - %s", resp.Status, string(body)), nil)
	}

	return nil
}

// GetAllSubjects 获取所有主体
func (a *OPAAuthorizer) GetAllSubjects(ctx context.Context) ([]authz.Subject, error) {
	entities, err := a.getEntities(ctx, "subjects", "")
	if err != nil {
		return nil, err
	}

	// 转换为授权操作类型
	result := make([]authz.Subject, len(entities))
	for i, act := range entities {
		result[i] = authz.Subject(act)
	}
	return result, nil
}

// GetAllObjects 获取所有对象
func (a *OPAAuthorizer) GetAllObjects(ctx context.Context) ([]authz.Object, error) {
	entities, err := a.getEntities(ctx, "objects", "")
	if err != nil {
		return nil, err
	}

	// 转换为授权操作类型
	result := make([]authz.Object, len(entities))
	for i, act := range entities {
		result[i] = authz.Object(act)
	}
	return result, nil
}

// GetAllActions 获取所有操作
func (a *OPAAuthorizer) GetAllActions(ctx context.Context) ([]authz.Action, error) {
	entities, err := a.getEntities(ctx, "actions", "")
	if err != nil {
		return nil, err
	}

	// 转换为授权操作类型
	result := make([]authz.Action, len(entities))
	for i, act := range entities {
		result[i] = authz.Action(act)
	}
	return result, nil
}

// GetAllDomains 获取所有域
func (a *OPAAuthorizer) GetAllDomains(ctx context.Context) ([]authz.Domain, error) {
	entities, err := a.getEntities(ctx, "domains", "")
	if err != nil {
		return nil, err
	}

	// 转换为授权操作类型
	result := make([]authz.Domain, len(entities))
	for i, act := range entities {
		result[i] = authz.Domain(act)
	}
	return result, nil
}

// getEntities 获取实体
func (a *OPAAuthorizer) getEntities(ctx context.Context, entityType string, domain authz.Domain) ([]string, error) {
	// 本地模式不支持获取实体
	if a.options.Mode == authz.ModeLocal {
		return nil, authz.NewAuthzError(authz.ErrCodeInvalidConfiguration, fmt.Sprintf("getting %s not supported in local mode", entityType), nil)
	}

	// 构建请求参数
	params := map[string]interface{}{
		"entity_type": entityType,
	}
	if domain != "" {
		params["domain"] = string(domain)
	}

	// 构建请求体
	reqBody, err := json.Marshal(params)
	if err != nil {
		return nil, authz.NewAuthzError(authz.ErrCodeEnforceFailed, fmt.Sprintf("failed to marshal entity query: %v", err), nil)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", a.opaURL+"/v1/data/authz/entities", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, authz.NewAuthzError(authz.ErrCodeEnforceFailed, fmt.Sprintf("failed to create entity query request: %v", err), nil)
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, authz.NewAuthzError(authz.ErrCodeEnforceFailed, fmt.Sprintf("failed to send entity query request: %v", err), nil)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, authz.NewAuthzError(authz.ErrCodeEnforceFailed, fmt.Sprintf("OPA server returned error for entity query: %s - %s", resp.Status, string(body)), nil)
	}

	// 解析响应
	var result struct {
		Result []string `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, authz.NewAuthzError(authz.ErrCodeEnforceFailed, fmt.Sprintf("failed to decode entity query response: %v", err), nil)
	}

	return result.Result, nil
}

// GetPolicies 获取策略
func (a *OPAAuthorizer) GetPolicies(ctx context.Context, domain authz.Domain) ([]authz.Policy, error) {
	// 本地模式不支持获取策略
	if a.options.Mode == authz.ModeLocal {
		return nil, authz.NewAuthzError(
			authz.ErrCodeInvalidConfiguration,
			"getting policies not supported in local mode",
			nil,
		)
	}

	// 构建请求参数
	params := map[string]interface{}{}
	if domain != "" {
		params["domain"] = string(domain)
	}

	// 构建请求体
	reqBody, err := json.Marshal(params)
	if err != nil {
		return nil, authz.NewAuthzError(
			authz.ErrCodeEnforceFailed,
			"failed to marshal policy query",
			err,
		)
	}

	// 创建请求
	url := fmt.Sprintf("%s/policies", a.options.RemoteURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, authz.NewAuthzError(
			authz.ErrCodeEnforceFailed,
			"failed to create policy query request",
			err,
		)
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, authz.NewAuthzError(
			authz.ErrCodeEnforceFailed,
			"failed to send policy query request",
			err,
		)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, authz.NewAuthzError(
			authz.ErrCodeEnforceFailed,
			fmt.Sprintf("unexpected status code: %d - %s", resp.StatusCode, string(body)),
			nil,
		)
	}

	// 解析响应
	var result struct {
		Result []struct {
			Subject string `json:"subject"`
			Object  string `json:"object"`
			Action  string `json:"action"`
			Domain  string `json:"domain"`
		} `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, authz.NewAuthzError(authz.ErrCodeEnforceFailed, fmt.Sprintf("failed to decode policy query response: %v", err), nil)
	}

	// 转换为策略
	policies := make([]authz.Policy, len(result.Result))
	for i, p := range result.Result {
		policies[i] = authz.Policy{
			Subject: authz.Subject(p.Subject),
			Object:  authz.Object(p.Object),
			Action:  authz.Action(p.Action),
			Domain:  authz.Domain(p.Domain),
		}
	}

	return policies, nil
}

// AddRoleForUser 为用户添加角色
func (a *OPAAuthorizer) AddRoleForUser(ctx context.Context, user authz.Subject, role authz.Subject, domain authz.Domain) (bool, error) {
	err := a.updateRoleForUser(ctx, "add", user, role, domain)
	if err != nil {
		return false, err
	}
	return true, nil
}

// RemoveRoleForUser 移除用户的角色
func (a *OPAAuthorizer) RemoveRoleForUser(ctx context.Context, user authz.Subject, role authz.Subject, domain authz.Domain) (bool, error) {
	err := a.updateRoleForUser(ctx, "remove", user, role, domain)
	if err != nil {
		return false, err
	}
	return true, nil
}

// DeleteRoleForUser 删除用户的角色
func (a *OPAAuthorizer) DeleteRoleForUser(ctx context.Context, user authz.Subject, role authz.Subject, domain authz.Domain) (bool, error) {
	// 直接调用 RemoveRoleForUser 实现相同功能
	return a.RemoveRoleForUser(ctx, user, role, domain)
}

// updateRoleForUser 更新用户角色
func (a *OPAAuthorizer) updateRoleForUser(ctx context.Context, operation string, user authz.Subject, role authz.Subject, domain authz.Domain) error {
	// 本地模式不支持动态更新角色
	if a.options.Mode == authz.ModeLocal {
		return authz.NewAuthzError(
			authz.ErrCodeInvalidConfiguration,
			"role updates not supported in local mode",
			nil,
		)
	}

	// 构建角色更新
	roleUpdate := map[string]interface{}{
		"operation": operation,
		"role": map[string]interface{}{
			"user":   string(user),
			"role":   string(role),
			"domain": string(domain),
		},
	}

	// 构建请求体
	reqBody, err := json.Marshal(roleUpdate)
	if err != nil {
		return authz.NewAuthzError(
			authz.ErrCodeUpdateRolesForUserFailed,
			"failed to marshal role update",
			err,
		)
	}

	// 创建请求
	url := fmt.Sprintf("%s/roles", a.options.RemoteURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return authz.NewAuthzError(
			authz.ErrCodeUpdateRolesForUserFailed,
			"failed to create role update request",
			err,
		)
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return authz.NewAuthzError(
			authz.ErrCodeUpdateRolesForUserFailed,
			"failed to send role update request",
			err,
		)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return authz.NewAuthzError(
			authz.ErrCodeUpdateRolesForUserFailed,
			fmt.Sprintf("unexpected status code: %d - %s", resp.StatusCode, string(body)),
			nil,
		)
	}

	return nil
}

// GetRolesForUser 获取用户的角色
func (a *OPAAuthorizer) GetRolesForUser(ctx context.Context, user authz.Subject, domain authz.Domain) ([]authz.Subject, error) {
	// 本地模式不支持获取角色
	if a.options.Mode == authz.ModeLocal {
		return nil, authz.NewAuthzError(authz.ErrCodeInvalidConfiguration, "getting roles not supported in local mode", nil)
	}

	// 构建请求参数
	params := map[string]interface{}{
		"user":   string(user),
		"domain": string(domain),
	}

	// 构建请求体
	reqBody, err := json.Marshal(params)
	if err != nil {
		return nil, authz.NewAuthzError(authz.ErrCodeEnforceFailed, fmt.Sprintf("failed to marshal role query: %v", err), nil)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", a.opaURL+"/v1/data/authz/user_roles", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, authz.NewAuthzError(authz.ErrCodeEnforceFailed, fmt.Sprintf("failed to create role query request: %v", err), nil)
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, authz.NewAuthzError(authz.ErrCodeEnforceFailed, fmt.Sprintf("failed to send role query request: %v", err), nil)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, authz.NewAuthzError(authz.ErrCodeEnforceFailed, fmt.Sprintf("OPA server returned error for role query: %s - %s", resp.Status, string(body)), nil)
	}

	// 解析响应
	var result struct {
		Result []string `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, authz.NewAuthzError(authz.ErrCodeEnforceFailed, fmt.Sprintf("failed to decode role query response: %v", err), nil)
	}

	// 转换为角色
	roles := make([]authz.Subject, len(result.Result))
	for i, r := range result.Result {
		roles[i] = authz.Subject(r)
	}

	return roles, nil
}

// GetUsersForRole 获取具有特定角色的用户
func (a *OPAAuthorizer) GetUsersForRole(ctx context.Context, role authz.Subject, domain authz.Domain) ([]authz.Subject, error) {
	// 本地模式不支持获取用户
	if a.options.Mode == authz.ModeLocal {
		return nil, authz.NewAuthzError(authz.ErrCodeInvalidConfiguration, "getting users not supported in local mode", nil)
	}

	// 构建请求参数
	params := map[string]interface{}{
		"role":   string(role),
		"domain": string(domain),
	}

	// 构建请求体
	reqBody, err := json.Marshal(params)
	if err != nil {
		return nil, authz.NewAuthzError(authz.ErrCodeEnforceFailed, fmt.Sprintf("failed to marshal user query: %v", err), nil)
	}

	// 创建请求
	req, err := http.NewRequestWithContext(ctx, "POST", a.opaURL+"/v1/data/authz/role_users", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, authz.NewAuthzError(authz.ErrCodeEnforceFailed, fmt.Sprintf("failed to create user query request: %v", err), nil)
	}

	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, authz.NewAuthzError(authz.ErrCodeEnforceFailed, fmt.Sprintf("failed to send user query request: %v", err), nil)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, authz.NewAuthzError(authz.ErrCodeEnforceFailed, fmt.Sprintf("OPA server returned error for user query: %s - %s", resp.Status, string(body)), nil)
	}

	// 解析响应
	var result struct {
		Result []string `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, authz.NewAuthzError(authz.ErrCodeEnforceFailed, fmt.Sprintf("failed to decode user query response: %v", err), nil)
	}

	// 转换为用户
	users := make([]authz.Subject, len(result.Result))
	for i, u := range result.Result {
		users[i] = authz.Subject(u)
	}

	return users, nil
}

// GetAllRoles 获取所有角色
func (a *OPAAuthorizer) GetAllRoles(ctx context.Context) ([]authz.Subject, error) {
	if a.options.Mode == authz.ModeLocal {
		return nil, authz.NewAuthzError(
			authz.ErrCodeInvalidConfiguration,
			"GetAllRoles is not supported in local mode",
			nil,
		)
	}

	// 远程模式下，通过HTTP请求获取所有角色
	url := fmt.Sprintf("%s/roles", a.options.RemoteURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, authz.NewAuthzError(
			authz.ErrCodeGetAllRolesFailed,
			"failed to create request",
			err,
		)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, authz.NewAuthzError(
			authz.ErrCodeGetAllRolesFailed,
			"failed to send request",
			err,
		)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return nil, authz.NewAuthzError(
			authz.ErrCodeGetAllRolesFailed,
			fmt.Sprintf("unexpected status code: %d", resp.StatusCode),
			nil,
		)
	}

	// 解析响应
	var result struct {
		Roles []string `json:"roles"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, authz.NewAuthzError(
			authz.ErrCodeGetAllRolesFailed,
			"failed to decode response",
			err,
		)
	}

	// 转换为授权主体类型
	roles := make([]authz.Subject, len(result.Roles))
	for i, role := range result.Roles {
		roles[i] = authz.Subject(role)
	}

	return roles, nil
}

// HasRoleForUser 检查用户是否拥有角色
func (a *OPAAuthorizer) HasRoleForUser(ctx context.Context, user authz.Subject, role authz.Subject, domain authz.Domain) (bool, error) {
	if a.options.Mode == authz.ModeLocal {
		return false, authz.NewAuthzError(
			authz.ErrCodeInvalidConfiguration,
			"HasRoleForUser is not supported in local mode",
			nil,
		)
	}

	// 获取用户的所有角色
	roles, err := a.GetRolesForUser(ctx, user, domain)
	if err != nil {
		return false, authz.NewAuthzError(
			authz.ErrCodeHasRoleForUserFailed,
			"failed to get roles for user",
			err,
		)
	}

	// 检查是否包含指定角色
	for _, r := range roles {
		if r == role {
			return true, nil
		}
	}

	return false, nil
}

// Close 关闭授权器，释放资源
func (a *OPAAuthorizer) Close() error {
	// 清理资源
	if a.options.Mode == authz.ModeLocal && a.preparedQuery != nil {
		// 本地模式下，清理Rego实例
		a.preparedQuery = nil
	}

	// 清理HTTP客户端
	if a.httpClient != nil {
		// 关闭所有空闲连接
		a.httpClient.CloseIdleConnections()
	}

	return nil
}
