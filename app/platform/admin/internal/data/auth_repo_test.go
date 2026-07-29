package data

import (
	"context"
	"errors"
	"io"
	"testing"

	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/pkg/auth/loginattempt"
	"backend-service/pkg/utils/crypto"

	"github.com/go-kratos/kratos/v2/log"
)

type fakeLoginAttemptGuard struct {
	checkErr   error
	failureErr error
	successErr error
	checks     int
	failures   int
	successes  int
}

type fakeLoginLogRepo struct {
	events []*pbCore.LoginLog
}

func (r *fakeLoginLogRepo) Append(_ context.Context, event *pbCore.LoginLog) error {
	r.events = append(r.events, event)
	return nil
}
func (r *fakeLoginLogRepo) List(context.Context, *pbCore.ListLoginLogsRequest) ([]*pbCore.LoginLog, int32, error) {
	return nil, 0, nil
}
func (r *fakeLoginLogRepo) Get(context.Context, uint64) (*pbCore.LoginLog, error) {
	return nil, nil
}

func (g *fakeLoginAttemptGuard) Check(context.Context, string, string, uint32) error {
	g.checks++
	return g.checkErr
}

func (g *fakeLoginAttemptGuard) Failure(context.Context, string, string, uint32) error {
	g.failures++
	return g.failureErr
}

func (g *fakeLoginAttemptGuard) Success(context.Context, string, string, uint32) error {
	g.successes++
	return g.successErr
}

func TestAuthRepoLoginAttemptProtection(t *testing.T) {
	t.Parallel()

	logger := log.NewHelper(log.NewStdLogger(io.Discard))
	t.Run("locked before database lookup", func(t *testing.T) {
		guard := &fakeLoginAttemptGuard{checkErr: loginattempt.ErrLocked}
		repo := &authRepo{data: &Data{}, log: logger, guard: guard}

		_, err := repo.LoginByUsername(context.Background(), "admin", "secret", 1)
		if !pb.IsUserTooManyLoginAttempts(err) {
			t.Fatalf("error = %v, want too many login attempts", err)
		}
		if guard.checks != 1 || guard.failures != 0 {
			t.Fatalf("guard calls = checks:%d failures:%d", guard.checks, guard.failures)
		}
	})

	t.Run("threshold failure returns lock error", func(t *testing.T) {
		guard := &fakeLoginAttemptGuard{failureErr: loginattempt.ErrLocked}
		repo := &authRepo{log: logger, guard: guard}

		err := repo.loginFailed(context.Background(), "username", "admin", 1)
		if !pb.IsUserTooManyLoginAttempts(err) {
			t.Fatalf("error = %v, want too many login attempts", err)
		}
	})

	t.Run("guard outage fails open", func(t *testing.T) {
		guard := &fakeLoginAttemptGuard{checkErr: errors.New("redis unavailable"), failureErr: errors.New("redis unavailable")}
		repo := &authRepo{log: logger, guard: guard}

		if err := repo.checkLogin(context.Background(), "username", "admin", 1); err != nil {
			t.Fatalf("checkLogin error = %v, want fail-open", err)
		}
		if err := repo.loginFailed(context.Background(), "username", "admin", 1); !pb.IsUserIncorrectPassword(err) {
			t.Fatalf("loginFailed error = %v, want generic auth failure", err)
		}
	})
}

func TestAuthRepoRecordsLoginSecurityResults(t *testing.T) {
	logger := log.NewHelper(log.NewStdLogger(io.Discard))
	loginLogs := &fakeLoginLogRepo{}
	repo := &authRepo{log: logger, llr: loginLogs}

	repo.recordLogin(context.Background(), "username", "admin", 1, &pb.LoginResponse{Id: 7}, nil)
	repo.recordLogin(context.Background(), "email", "unknown@example.com", 1, nil, pb.ErrorUserIncorrectPassword("用户名或密码错误"))
	repo.recordLogin(context.Background(), "username", "locked", 1, nil, pb.ErrorUserTooManyLoginAttempts("登录失败次数过多"))

	if len(loginLogs.events) != 3 {
		t.Fatalf("login event count = %d, want 3", len(loginLogs.events))
	}
	if loginLogs.events[0].GetResult() != "success" || loginLogs.events[0].GetUserId() != 7 {
		t.Fatalf("success event = %+v", loginLogs.events[0])
	}
	if loginLogs.events[1].GetResult() != "failure" || loginLogs.events[1].UserId != nil {
		t.Fatalf("failure event = %+v", loginLogs.events[1])
	}
	if loginLogs.events[2].GetResult() != "locked" {
		t.Fatalf("locked event = %+v", loginLogs.events[2])
	}
}

func TestAuthRepoMenusFiltersDisabledRolesAndAddsAncestors(t *testing.T) {
	ctx := tenantContext(1)
	seedCtx := systemContext()
	client := newTestClient(t)
	defer client.Close()

	parent := client.Menu.Create().
		SetName("system").
		SetTitle("System").
		SetStatus(1).
		SetType(int32(pbCore.MenuType_MENU_TYPE_CATALOG)).
		SaveX(seedCtx)
	child := client.Menu.Create().
		SetName("users").
		SetTitle("Users").
		SetStatus(1).
		SetType(int32(pbCore.MenuType_MENU_TYPE_MENU)).
		SetParentID(parent.ID).
		SetAuthCode("system:user:list").
		SaveX(seedCtx)
	seedTenantMenuPermissionGroup(t, client, 1, child.ID)
	disabledMenu := client.Menu.Create().
		SetName("disabled-role-menu").
		SetTitle("Disabled Role Menu").
		SetStatus(1).
		SetType(int32(pbCore.MenuType_MENU_TYPE_MENU)).
		SetAuthCode("system:disabled:list").
		SaveX(seedCtx)

	enabledRole := client.Role.Create().
		SetName("enabled_role").
		SetDefaultRouter("/").
		SetDataScope(1).
		SetMenuCheckStrictly(1).
		SetDeptCheckStrictly(1).
		SetStatus(1).
		SetTenantID(1).
		AddMenuIDs(child.ID).
		SaveX(seedCtx)
	disabledRole := client.Role.Create().
		SetName("disabled_role").
		SetDefaultRouter("/").
		SetDataScope(1).
		SetMenuCheckStrictly(1).
		SetDeptCheckStrictly(1).
		SetStatus(2).
		SetTenantID(1).
		AddMenuIDs(disabledMenu.ID).
		SaveX(seedCtx)
	otherTenantRole := client.Role.Create().
		SetName("other_tenant_role").
		SetDefaultRouter("/").
		SetDataScope(1).
		SetMenuCheckStrictly(1).
		SetDeptCheckStrictly(1).
		SetStatus(1).
		SetTenantID(2).
		AddMenuIDs(disabledMenu.ID).
		SaveX(seedCtx)
	user := client.User.Create().
		SetName("tester").
		SetPassword("secret1").
		SetStatus(1).
		SetTenantID(1).
		AddRoleIDs(enabledRole.ID, disabledRole.ID, otherTenantRole.ID).
		SaveX(seedCtx)

	logger := log.NewStdLogger(io.Discard)
	repo := &authRepo{
		data: &Data{db: client},
		log:  log.NewHelper(logger),
		mr:   NewMenuRepo(&Data{db: client}, logger).(*menuRepo),
	}

	codes, err := repo.Codes(ctx, user.ID)
	if err != nil {
		t.Fatalf("codes: %v", err)
	}
	if len(codes) != 1 || codes[0] != "system:user:list" {
		t.Fatalf("codes = %#v", codes)
	}

	menus, err := repo.Menus(ctx, user.ID)
	if err != nil {
		t.Fatalf("menus: %v", err)
	}
	if len(menus) != 1 {
		t.Fatalf("root menu len = %d, menus=%#v", len(menus), menus)
	}
	if menus[0].GetId() != parent.ID {
		t.Fatalf("root id = %d, want parent %d", menus[0].GetId(), parent.ID)
	}
	if len(menus[0].Children) != 1 || menus[0].Children[0].GetId() != child.ID {
		t.Fatalf("children = %#v, want child %d", menus[0].Children, child.ID)
	}
	if _, err := repo.Codes(tenantContext(2), user.ID); err == nil {
		t.Fatal("cross-tenant codes error = nil")
	}
}
