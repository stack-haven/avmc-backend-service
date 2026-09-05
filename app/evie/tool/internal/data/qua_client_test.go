package data

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	v1 "backend-service/api/evie/tool/v1"
	"backend-service/app/evie/tool/internal/conf"
	"google.golang.org/protobuf/types/known/durationpb"
)

// testCtx 把 AuthInfo 注入 ctx（模拟 TokenAuthMiddleware 后的状态）。
func testCtx() context.Context {
	return WithAuthInfo(context.Background(), &AuthInfo{
		TenantID:    "158",
		UserID:      "2031552504886435841",
		AccessToken: "5202d271dec64f0887a9e462c5299fad",
	})
}

// newTestFetcher 启动 mock qua server 并返回 QuaFetcher。
func newTestFetcher(t *testing.T, handler http.HandlerFunc) (QuaFetcher, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	cfg := &conf.Qua{
		BaseUrl: srv.URL,
		Timeout: durationpb.New(2 * time.Second),
		Endpoints: &conf.Qua_Endpoints{
			ListUsers: "/admin-api/qua/member-extended/page",
			ListDepts: "/admin-api/system/dept/list",
		},
		ExtraHeaders: map[string]string{"zone": "Asia/Shanghai"},
	}
	f, err := NewQuaClient(cfg, nil)
	if err != nil {
		srv.Close()
		t.Fatalf("NewQuaClient: %v", err)
	}
	return f, srv
}

// ============================================================================
// FetchUsersRaw 测试
// ============================================================================

func TestQuaClient_FetchUsersRaw_Success(t *testing.T) {
	var capturedAuth, capturedTenantID, capturedSelectAll string
	var capturedMethod string

	f, srv := newTestFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		capturedTenantID = r.Header.Get("tenant-id")
		capturedMethod = r.Method
		capturedSelectAll = r.URL.Query().Get("selectAll")

		// 服务端返回的字段与 qua 真实保持一致（realName / nickname / userInfo.nickname 都要测到）
		resp := map[string]any{
			"code": 0,
			"msg":  "ok",
			"data": map[string]any{
				"list": []map[string]any{
					{
						"id":       "2031552504886435841",
						"tenantId": "158",
						"username": "tester01",
						"realName": "测试一号",
						"nickname": "一号",
						"deptId":   "1904450235179954177",
						"status":   1,
					},
					{
						"id":       "2031552504886435843",
						"tenantId": "158",
						"userInfo": map[string]any{
							"nickname": "嵌套昵称",
							"deptId":   "9999",
						},
						"status": 1,
					},
				},
				"total": 2,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer srv.Close()

	users, err := f.FetchUsersRaw(testCtx())
	if err != nil {
		t.Fatalf("FetchUsersRaw: %v", err)
	}

	if capturedMethod != "GET" {
		t.Errorf("method = %q, want GET", capturedMethod)
	}
	if capturedAuth != "Bearer 5202d271dec64f0887a9e462c5299fad" {
		t.Errorf("Authorization = %q", capturedAuth)
	}
	if capturedTenantID != "158" {
		t.Errorf("tenant-id = %q, want 158 (digits)", capturedTenantID)
	}
	if capturedSelectAll != "true" {
		t.Errorf("selectAll = %q, want true", capturedSelectAll)
	}

	if len(users) != 2 {
		t.Fatalf("len(users) = %d, want 2", len(users))
	}

	// opaque map：验证关键字段可被读到（但 Normalizer 才解释）
	u0 := users[0]
	if u0["realName"] != "测试一号" {
		t.Errorf("users[0].realName = %v", u0["realName"])
	}
	if u0["nickname"] != "一号" {
		t.Errorf("users[0].nickname = %v", u0["nickname"])
	}

	// 嵌套字段也应透传
	u1 := users[1]
	if info, ok := u1["userInfo"].(map[string]any); !ok || info["nickname"] != "嵌套昵称" {
		t.Errorf("users[1].userInfo.nickname = %v", u1["userInfo"])
	}
}

func TestQuaClient_FetchUsersRaw_BusinessError(t *testing.T) {
	f, srv := newTestFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 401, "msg": "qua token expired"})
	})
	defer srv.Close()

	_, err := f.FetchUsersRaw(testCtx())
	if err == nil {
		t.Fatal("expected error")
	}
	if !v1.IsQuaUnauthorized(err) {
		t.Errorf("expected ErrorQuaUnauthorized, got %v", err)
	}
}

func TestQuaClient_FetchUsersRaw_InternalError(t *testing.T) {
	f, srv := newTestFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 500, "msg": "qua down"})
	})
	defer srv.Close()

	_, err := f.FetchUsersRaw(testCtx())
	if !v1.IsQuaInternalError(err) {
		t.Errorf("expected ErrorQuaInternalError, got %v", err)
	}
}

func TestQuaClient_FetchUsersRaw_InvalidJSON(t *testing.T) {
	f, srv := newTestFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not a json"))
	})
	defer srv.Close()

	_, err := f.FetchUsersRaw(testCtx())
	if !v1.IsQuaInvalidResponse(err) {
		t.Errorf("expected ErrorQuaInvalidResponse, got %v", err)
	}
}

func TestQuaClient_FetchUsersRaw_Unreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	cfg := &conf.Qua{
		BaseUrl: url,
		Timeout: durationpb.New(200 * time.Millisecond),
		Endpoints: &conf.Qua_Endpoints{
			ListUsers: "/admin-api/qua/member-extended/page",
			ListDepts: "/admin-api/system/dept/list",
		},
	}
	f, err := NewQuaClient(cfg, nil)
	if err != nil {
		t.Fatalf("NewQuaClient: %v", err)
	}
	_, err = f.FetchUsersRaw(testCtx())
	if !v1.IsQuaUnreachable(err) {
		t.Errorf("expected ErrorQuaUnreachable, got %v", err)
	}
}

// ============================================================================
// FetchDeptsRaw 测试
// ============================================================================

func TestQuaClient_FetchDeptsRaw_Success(t *testing.T) {
	var capturedZone string
	f, srv := newTestFetcher(t, func(w http.ResponseWriter, r *http.Request) {
		capturedZone = r.Header.Get("zone")
		if r.Method != "GET" {
			t.Errorf("method = %q, want GET", r.Method)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0,
			"msg":  "ok",
			"data": []map[string]any{
				{"id": "1", "parentId": "0", "name": "根部门"},
				{"id": "2", "parentId": "1", "name": "技术研发部"},
			},
		})
	})
	defer srv.Close()

	depts, err := f.FetchDeptsRaw(testCtx())
	if err != nil {
		t.Fatalf("FetchDeptsRaw: %v", err)
	}
	if capturedZone != "Asia/Shanghai" {
		t.Errorf("zone = %q, want Asia/Shanghai", capturedZone)
	}
	if len(depts) != 2 {
		t.Fatalf("len(depts) = %d, want 2", len(depts))
	}
	if depts[0]["name"] != "根部门" {
		t.Errorf("depts[0].name = %v", depts[0]["name"])
	}
}

// ============================================================================
// 错误情况
// ============================================================================

func TestQuaClient_NoAuthInContext(t *testing.T) {
	f, srv := newTestFetcher(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	_, err := f.FetchUsersRaw(context.Background())
	if !v1.IsTokenMissing(err) {
		t.Errorf("expected ErrorTokenMissing, got %v", err)
	}
}

func TestQuaClient_NilConfig(t *testing.T) {
	_, err := NewQuaClient(nil, nil)
	if !v1.IsInternalError(err) {
		t.Errorf("expected ErrorInternalError, got %v", err)
	}
}

func TestQuaClient_MissingEndpoints(t *testing.T) {
	cfg := &conf.Qua{
		BaseUrl:   "http://localhost",
		Timeout:   durationpb.New(2 * time.Second),
		Endpoints: &conf.Qua_Endpoints{ListUsers: "", ListDepts: "/depts"},
	}
	_, err := NewQuaClient(cfg, nil)
	if !v1.IsInternalError(err) {
		t.Errorf("expected ErrorInternalError for missing list_users, got %v", err)
	}
}

// ============================================================================
// Adapter (QuaVocabularySource) 测试
// ============================================================================

func TestQuaVocabularySource_Fetch(t *testing.T) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/admin-api/qua/member-extended/page":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": map[string]any{
					"list": []map[string]any{
						{"id": 2031552504886435841, "realName": "测试一号"}, // 数字 ID
					},
				},
			})
		case "/admin-api/system/dept/list":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": []map[string]any{
					{"id": "1", "name": "根部门"}, // 字符串 ID
				},
			})
		default:
			w.WriteHeader(404)
		}
	}
	f, srv := newTestFetcher(t, handler)
	defer srv.Close()

	src := NewQuaVocabularySource(f)
	if src.Name() != "qua" {
		t.Errorf("Name() = %q, want qua", src.Name())
	}

	raws, err := src.Fetch(testCtx())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(raws) != 2 {
		t.Fatalf("len(raws) = %d, want 2 (1 user + 1 dept)", len(raws))
	}

	// 验证：数字 ID → string
	user := raws[0]
	if user.EntityType != "user" {
		t.Errorf("EntityType = %q, want user", user.EntityType)
	}
	if user.Source != "qua" {
		t.Errorf("Source = %q, want qua", user.Source)
	}
	if user.SourceID == "" {
		t.Errorf("SourceID empty; want non-empty from numeric id 2031552504886435841")
	}
	if user.Data["realName"] != "测试一号" {
		t.Errorf("user.Data[realName] = %v", user.Data["realName"])
	}

	// 验证：字符串 ID 透传
	dept := raws[1]
	if dept.EntityType != "department" {
		t.Errorf("EntityType = %q, want department", dept.EntityType)
	}
	if dept.SourceID != "1" {
		t.Errorf("dept.SourceID = %q, want 1", dept.SourceID)
	}
}

// 防止 errors 包被误删
var _ = errors.Is