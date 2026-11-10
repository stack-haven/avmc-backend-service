package biz

import (
	"context"
	"strings"
	"testing"
	"time"

	pbenum "backend-service/api/common/enum"
	pb "backend-service/api/edai/service/v1"
	"backend-service/pkg/auth/authn"
	"backend-service/pkg/utils/crypto"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ── fakes ────────────────────────────────────────────────────────────────

type fakeUserRepo struct {
	user *pb.User
	hash string
}

func (f *fakeUserRepo) GetByUsername(context.Context, string) (*pb.User, string, error) {
	return f.user, f.hash, nil
}
func (f *fakeUserRepo) GetByID(context.Context, uint32) (*pb.User, error)        { return f.user, nil }
func (f *fakeUserRepo) UpdateLastLogin(context.Context, uint32, time.Time) error { return nil }
func (f *fakeUserRepo) Create(context.Context, *pb.User, string) (*pb.User, error) {
	return f.user, nil
}
func (f *fakeUserRepo) Count(context.Context) (int, error) { return 1, nil }

type fakeAuth struct{}

func (fakeAuth) CreateToken(context.Context, authn.AuthClaims, time.Duration) (string, error) {
	return "token", nil
}
func (fakeAuth) ValidateToken(context.Context, string) (*authn.AuthClaims, error) {
	claims := authn.AuthClaims{"sub": "1"}
	return &claims, nil
}
func (fakeAuth) RefreshToken(context.Context, string) (string, error) { return "token", nil }
func (fakeAuth) RevokeToken(context.Context, string) error            { return nil }

// bcryptHash 生成测试用密码哈希（"secret"）。
func bcryptHash(t *testing.T) string {
	t.Helper()
	h, err := crypto.HashPassword("secret")
	require.NoError(t, err)
	return h
}

// discard 是 log 的最小 Writer，避免测试输出噪声。
type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }

// statusEnabled / statusDisabled 返回用户状态枚举值。
func statusEnabled() *pbenum.Status {
	s := pbenum.Status_STATUS_ENABLED
	return &s
}

func statusDisabled() *pbenum.Status {
	s := pbenum.Status_STATUS_DISABLED
	return &s
}

// ── tests ────────────────────────────────────────────────────────────────

func TestAuthUsecase_LoginSuccess(t *testing.T) {
	uc := NewAuthUsecase(&fakeUserRepo{
		user: &pb.User{Id: 1, Username: "admin", Role: "admin", Status: statusEnabled()},
		hash: bcryptHash(t),
	}, fakeAuth{}, time.Hour, log.NewStdLogger(discard{}))

	token, user, err := uc.Login(context.Background(), "admin", "secret")
	require.NoError(t, err)
	require.NotNil(t, token)
	assert.Equal(t, "Bearer", token.GetTokenType())
	assert.Equal(t, int64(3600), token.GetExpiresIn())
	assert.Equal(t, uint32(1), user.GetId())
}

func TestAuthUsecase_LoginWrongPassword(t *testing.T) {
	uc := NewAuthUsecase(&fakeUserRepo{
		user: &pb.User{Id: 1, Username: "admin", Status: statusEnabled()},
		hash: bcryptHash(t),
	}, fakeAuth{}, time.Hour, log.NewStdLogger(discard{}))

	_, _, err := uc.Login(context.Background(), "admin", "wrong")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "账号或密码错误")
}

func TestAuthUsecase_LoginUserNotFound(t *testing.T) {
	uc := NewAuthUsecase(&fakeUserRepo{user: nil, hash: ""}, fakeAuth{}, time.Hour, log.NewStdLogger(discard{}))

	_, _, err := uc.Login(context.Background(), "ghost", "any")
	require.Error(t, err)
	// 与密码错误返回同一错误，防止账号枚举
	assert.Contains(t, err.Error(), "账号或密码错误")
}

func TestAuthUsecase_LoginDisabledAccount(t *testing.T) {
	uc := NewAuthUsecase(&fakeUserRepo{
		user: &pb.User{Id: 1, Username: "admin", Status: statusDisabled()},
		hash: bcryptHash(t),
	}, fakeAuth{}, time.Hour, log.NewStdLogger(discard{}))

	_, _, err := uc.Login(context.Background(), "admin", "secret")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "账号已被禁用")
}

func TestAuthUsecase_RefreshInvalid(t *testing.T) {
	uc := NewAuthUsecase(&fakeUserRepo{}, failingAuth{}, time.Hour, log.NewStdLogger(discard{}))
	_, err := uc.Refresh(context.Background(), "garbage")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "刷新令牌无效")
}

type failingAuth struct{ fakeAuth }

func (failingAuth) ValidateToken(context.Context, string) (*authn.AuthClaims, error) {
	return nil, assert.AnError
}

// ── source validation ───────────────────────────────────────────────────

func TestValidateSource(t *testing.T) {
	tests := []struct {
		name    string
		src     *pb.DataSource
		wantErr string
	}{
		{"nil", nil, "数据源不能为空"},
		{"empty name", &pb.DataSource{RootPath: "/tmp"}, "数据源名称不能为空"},
		{"bad type", &pb.DataSource{Name: "a", Type: "ftp", RootPath: "/tmp"}, "不支持的数据源类型"},
		{"relative path", &pb.DataSource{Name: "a", Type: "local", RootPath: "rel"}, "必须使用绝对路径"},
		{"empty root", &pb.DataSource{Name: "a", Type: "local"}, "根路径不能为空"},
		{"ok", &pb.DataSource{Name: "a", Type: "local", RootPath: "/tmp"}, ""},
		{"ok default type", &pb.DataSource{Name: "a", RootPath: "/tmp"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSource(tt.src)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.True(t, strings.Contains(err.Error(), tt.wantErr), "got: %v", err)
		})
	}
}
