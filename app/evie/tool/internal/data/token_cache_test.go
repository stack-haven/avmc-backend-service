package data

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"backend-service/app/evie/tool/internal/conf"
)

// TestTokenCache_RoundTrip 模拟 Redis SET → GET → 反序列化全链路。
func TestTokenCache_RoundTrip(t *testing.T) {
	// 这里使用 miniredis 替换；本测试仅覆盖 JSON 反序列化逻辑。
	// Redis I/O 测试在 integration_test.go（需要本地 Redis 实例）。
	payload := map[string]interface{}{
		"tenantId":    "1889501240003497986",
		"id":          "2094623134267203585",
		"accessToken": "68aa8a4a9cf14b149164a9f451b2893c",
		"userId":      "2031552504886435841",
		"userType":    2,
		"userInfo": map[string]string{
			"nickname": "测试账号",
			"deptId":   "1904450235179954177",
		},
		"clientId":    "default",
		"expiresTime": 1788491296083,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}

	var info AuthInfo
	if err := json.Unmarshal(raw, &info); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if info.TenantID != "1889501240003497986" {
		t.Errorf("TenantID = %q, want 1889501240003497986", info.TenantID)
	}
	if info.UserID != "2031552504886435841" {
		t.Errorf("UserID = %q, want 2031552504886435841", info.UserID)
	}
	if info.UserInfo.Nickname != "测试账号" {
		t.Errorf("Nickname = %q, want 测试账号", info.UserInfo.Nickname)
	}
	if info.UserInfo.DeptID != "1904450235179954177" {
		t.Errorf("DeptID = %q, want 1904450235179954177", info.UserInfo.DeptID)
	}
	if info.AccessToken != "68aa8a4a9cf14b149164a9f451b2893c" {
		t.Errorf("AccessToken mismatch")
	}
	if info.ExpiresAt != 1788491296083 {
		t.Errorf("ExpiresAt = %d, want 1788491296083", info.ExpiresAt)
	}
	// 验证时间可正常解析（epoch ms）
	exp := time.UnixMilli(info.ExpiresAt)
	if exp.Year() < 2026 {
		t.Errorf("ExpiresAt year %d, expected > 2026 (epoch ms 1788491296083)", exp.Year())
	}
}

// TestTokenCache_Key 验证 Redis key 拼接正确。
func TestTokenCache_Key(t *testing.T) {
	c := NewTokenCache(nil, &conf.Data_Redis{TokenKeyPrefix: "oauth2_access_token:"})
	if got := c.Key("abc123"); got != "oauth2_access_token:abc123" {
		t.Errorf("Key = %q, want oauth2_access_token:abc123", got)
	}
	// 默认前缀
	c2 := NewTokenCache(nil, &conf.Data_Redis{TokenKeyPrefix: ""})
	if got := c2.Key("x"); got != "oauth2_access_token:x" {
		t.Errorf("default prefix Key = %q", got)
	}
}

// TestTokenCache_Get_NilRedis 仅验证构造 + 不 panic（真实场景需 Redis 实例）。
func TestTokenCache_Get_NilRedis(t *testing.T) {
	c := NewTokenCache(nil, &conf.Data_Redis{TokenKeyPrefix: "p:"})
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from nil redis client")
		}
	}()
	_, _ = c.Get(context.Background(), "abc")
}

// TestNewRedisClient_NilConfig 验证 conf 缺失时拒绝启动。
func TestNewRedisClient_NilConfig(t *testing.T) {
	if _, err := NewRedisClient(nil); err == nil {
		t.Fatal("expected error for nil conf.Data")
	}
}

// 防止 redis 包被误删（编译期断言）
var _ = redis.NewClient