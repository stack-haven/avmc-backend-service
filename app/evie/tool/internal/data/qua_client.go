// Package data · qua_client.go
// quaClient：外部系统 HTTP fetcher，仅返回 opaque `[]map[string]any`。
//
// 设计原则（Q13 决定）：
//   1. fetcher 不解释业务字段语义
//   2. opaque payload 由 VocabularySource adapter（qua_source.go）包成 RawEntity
//   3. 字段映射由 Normalizer + YAML 规则解释
//   4. 加新 source = 新 fetcher + 新 adapter + 新规则 YAML；零核心代码变更
//
// 协议约定（Q1/Q5/Q6 + 真实联调样本）：
//   1. 调用方从 ctx 取 AuthInfo（data.AuthInfoFromContext）
//   2. 透传 authorization: Bearer <token> 头
//   3. 透传 tenant-id: <int> 头（qua 端要求数字）
//   4. 静态透传 qua.extra_headers（如 zone）
//   5. 部门： GET {baseURL}/admin-api/system/dept/list
//   6. 用户： GET {baseURL}/admin-api/qua/member-extended/page?selectAll=true
package data

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	v1 "backend-service/api/evie/tool/v1"
	"backend-service/app/evie/tool/internal/conf"
)

// quaClient 实现 qua 系统的 opaque HTTP fetcher。
type quaClient struct {
	baseURL    string
	httpClient *http.Client
	endpoints  quaEndpoints
	timeout    time.Duration
	extraHdr   map[string]string
}

// BaseURL 返回 baseURL（健康检查用）。
func (c *quaClient) BaseURL() string { return c.baseURL }

// Ping 探测 qua 服务可达性（M9 健康检查用）。
//
// 实施：HEAD 请求 baseURL；qua 端 404 也算可达（说明路由生效）。
func (c *quaClient) Ping(ctx context.Context) error {
	if c == nil || c.baseURL == "" {
		return errors.New("qua: not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, c.baseURL, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("qua: status %d", resp.StatusCode)
	}
	return nil
}

// quaEndpoints 镜像 conf.Qua.Endpoints。
type quaEndpoints struct {
	ListUsers string
	ListDepts string
}

// QuaClientOption quaClient 配置函数（便于测试时覆盖 baseURL / timeout）。
type QuaClientOption func(*quaClient)

// WithHTTPClient 注入自定义 *http.Client（测试 / tracing middleware 时用）。
func WithHTTPClient(hc *http.Client) QuaClientOption {
	return func(c *quaClient) { c.httpClient = hc }
}

// WithTimeout 覆盖默认超时。
func WithTimeout(d time.Duration) QuaClientOption {
	return func(c *quaClient) { c.timeout = d }
}

// NewQuaClient 构造 qua HTTP fetcher。
//
//   c:      conf.Qua 配置
//   logger: kratos logger（保留入参；M5+ 阶段用于错误日志）
//   opts:   测试或定制选项
func NewQuaClient(c *conf.Qua, _ log.Logger, opts ...QuaClientOption) (QuaFetcher, error) {
	if c == nil {
		return nil, v1.ErrorInternalError("qua config is required")
	}
	if c.GetBaseUrl() == "" {
		return nil, v1.ErrorInternalError("qua.base_url is required")
	}
	timeout := 5 * time.Second
	if c.GetTimeout() != nil {
		timeout = c.GetTimeout().AsDuration()
	}
	qc := &quaClient{
		baseURL:    strings.TrimRight(c.GetBaseUrl(), "/"),
		httpClient: &http.Client{Timeout: timeout},
		timeout:    timeout,
		extraHdr:   c.GetExtraHeaders(),
		endpoints: quaEndpoints{
			ListUsers: c.GetEndpoints().GetListUsers(),
			ListDepts: c.GetEndpoints().GetListDepts(),
		},
	}
	for _, opt := range opts {
		opt(qc)
	}
	if qc.endpoints.ListUsers == "" {
		return nil, v1.ErrorInternalError("qua.endpoints.list_users is required")
	}
	if qc.endpoints.ListDepts == "" {
		return nil, v1.ErrorInternalError("qua.endpoints.list_depts is required")
	}
	return qc, nil
}

// QuaFetcher opaque fetcher 接口（data 层契约）。
//
// 返回值为 opaque map，**不含任何业务类型**；语义解析由
// adapter + Normalizer 处理。
type QuaFetcher interface {
	FetchUsersRaw(ctx context.Context) ([]map[string]any, error)
	FetchDeptsRaw(ctx context.Context) ([]map[string]any, error)
}

// FetchUsersRaw GET {baseURL}/list_users?selectAll=true → opaque []map。
func (c *quaClient) FetchUsersRaw(ctx context.Context) ([]map[string]any, error) {
	auth, err := c.authInfo(ctx)
	if err != nil {
		return nil, err
	}
	endpoint := c.baseURL + c.endpoints.ListUsers
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, v1.ErrorInternalError("parse qua user endpoint: %v", err)
	}
	q := u.Query()
	q.Set("selectAll", "true")
	u.RawQuery = q.Encode()

	// qua 用户接口是 GET（实测：POST 会返回 501 "yudao-module-bdk-qua - 已禁用"）
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, v1.ErrorInternalError("build qua users request: %v", err)
	}
	c.applyCommonHeaders(req, auth)

	var resp quaUsersRawResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return resp.Data.List, nil
}

// FetchDeptsRaw GET {baseURL}/list_depts → opaque []map。
func (c *quaClient) FetchDeptsRaw(ctx context.Context) ([]map[string]any, error) {
	auth, err := c.authInfo(ctx)
	if err != nil {
		return nil, err
	}
	endpoint := c.baseURL + c.endpoints.ListDepts
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, v1.ErrorInternalError("build qua depts request: %v", err)
	}
	c.applyCommonHeaders(req, auth)

	var resp quaDeptsRawResponse
	if err := c.do(req, &resp); err != nil {
		return nil, err
	}
	return resp.Data, nil
}

// authInfo 从 ctx 取 AuthInfo 并校验必要字段。
func (c *quaClient) authInfo(ctx context.Context) (*AuthInfo, error) {
	info, ok := AuthInfoFromContext(ctx)
	if !ok {
		return nil, v1.ErrorTokenMissing("qua call requires auth context")
	}
	if info.AccessToken == "" {
		return nil, v1.ErrorTokenPayloadInvalid("qua call requires accessToken in AuthInfo")
	}
	if info.TenantID == "" {
		return nil, v1.ErrorTokenPayloadInvalid("qua call requires tenantId in AuthInfo")
	}
	return info, nil
}

// applyCommonHeaders 设置 qua 必需 header：authorization / tenant-id / extra_headers。
func (c *quaClient) applyCommonHeaders(req *http.Request, info *AuthInfo) {
	req.Header.Set("Authorization", "Bearer "+info.AccessToken)
	// tenant-id 在 qua 端是数字（bigint → 字符串）
	if tID, err := strconv.ParseInt(info.TenantID, 10, 64); err == nil {
		req.Header.Set("tenant-id", strconv.FormatInt(tID, 10))
	} else {
		// 异常 tenantId 不阻断；qua 端如果强校验则返回 401/403，由 HTTP 错误层处理
		req.Header.Set("tenant-id", info.TenantID)
	}
	for k, v := range c.extraHdr {
		if k == "" {
			continue
		}
		req.Header.Set(k, v)
	}
}

// do 统一处理：HTTP 状态码 → kratos error；body 读取 → JSON 反序列化。
func (c *quaClient) do(req *http.Request, out any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return v1.ErrorQuaUnreachable("qua http call: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return v1.ErrorQuaInvalidResponse("read qua response: %v", err)
	}

	var base quaBaseResponse
	if err := json.Unmarshal(body, &base); err != nil {
		return v1.ErrorQuaInvalidResponse("decode qua base response: %v (body=%s)", err, string(body))
	}
	if base.Code != 0 {
		return mapQuaBusinessError(base.Code, base.Msg)
	}
	if out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return v1.ErrorQuaInvalidResponse("decode qua payload: %v (body=%s)", err, string(body))
		}
	}
	return nil
}

// mapQuaBusinessError 将 qua 业务码映射为本工具错误码。
// qua 业务码约定（基于常见 Spring Cloud 模板）：
//   0    = ok
//   400  = BAD_REQUEST
//   401  = UNAUTHORIZED
//   403  = FORBIDDEN
//   404  = NOT_FOUND
//   500  = INTERNAL
//   其它  = INTERNAL_ERROR（qua 内部错误）
func mapQuaBusinessError(code int32, msg string) error {
	switch code {
	case 400:
		return v1.ErrorQuaBadRequest("%s", msg)
	case 401:
		return v1.ErrorQuaUnauthorized("%s", msg)
	case 403:
		return v1.ErrorQuaForbidden("%s", msg)
	case 404:
		return v1.ErrorQuaNotFound("%s", msg)
	case 500:
		return v1.ErrorQuaInternalError("%s", msg)
	default:
		return v1.ErrorQuaInternalError("qua code=%d msg=%s", code, msg)
	}
}

// ============================================================================
// qua opaque 响应模型（私有）
// ============================================================================

// quaBaseResponse qua 通用响应外壳。
type quaBaseResponse struct {
	Code int32  `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data"`
}

// quaUsersRawResponse qua users 接口响应（opaque）。
type quaUsersRawResponse struct {
	Code int32           `json:"code"`
	Msg  string          `json:"msg"`
	Data quaUsersRawData `json:"data"`
}

// quaUsersRawData 仅暴露 list 字段为 opaque map。
type quaUsersRawData struct {
	List     []map[string]any `json:"list"`
	Total    int64           `json:"total"`
	PageNo   int32           `json:"pageNo"`
	PageSize int32           `json:"pageSize"`
}

// quaDeptsRawResponse qua depts 接口响应（opaque）。
type quaDeptsRawResponse struct {
	Code int32            `json:"code"`
	Msg  string           `json:"msg"`
	Data []map[string]any `json:"data"`
}

// 编译期断言
var (
	_ QuaFetcher = (*quaClient)(nil)
	_           = fmt.Sprintf
	_           = errors.Is
)