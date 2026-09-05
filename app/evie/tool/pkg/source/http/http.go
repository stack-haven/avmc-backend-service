// Package http provides a generic, configurable vocabulary Source
// adapter for REST APIs returning JSON arrays of opaque entities.
//
// # Use case
//
// Most business systems expose some flavour of:
//   GET {baseURL}/users   → {"code":0, "data":{"list":[...], ...}}
//   GET {baseURL}/depts   → {"code":0, "data":[...]}
//
// HTTPSource makes every part of that exchange configurable: the base
// URL, the two endpoint paths, the entity types emitted, the JSON
// envelope shape, the headers to send, and how the upstream user-id
// is read. The adapter itself never interprets the entity payload
// (the downstream Normalizer does that via YAML rules).
//
// # Example
//
//	s, _ := httpsource.New(httpsource.Config{
//	    BaseURL:  "http://api.example.com",
//	    UserPath: "/admin-api/users?selectAll=true",
//	    DeptPath: "/admin-api/depts",
//	    UserEntityType: "user",
//	    DeptEntityType: "department",
//	    AuthHeader: "Authorization",
//	    TokenProvider: myTokenProvider,   // implements AuthTokenProvider
//	    Headers: map[string]string{"Zone": "Asia/Shanghai"},
//	    IDKey:    "id",
//	})
//	entities, err := s.Fetch(ctx)
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"backend-service/app/evie/tool/pkg/source"
)

// Config configures a generic HTTPSource. Every field is optional
// except BaseURL and at least one of {UserPath, DeptPath}.
type Config struct {
	// BaseURL is the upstream API root (e.g. "http://api.example.com").
	// Trailing slashes are trimmed automatically.
	BaseURL string

	// UserPath / DeptPath are appended to BaseURL when fetching
	// users / departments. Either may be empty (the corresponding
	// entity type is skipped silently).
	UserPath string
	DeptPath string

	// UserEntityType / DeptEntityType are emitted on each
	// RawEntity.EntityType. Defaults to "user" / "department".
	UserEntityType string
	DeptEntityType string

	// Method overrides the HTTP method (default "GET"). Both
	// endpoints use the same method.
	Method string

	// Body overrides the request body (optional). When set, this body
	// is sent with every request regardless of Method.
	Body []byte

	// QueryParams are appended to the request URL on every call
	// (e.g. {"selectAll": "true"}). The user/dept URL builders run
	// before QueryParams are applied, so QueryParams can override
	// path-internal values.
	QueryParams map[string]string

	// AuthHeader is the HTTP header used to carry the bearer token
	// (default "Authorization"). TokenProvider provides the value.
	AuthHeader string

	// AuthScheme is the prefix written before the token (default "Bearer ").
	AuthScheme string

	// TokenProvider is called for each Fetch to obtain the bearer
	// token. When nil, no Authorization header is sent.
	TokenProvider AuthTokenProvider

	// StaticToken, when non-empty, overrides TokenProvider and is
	// used verbatim as the Authorization value (still prefixed with
	// AuthScheme). Intended for service-account credentials.
	StaticToken string

	// Headers are sent on every request, in addition to AuthHeader.
	// Useful for static metadata (e.g. "Zone: Asia/Shanghai").
	Headers map[string]string

	// TenantHeader, when non-empty, sends the tenant id under this
	// header name (e.g. "tenant-id").
	//
	// TenantIDProvider is consulted at request time so the value
	// can come from request-scoped state (e.g. AuthContext).
	// When TenantIDProvider is nil, TenantID is sent verbatim.
	TenantHeader     string
	TenantID         string
	TenantIDProvider TenantIDProvider

	// HTTPClient is the underlying client. When nil, a default
	// client with Timeout is created.
	HTTPClient *http.Client

	// Timeout is used when HTTPClient is nil.
	Timeout time.Duration

	// IDKey is the JSON field whose value becomes RawEntity.SourceID
	// (default "id"). IDKey accepts dotted paths (e.g. "user.id").
	IDKey string

	// Envelope describes how to unwrap the response JSON. When nil
	// or empty, the response body is treated as either an object with
	// a top-level list under "data.list", or a top-level array.
	Envelope Envelope

	// CodeErrorMap maps upstream business codes (int) to error
	// factories. When set, a non-OK business code triggers the
	// factory; the returned error is propagated to Fetch callers.
	// Codes not in the map fall through to the default behaviour
	// (returns "upstream code=N" error).
	CodeErrorMap map[int]func(code int, msg string) error
}

// TenantIDProvider returns the tenant id for the current request.
type TenantIDProvider interface {
	TenantID(ctx context.Context) (string, error)
}

// TenantIDFunc adapts a closure to TenantIDProvider.
type TenantIDFunc func(ctx context.Context) (string, error)

// TenantID implements TenantIDProvider.
func (f TenantIDFunc) TenantID(ctx context.Context) (string, error) { return f(ctx) }

// Envelope describes how to find the list of entities in the upstream
// JSON response. All path components are dotted: "data.list" means
// root["data"]["list"].
//
//   - DataPath: top-level list path (e.g. "data.list" or empty for
//     top-level array)
//   - CodePath / CodeOK: optional error envelope. When CodePath is
//     non-empty, the adapter reads root[CodePath]; if it does not
//     equal CodeOK the request is treated as failed.
type Envelope struct {
	UsersPath string
	DeptsPath string
	CodePath  string
	CodeOK    any
}

// AuthTokenProvider returns the bearer token for the current request.
// Implementations MUST be safe for concurrent use.
type AuthTokenProvider interface {
	Token(ctx context.Context) (string, error)
}

// TokenFunc adapts a closure to AuthTokenProvider.
type TokenFunc func(ctx context.Context) (string, error)

// Token implements AuthTokenProvider.
func (f TokenFunc) Token(ctx context.Context) (string, error) { return f(ctx) }

// Source is the generic HTTP vocabulary adapter.
type Source struct {
	cfg    Config
	client *http.Client
}

// UserPath returns the user endpoint path (used by adapters that
// need ctx-aware single-entity fetches).
func (s *Source) UserPath() string { return s.cfg.UserPath }

// DeptPath returns the department endpoint path.
func (s *Source) DeptPath() string { return s.cfg.DeptPath }

// FetchFor returns a one-shot fetch using s.client / s.applyHeaders
// but with a ctx-derived TenantID. Internal use only; exposed to
// adapters that need per-request ctx propagation beyond Token.
func (s *Source) FetchWithCtx(ctx context.Context, path string) ([]map[string]any, error) {
	endpoint := s.applyQueryParams(s.cfg.BaseURL + path)
	var body io.Reader
	if len(s.cfg.Body) > 0 {
		body = bytes.NewReader(s.cfg.Body)
	}
	req, err := http.NewRequestWithContext(ctx, s.cfg.Method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if err := s.applyHeadersWithErrors(req, ctx); err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("upstream %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return parseList(raw, s.cfg.Envelope, resp.StatusCode, s.cfg.CodeErrorMap)
}

// applyQueryParams appends Config.QueryParams to endpoint, preserving
// existing query parameters when present.
func (s *Source) applyQueryParams(endpoint string) string {
	if len(s.cfg.QueryParams) == 0 {
		return endpoint
	}
	if strings.Index(endpoint, "?") >= 0 {
		endpoint += "&"
	} else {
		endpoint += "?"
	}
	first := true
	for k, v := range s.cfg.QueryParams {
		if k == "" {
			continue
		}
		sep := "&"
		if first {
			sep = ""
			first = false
		}
		endpoint += sep + url.QueryEscape(k) + "=" + url.QueryEscape(v)
	}
	return endpoint
}

// applyHeadersWithErrors is like applyHeaders but propagates
// TokenProvider errors so callers see them (e.g. "missing token").
func (s *Source) applyHeadersWithErrors(req *http.Request, ctx context.Context) error {
	// Authorization (errors propagate)
	token := s.cfg.StaticToken
	if token == "" && s.cfg.TokenProvider != nil {
		t, err := s.cfg.TokenProvider.Token(ctx)
		if err != nil {
			return err
		}
		token = t
	}
	if token != "" {
		req.Header.Set(s.cfg.AuthHeader, s.cfg.AuthScheme+token)
	}
	// Static headers
	for k, v := range s.cfg.Headers {
		if k == "" {
			continue
		}
		req.Header.Set(k, v)
	}
	// Tenant
	if s.cfg.TenantHeader != "" {
		tid := s.cfg.TenantID
		if s.cfg.TenantIDProvider != nil {
			if t, err := s.cfg.TenantIDProvider.TenantID(ctx); err == nil && t != "" {
				tid = t
			}
		}
		if tid != "" {
			req.Header.Set(s.cfg.TenantHeader, tid)
		}
	}
	return nil
}

// New constructs a HTTPSource. It does NOT perform any network IO.
func New(cfg Config) (*Source, error) {
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("http: BaseURL required")
	}
	if cfg.UserPath == "" && cfg.DeptPath == "" {
		return nil, fmt.Errorf("http: at least one of UserPath/DeptPath required")
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	if cfg.UserEntityType == "" {
		cfg.UserEntityType = "user"
	}
	if cfg.DeptEntityType == "" {
		cfg.DeptEntityType = "department"
	}
	if cfg.Method == "" {
		cfg.Method = http.MethodGet
	}
	if cfg.AuthHeader == "" {
		cfg.AuthHeader = "Authorization"
	}
	if cfg.AuthScheme == "" {
		cfg.AuthScheme = "Bearer "
	}
	if cfg.IDKey == "" {
		cfg.IDKey = "id"
	}
	if cfg.HTTPClient == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = 10 * time.Second
		}
		cfg.HTTPClient = &http.Client{Timeout: timeout}
	}
	return &Source{cfg: cfg, client: cfg.HTTPClient}, nil
}

// Name implements source.Source.
func (s *Source) Name() string { return "http" }

// Fetch retrieves users and/or departments from the upstream API.
// Partial failures (one endpoint errors, the other succeeds) are
// returned as (data, joinedErr) so the caller can decide whether
// the partial result is still useful.
func (s *Source) Fetch(ctx context.Context) ([]source.RawEntity, error) {
	out := make([]source.RawEntity, 0)
	var errs []error

	if s.cfg.UserPath != "" {
		users, err := s.fetchOne(ctx, s.cfg.UserPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("users: %w", err))
		} else {
			for _, u := range users {
				out = append(out, source.RawEntity{
					SourceID:   extractID(u, s.cfg.IDKey),
					EntityType: s.cfg.UserEntityType,
					Source:     s.Name(),
					Data:       u,
				})
			}
		}
	}
	if s.cfg.DeptPath != "" {
		depts, err := s.fetchOne(ctx, s.cfg.DeptPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("depts: %w", err))
		} else {
			for _, d := range depts {
				out = append(out, source.RawEntity{
					SourceID:   extractID(d, s.cfg.IDKey),
					EntityType: s.cfg.DeptEntityType,
					Source:     s.Name(),
					Data:       d,
				})
			}
		}
	}

	switch {
	case len(errs) == 0:
		return out, nil
	case len(out) == 0:
		return nil, joinErrs(errs)
	default:
		return out, joinErrs(errs)
	}
}

func (s *Source) fetchOne(ctx context.Context, path string) ([]map[string]any, error) {
	endpoint := s.applyQueryParams(s.cfg.BaseURL + path)
	var body io.Reader
	if len(s.cfg.Body) > 0 {
		body = bytes.NewReader(s.cfg.Body)
	}
	req, err := http.NewRequestWithContext(ctx, s.cfg.Method, endpoint, body)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if err := s.applyHeadersWithErrors(req, ctx); err != nil {
		return nil, err
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http call: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("upstream %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	return parseList(raw, s.cfg.Envelope, resp.StatusCode, s.cfg.CodeErrorMap)
}

// applyHeaders sets Authorization (via TokenProvider or StaticToken),
// the static Headers map, and the optional TenantHeader.
//
// Deprecated: use applyHeadersWithErrors which propagates auth errors.
// Kept for backward compatibility with adapters that call applyHeaders
// outside FetchWithCtx.
func (s *Source) applyHeaders(req *http.Request) {
	_ = s.applyHeadersWithErrors(req, req.Context())
}

// parseList extracts a list of opaque maps from the upstream JSON.
//
// Behaviour:
//   - First try the path under Envelope.UsersPath/DeptsPath if set.
//   - Else try envelope "data.list" / "data" as an array.
//   - Else accept a top-level array.
//   - Envelope.CodePath / CodeOK is checked first; non-OK returns error.
//   - CodeErrorMap may intercept non-OK codes and produce a typed
//     error (useful for kratos-error compatibility).
func parseList(body []byte, env Envelope, status int, codeErrs map[int]func(code int, msg string) error) ([]map[string]any, error) {
	if len(body) == 0 {
		return nil, nil
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		// Some upstreams return a top-level array.
		var arr []map[string]any
		if err2 := json.Unmarshal(body, &arr); err2 != nil {
			return nil, fmt.Errorf("decode root: %v (body=%s)", err, bodyPreview(body))
		}
		return arr, nil
	}
	// Code envelope check.
	if env.CodePath != "" {
		if v, ok := lookupPath(root, env.CodePath); ok {
			if !codeEquals(v, env.CodeOK) {
				if codeErrs != nil {
					if n, ok := toInt(v); ok {
						if factory, hit := codeErrs[n]; hit {
							msg := stringOf(lookupPathOr(root, "msg", ""))
							return nil, factory(n, msg)
						}
					}
				}
				return nil, fmt.Errorf("upstream code=%v", v)
			}
		}
	}
	// Try the configured UsersPath / DeptsPath (caller picks one).
	// We just look for any of these; the path was already chosen by
	// the caller (UserPath vs DeptPath). To keep parsing single-path,
	// we look for `data.list` first, then `data`.
	if v, ok := lookupPath(root, "data.list"); ok {
		return toMapList(v)
	}
	if v, ok := lookupPath(root, "data"); ok {
		// data could be an array or an object containing "list"
		if arr, ok := v.([]any); ok {
			return toMapList(arr)
		}
		if m, ok := v.(map[string]any); ok {
			if l, ok := m["list"]; ok {
				return toMapList(l)
			}
		}
	}
	// Fallback: top-level list with key "list"
	if v, ok := root["list"]; ok {
		return toMapList(v)
	}
	// Last resort: top-level array (not handled above because we
	// parsed into a map; re-parse as array).
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}
	_ = status
	return nil, nil
}

func toMapList(v any) ([]map[string]any, error) {
	arr, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("expected array, got %T", v)
	}
	out := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		out = append(out, m)
	}
	return out, nil
}

func lookupPath(root map[string]any, path string) (any, bool) {
	if path == "" {
		return nil, false
	}
	parts := strings.Split(path, ".")
	cur := any(root)
	for _, p := range parts {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		v, ok := m[p]
		if !ok {
			return nil, false
		}
		cur = v
	}
	return cur, true
}

func codeEquals(v, want any) bool {
	// Support numeric (JSON) vs string comparison transparently.
	if vf, ok := v.(float64); ok {
		if wi, ok := want.(int); ok {
			return int(vf) == wi
		}
		if wf, ok := want.(float64); ok {
			return vf == wf
		}
	}
	return v == want
}

// toInt coerces a JSON-decoded value to int when possible.
func toInt(v any) (int, bool) {
	switch x := v.(type) {
	case float64:
		return int(x), true
	case int:
		return x, true
	case int64:
		return int(x), true
	}
	return 0, false
}

// lookupPathOr is like lookupPath but returns def when missing.
func lookupPathOr(root map[string]any, path string, def string) string {
	if v, ok := lookupPath(root, path); ok {
		return stringOf(v)
	}
	return def
}

// stringOf best-efforts to render v as a string.
func stringOf(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		return fmt.Sprintf("%v", x)
	}
	return fmt.Sprintf("%v", v)
}

// bodyPreview returns body[:200] for error messages.
func bodyPreview(body []byte) string {
	const max = 200
	if len(body) <= max {
		return string(body)
	}
	return string(body[:max]) + "..."
}

func joinErrs(errs []error) error {
	if len(errs) == 1 {
		return errs[0]
	}
	msg := errs[0].Error()
	for _, e := range errs[1:] {
		msg += "; " + e.Error()
	}
	return fmt.Errorf("%s", msg)
}

// extractID reads SourceID from m[key] (dotted path).
func extractID(m map[string]any, key string) string {
	if key == "" {
		key = "id"
	}
	v, ok := lookupPath(m, key)
	if !ok {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		// JSON numbers → string. Preserve integer form when possible.
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%v", x)
	case int64:
		return fmt.Sprintf("%d", x)
	case int:
		return fmt.Sprintf("%d", x)
	}
	return ""
}
