package biz

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/proto"

	pb "backend-service/api/platform/service/v1"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"
)

type stubUserRepo struct {
	saved   *pb.User
	updated *pb.User
	current *pb.User
}

type stubSessionRepo struct {
	revokedUser   uint32
	revokedTenant uint32
}

func (*stubSessionRepo) List(context.Context, *pb.ListSessionsRequest, string) ([]*pb.Session, int32, error) {
	return nil, 0, nil
}
func (*stubSessionRepo) ListMine(context.Context, uint32, string) ([]*pb.Session, error) {
	return nil, nil
}
func (*stubSessionRepo) Revoke(context.Context, string) error { return nil }
func (r *stubSessionRepo) RevokeUser(_ context.Context, id uint32) error {
	r.revokedUser = id
	return nil
}
func (r *stubSessionRepo) RevokeTenant(_ context.Context, id uint32) error {
	r.revokedTenant = id
	return nil
}

func (r *stubUserRepo) Save(_ context.Context, user *pb.User) (*pb.User, error) {
	r.saved = user
	return user, nil
}

func (r *stubUserRepo) Update(_ context.Context, user *pb.User) (*pb.User, error) {
	r.updated = user
	return user, nil
}

func (r *stubUserRepo) FindByID(context.Context, uint32) (*pb.User, error) {
	if r.current == nil {
		return nil, nil
	}
	return proto.Clone(r.current).(*pb.User), nil
}
func (*stubUserRepo) ListByName(context.Context, string) ([]*pb.User, error) {
	return nil, nil
}
func (*stubUserRepo) ListByPhone(context.Context, string) ([]*pb.User, error) {
	return nil, nil
}
func (*stubUserRepo) ListUsers(context.Context, ...listing.Option) ([]*pb.User, error) {
	return nil, nil
}
func (*stubUserRepo) CountUsers(context.Context, ...listing.Option) (int32, error) { return 0, nil }
func (*stubUserRepo) ListUsersByDept(context.Context, uint32, bool, ...listing.Option) ([]*pb.User, error) {
	return nil, nil
}
func (*stubUserRepo) CountUsersByDept(context.Context, uint32, bool, ...listing.Option) (int32, error) {
	return 0, nil
}
func (*stubUserRepo) ListAll(context.Context) ([]*pb.User, error) { return nil, nil }
func (*stubUserRepo) ListPageSimple(context.Context, ...listing.Option) ([]*pb.User, error) {
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
		user    *pb.User
		wantErr bool
	}{
		{name: "missing password", user: &pb.User{Name: &name}, wantErr: true},
		{name: "weak password", user: &pb.User{Name: &name, Password: convert.ToPointer("weakpass")}, wantErr: true},
		{name: "strong password", user: &pb.User{Name: &name, Password: &strong}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &stubUserRepo{}
			uc := NewUserUsecase(repo, nil, log.NewStdLogger(io.Discard))
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
	uc := NewUserUsecase(repo, nil, log.NewStdLogger(io.Discard))

	if _, err := uc.Update(context.Background(), &pb.User{Id: 1, Password: &empty}); err == nil {
		t.Fatal("Update() error = nil")
	}
	if repo.updated != nil {
		t.Fatal("user with empty password was updated")
	}
}

func TestUserUsecasePasswordChangeRevokesAllUserSessions(t *testing.T) {
	password := "N3w!Password#2026"
	repo := &stubUserRepo{}
	sessions := &stubSessionRepo{}
	uc := NewUserUsecase(repo, sessions, log.NewStdLogger(io.Discard))

	if _, err := uc.Update(context.Background(), &pb.User{Id: 42, Password: &password}); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if sessions.revokedUser != 42 {
		t.Fatalf("revoked user = %d, want 42", sessions.revokedUser)
	}
}
