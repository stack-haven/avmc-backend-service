package biz

import (
	"context"
	"io"
	"strings"
	"testing"

	pbCore "backend-service/api/core/service/v1"

	"github.com/go-kratos/kratos/v2/log"
)

type stubUserRepo struct {
	saved   *pbCore.User
	updated *pbCore.User
}

func (r *stubUserRepo) Save(_ context.Context, user *pbCore.User) (*pbCore.User, error) {
	r.saved = user
	return user, nil
}

func (r *stubUserRepo) Update(_ context.Context, user *pbCore.User) (*pbCore.User, error) {
	r.updated = user
	return user, nil
}

func (*stubUserRepo) FindByID(context.Context, uint32) (*pbCore.User, error) { return nil, nil }
func (*stubUserRepo) ListByName(context.Context, string) ([]*pbCore.User, error) {
	return nil, nil
}
func (*stubUserRepo) ListByPhone(context.Context, string) ([]*pbCore.User, error) {
	return nil, nil
}
func (*stubUserRepo) ListUsers(context.Context, ...ListOption) ([]*pbCore.User, error) {
	return nil, nil
}
func (*stubUserRepo) CountUsers(context.Context, ...ListOption) (int32, error) { return 0, nil }
func (*stubUserRepo) ListAll(context.Context) ([]*pbCore.User, error)          { return nil, nil }
func (*stubUserRepo) ListPageSimple(context.Context, ...ListOption) ([]*pbCore.User, error) {
	return nil, nil
}
func (*stubUserRepo) Delete(context.Context, uint32) error                 { return nil }
func (*stubUserRepo) ExistByName(context.Context, string) (uint32, error)  { return 0, nil }
func (*stubUserRepo) ExistByPhone(context.Context, string) (uint32, error) { return 0, nil }
func (*stubUserRepo) ExistByEmail(context.Context, string) (uint32, error) { return 0, nil }

func TestUserUsecaseCreateRequiresAndHashesStrongPassword(t *testing.T) {
	t.Parallel()

	name := "admin"
	strong := "Str0ng!Admin#2026"
	tests := []struct {
		name    string
		user    *pbCore.User
		wantErr bool
	}{
		{name: "missing password", user: &pbCore.User{Name: &name}, wantErr: true},
		{name: "weak password", user: &pbCore.User{Name: &name, Password: stringPtr("weakpass")}, wantErr: true},
		{name: "strong password", user: &pbCore.User{Name: &name, Password: &strong}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubUserRepo{}
			uc := NewUserUsecase(repo, log.NewStdLogger(io.Discard))
			_, err := uc.Create(context.Background(), tt.user)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Create() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && repo.saved != nil {
				t.Fatal("invalid user was saved")
			}
			if !tt.wantErr {
				if repo.saved == nil || repo.saved.Password == nil {
					t.Fatal("valid user was not saved")
				}
				if *repo.saved.Password == strong || !strings.HasPrefix(*repo.saved.Password, "$2") {
					t.Fatalf("password was not bcrypt hashed: %q", *repo.saved.Password)
				}
			}
		})
	}
}

func TestUserUsecaseUpdateRejectsExplicitEmptyPassword(t *testing.T) {
	t.Parallel()

	empty := ""
	repo := &stubUserRepo{}
	uc := NewUserUsecase(repo, log.NewStdLogger(io.Discard))

	if _, err := uc.Update(context.Background(), &pbCore.User{Id: 1, Password: &empty}); err == nil {
		t.Fatal("Update() error = nil")
	}
	if repo.updated != nil {
		t.Fatal("user with empty password was updated")
	}
}

func stringPtr(value string) *string {
	return &value
}
