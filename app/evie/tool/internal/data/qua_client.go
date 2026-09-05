// Package data · qua_client.go
// QuaFetcher：薄包装 evie/tool/pkg/source/qua.Source。
//
// 本文件保留：
//   1. QuaFetcher 接口（health check + QuaVocabularySource 调用方依赖）
//   2. QuaClientOption / WithHTTPClient / WithTimeout（保持 wire 兼容）
//   3. NewQuaClient：根据 conf.Qua 构造 QuaFetcher，内部委托 evie/tool/pkg/source/qua
//   4. NewQuaClientOptions：返回空的 options 列表（wire provider）
//
// QuaFetcher 实现的 HTTP 行为（必须与旧实现一致）：
//   - Authorization: Bearer <ctx.AuthInfo.AccessToken>
//   - tenant-id: <ctx.AuthInfo.TenantID as int>
//   - selectAll=true 加到 user 接口的 query string
//   - qua 业务错误码 (400/401/403/404/500) → v1 kratos errors
//
// 新代码应直接用 evie/tool/pkg/source/qua + evie/tool/pkg/source/adapter。
package data

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	v1 "backend-service/api/evie/tool/v1"

	"backend-service/app/evie/tool/internal/conf"
	httpsrc "backend-service/app/evie/tool/pkg/source/http"
	quasrc "backend-service/app/evie/tool/pkg/source/qua"
)

// QuaFetcher 是 qua 系统的 opaque fetcher 接口。
type QuaFetcher interface {
	FetchUsersRaw(ctx context.Context) ([]map[string]any, error)
	FetchDeptsRaw(ctx context.Context) ([]map[string]any, error)
	BaseURL() string
	Ping(ctx context.Context) error
}

// quaFetcher 实现 QuaFetcher：内部委托 evie/tool/pkg/source/qua.Source。
type quaFetcher struct {
	src       *quasrc.Source
	rawSource *httpsrc.Source // 直接访问底层 HTTP source（用于 ctx-aware tenant header）
	baseURL   string
}

// QuaClientOption qua 配置函数（保留签名）。
type QuaClientOption func(*quaOptions)

type quaOptions struct {
	httpClient *http.Client
	timeout    time.Duration
}

// WithHTTPClient 注入自定义 *http.Client。
func WithHTTPClient(hc *http.Client) QuaClientOption {
	return func(o *quaOptions) { o.httpClient = hc }
}

// WithTimeout 覆盖默认超时。
func WithTimeout(d time.Duration) QuaClientOption {
	return func(o *quaOptions) { o.timeout = d }
}

// NewQuaClient 根据 conf.Qua 构造 QuaFetcher（委托给 evie/tool/pkg/source/qua）。
func NewQuaClient(c *conf.Qua, _ log.Logger, opts ...QuaClientOption) (QuaFetcher, error) {
	if c == nil {
		return nil, v1.ErrorInternalError("qua config is required")
	}
	if c.GetBaseUrl() == "" {
		return nil, v1.ErrorInternalError("qua.base_url is required")
	}

	o := &quaOptions{}
	for _, opt := range opts {
		opt(o)
	}

	endpoints := c.GetEndpoints()
	if endpoints.GetListUsers() == "" {
		return nil, v1.ErrorInternalError("qua.endpoints.list_users is required")
	}
	if endpoints.GetListDepts() == "" {
		return nil, v1.ErrorInternalError("qua.endpoints.list_depts is required")
	}

	headers := c.GetExtraHeaders()
	tenantHeader := c.GetTenantHeader()
	if tenantHeader == "" {
		tenantHeader = "tenant-id" // qua 默认 header
	}
	timeout := o.timeout
	if timeout == 0 {
		if c.GetTimeout() != nil {
			timeout = c.GetTimeout().AsDuration()
		} else {
			timeout = 5 * time.Second
		}
	}
	httpClient := o.httpClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: timeout}
	}

	// 业务错误码 → v1.ErrorXxx 映射。
	codeErrs := map[int]func(code int, msg string) error{
		400: func(_ int, msg string) error { return v1.ErrorQuaBadRequest("%s", msg) },
		401: func(_ int, msg string) error { return v1.ErrorQuaUnauthorized("%s", msg) },
		403: func(_ int, msg string) error { return v1.ErrorQuaForbidden("%s", msg) },
		404: func(_ int, msg string) error { return v1.ErrorQuaNotFound("%s", msg) },
		500: func(_ int, msg string) error { return v1.ErrorQuaInternalError("%s", msg) },
	}

	rawSrc, err := httpsrc.New(httpsrc.Config{
		BaseURL:        c.GetBaseUrl(),
		UserPath:       endpoints.GetListUsers(),
		DeptPath:       endpoints.GetListDepts(),
		QueryParams:    map[string]string{"selectAll": "true"},
		TokenProvider:  quaTokenProvider{}, // from ctx
		Headers:        headers,
		TenantHeader:   tenantHeader,
		TenantIDProvider: quaTenantProvider{}, // from ctx,转 int
		HTTPClient:     httpClient,
		Timeout:        timeout,
		Envelope: httpsrc.Envelope{
			UsersPath: "data.list",
			DeptsPath: "data",
			CodePath:  "code",
			CodeOK:    0,
		},
		CodeErrorMap: codeErrs,
	})
	if err != nil {
		return nil, fmt.Errorf("qua: build source: %w", err)
	}
	src, err := quasrc.New(quasrc.Config{
		BaseURL:     c.GetBaseUrl(),
		UserPath:    endpoints.GetListUsers(),
		DeptPath:    endpoints.GetListDepts(),
		Headers:     headers,
		TenantHeader: tenantHeader,
	})
	if err != nil {
		return nil, fmt.Errorf("qua: build source: %w", err)
	}
	_ = src // quaVocabularySource 使用 rawSrc 路径
	return &quaFetcher{src: src, rawSource: rawSrc, baseURL: c.GetBaseUrl()}, nil
}

// BaseURL 返回 baseURL（健康检查用）。
func (q *quaFetcher) BaseURL() string { return q.baseURL }

// Ping 探测 qua 可达性（HEAD 请求 baseURL）。
func (q *quaFetcher) Ping(ctx context.Context) error {
	if q == nil || q.baseURL == "" {
		return errors.New("qua: not configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, q.baseURL, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return fmt.Errorf("qua: status %d", resp.StatusCode)
	}
	return nil
}

// FetchUsersRaw 拉取用户 opaque 列表。
func (q *quaFetcher) FetchUsersRaw(ctx context.Context) ([]map[string]any, error) {
	return q.fetchByType(ctx, quasrc.UserEntityType)
}

// FetchDeptsRaw 拉取部门 opaque 列表。
func (q *quaFetcher) FetchDeptsRaw(ctx context.Context) ([]map[string]any, error) {
	return q.fetchByType(ctx, quasrc.DepartmentEntityType)
}

// fetchByType 直接用底层 httpsrc 拉指定 entity_type。
//
// 错误包装：
//   - 网络层错误 → v1.ErrorQuaUnreachable
//   - JSON 解析错误 → v1.ErrorQuaInvalidResponse
//   - 其他 → 透传
func (q *quaFetcher) fetchByType(ctx context.Context, typ string) ([]map[string]any, error) {
	var path string
	switch typ {
	case quasrc.UserEntityType:
		path = q.rawSource.UserPath()
	case quasrc.DepartmentEntityType:
		path = q.rawSource.DeptPath()
	default:
		return nil, fmt.Errorf("qua: unknown entity_type %q", typ)
	}
	entities, err := q.rawSource.FetchWithCtx(ctx, path)
	if err != nil {
		return nil, wrapHTTPError(err)
	}
	return entities, nil
}

// wrapHTTPError 把 evie/tool/evie/tool/pkg/source 的通用 error 映射到 qua 特定 v1 error。
func wrapHTTPError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "http call:"):
		return v1.ErrorQuaUnreachable("%s", msg)
	case strings.Contains(msg, "decode root:") || strings.Contains(msg, "decode payload:"):
		return v1.ErrorQuaInvalidResponse("%s", msg)
	default:
		return err
	}
}

// --- ctx-aware providers ---

// quaTokenProvider 从 ctx 取 access token（透传给 qua）。
type quaTokenProvider struct{}

func (quaTokenProvider) Token(ctx context.Context) (string, error) {
	info, ok := AuthInfoFromContext(ctx)
	if !ok {
		return "", v1.ErrorTokenMissing("qua call requires auth context")
	}
	if info.AccessToken == "" {
		return "", v1.ErrorTokenPayloadInvalid("qua call requires accessToken in AuthInfo")
	}
	return info.AccessToken, nil
}

// quaTenantProvider 从 ctx 取 tenant id 并转为数字字符串（qua 端要求）。
type quaTenantProvider struct{}

func (quaTenantProvider) TenantID(ctx context.Context) (string, error) {
	info, ok := AuthInfoFromContext(ctx)
	if !ok {
		return "", v1.ErrorTokenMissing("qua call requires auth context")
	}
	if info.TenantID == "" {
		return "", v1.ErrorTokenPayloadInvalid("qua call requires tenantId in AuthInfo")
	}
	if n, err := strconv.ParseInt(info.TenantID, 10, 64); err == nil {
		return strconv.FormatInt(n, 10), nil
	}
	return info.TenantID, nil // 非数字透传
}
