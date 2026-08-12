package service

import (
	"context"
	"errors"
	"io"
	"reflect"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/pkg/aip/listing"
)

type roleRepoServiceStub struct {
	current *pbCore.Role
	updated *pbCore.Role
	saved   *pbCore.Role
	roles   []*pbCore.Role

	deletedID uint32
	existName string
	existID   uint32
	exist     bool
	findErr   error
}

func (r *roleRepoServiceStub) Save(_ context.Context, role *pbCore.Role) (*pbCore.Role, error) {
	r.saved = role
	return role, nil
}

func (r *roleRepoServiceStub) Update(_ context.Context, role *pbCore.Role) (*pbCore.Role, error) {
	r.updated = role
	return role, nil
}

func (r *roleRepoServiceStub) FindByID(context.Context, uint32) (*pbCore.Role, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	if r.current != nil {
		return protoCloneRole(r.current), nil
	}
	return &pbCore.Role{Id: 1}, nil
}

func (*roleRepoServiceStub) CountRoles(context.Context, ...listing.Option) (int32, error) {
	return 0, nil
}
func (r *roleRepoServiceStub) ListAll(context.Context) ([]*pbCore.Role, error) { return r.roles, nil }
func (*roleRepoServiceStub) ListRoles(context.Context, ...listing.Option) ([]*pbCore.Role, error) {
	return nil, nil
}
func (r *roleRepoServiceStub) Delete(_ context.Context, id uint32) error {
	r.deletedID = id
	return nil
}
func (r *roleRepoServiceStub) ExistByName(_ context.Context, name string, excludeID uint32) (bool, error) {
	r.existName = name
	r.existID = excludeID
	return r.exist, nil
}

func protoCloneRole(role *pbCore.Role) *pbCore.Role {
	return proto.Clone(role).(*pbCore.Role) //nolint:errcheck // proto.Clone does not return error
}

func newRoleServiceForTest(repo *roleRepoServiceStub) *RoleServiceService {
	return NewRoleServiceService(biz.NewRoleUsecase(repo, log.NewStdLogger(io.Discard)), log.NewStdLogger(io.Discard))
}

func TestRoleServiceUpdateRolePreservesMenuIDsWhenMaskExcludesAuthorization(t *testing.T) {
	t.Parallel()

	oldName := "租户管理员"
	newName := "租户运营"
	repo := &roleRepoServiceStub{current: &pbCore.Role{Id: 7, Name: &oldName, MenuIds: []uint32{1, 2, 3}}}
	service := newRoleServiceForTest(repo)

	if _, err := service.UpdateRole(context.Background(), &pbCore.UpdateRoleRequest{
		Id:         7,
		Role:       &pbCore.Role{Name: &newName, MenuIds: []uint32{99}},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"name"}},
	}); err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}
	if repo.updated.GetName() != "租户运营" {
		t.Fatalf("updated name = %q, want 租户运营", repo.updated.GetName())
	}
	if !reflect.DeepEqual(repo.updated.GetMenuIds(), []uint32{1, 2, 3}) {
		t.Fatalf("menu ids = %v, want preserved [1 2 3]", repo.updated.GetMenuIds())
	}
}

func TestRoleServiceListRoleSimpleUsesLightweightRoleList(t *testing.T) {
	repo := &roleRepoServiceStub{
		roles: []*pbCore.Role{
			{Id: 1, Name: ptrString("管理员"), IsTenantAdmin: true},
			{Id: 2, Name: ptrString("普通用户")},
		},
	}
	service := newRoleServiceForTest(repo)

	resp, err := service.ListRoleSimple(context.Background(), &pbCore.ListRoleSimpleRequest{})
	if err != nil {
		t.Fatalf("ListRoleSimple() error = %v", err)
	}
	if got, want := resp.GetTotal(), int32(2); got != want {
		t.Fatalf("ListRoleSimple() total = %d, want %d", got, want)
	}
	if got := len(resp.GetItems()); got != 2 {
		t.Fatalf("ListRoleSimple() items len = %d, want 2", got)
	}
}

func TestRoleServiceUpdateRoleAppliesMenuIDsWhenMaskIncludesAuthorization(t *testing.T) {
	t.Parallel()

	name := "租户管理员"
	repo := &roleRepoServiceStub{current: &pbCore.Role{Id: 7, Name: &name, MenuIds: []uint32{1, 2, 3}}}
	service := newRoleServiceForTest(repo)

	if _, err := service.UpdateRole(context.Background(), &pbCore.UpdateRoleRequest{
		Id:         7,
		Role:       &pbCore.Role{MenuIds: []uint32{10, 20}},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"menu_ids"}},
	}); err != nil {
		t.Fatalf("UpdateRole() error = %v", err)
	}
	if !reflect.DeepEqual(repo.updated.GetMenuIds(), []uint32{10, 20}) {
		t.Fatalf("menu ids = %v, want [10 20]", repo.updated.GetMenuIds())
	}
}

func TestRoleServiceValidationAndExistByName(t *testing.T) {
	t.Parallel()

	repo := &roleRepoServiceStub{exist: true}
	service := newRoleServiceForTest(repo)
	roleID := uint32(7)

	if _, err := service.GetRole(context.Background(), &pbCore.GetRoleRequest{}); err == nil {
		t.Fatal("GetRole() without id succeeded")
	}
	if _, err := service.CreateRole(context.Background(), &pbCore.CreateRoleRequest{}); err == nil {
		t.Fatal("CreateRole() without role succeeded")
	}
	if _, err := service.DeleteRole(context.Background(), &pbCore.DeleteRoleRequest{}); err == nil {
		t.Fatal("DeleteRole() without id succeeded")
	}

	resp, err := service.ExistRoleByName(context.Background(), &pbCore.ExistRoleByNameRequest{Name: "admin", Id: &roleID})
	if err != nil {
		t.Fatalf("ExistRoleByName() error = %v", err)
	}
	if !resp.GetExist() || repo.existName != "admin" || repo.existID != 7 {
		t.Fatalf("exist response=%v name=%q id=%d", resp.GetExist(), repo.existName, repo.existID)
	}

	if _, err := service.UpdateRole(context.Background(), &pbCore.UpdateRoleRequest{Id: 7, Role: &pbCore.Role{}}); err != nil {
		t.Fatalf("UpdateRole() with empty mask error = %v", err)
	}
	if repo.updated.GetId() != 7 {
		t.Fatalf("updated id = %d, want 7", repo.updated.GetId())
	}

	expected := errors.New("missing")
	repo.findErr = expected
	if _, err := service.UpdateRole(context.Background(), &pbCore.UpdateRoleRequest{Id: 7, Role: &pbCore.Role{}}); !errors.Is(err, expected) {
		t.Fatalf("UpdateRole() error = %v, want %v", err, expected)
	}
}

func TestRoleServiceUpdateStatus(t *testing.T) {
	t.Parallel()

	enabled := pbEnum.Status_STATUS_ENABLED
	disabled := pbEnum.Status_STATUS_DISABLED
	repo := &roleRepoServiceStub{current: &pbCore.Role{Id: 7, Status: &enabled}}
	service := newRoleServiceForTest(repo)

	if _, err := service.UpdateRoleByStatus(context.Background(), &pbCore.UpdateRoleByStatusRequest{Id: 7, Status: &disabled}); err != nil {
		t.Fatalf("UpdateRoleByStatus() error = %v", err)
	}
	if repo.updated.GetStatus() != pbEnum.Status_STATUS_DISABLED {
		t.Fatalf("status = %v, want disabled", repo.updated.GetStatus())
	}
}
