package data

import (
	"context"
	"errors"
	"io"
	"testing"

	pb "backend-service/api/platform/service/v1"
	"backend-service/pkg/auth/loginattempt"

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
	events []*pb.LoginLog
}

func (r *fakeLoginLogRepo) Append(_ context.Context, event *pb.LoginLog) error {
	r.events = append(r.events, event)
	return nil
}
func (r *fakeLoginLogRepo) List(context.Context, *pb.ListLoginLogsRequest) ([]*pb.LoginLog, int32, error) {
	return nil, 0, nil
}
func (r *fakeLoginLogRepo) Get(context.Context, uint64) (*pb.LoginLog, error) {
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
		repo := &authRepo{BaseRepo: BaseRepo{Log: logger}, guard: guard}

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
		repo := &authRepo{BaseRepo: BaseRepo{Log: logger}, guard: guard}

		err := repo.loginFailed(context.Background(), "username", "admin", 1)
		if !pb.IsUserTooManyLoginAttempts(err) {
			t.Fatalf("error = %v, want too many login attempts", err)
		}
	})

	t.Run("guard outage fails open", func(t *testing.T) {
		guard := &fakeLoginAttemptGuard{checkErr: errors.New("redis unavailable"), failureErr: errors.New("redis unavailable")}
		repo := &authRepo{BaseRepo: BaseRepo{Log: logger}, guard: guard}

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
	repo := &authRepo{BaseRepo: BaseRepo{Log: logger}, llr: loginLogs}

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
