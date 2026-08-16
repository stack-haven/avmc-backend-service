package service

import (
	"context"
	"io"
	"reflect"
	"testing"

	"github.com/go-kratos/kratos/v2/log"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
	"backend-service/pkg/aip/listing"
)

type menuRepoServiceStub struct {
	current *pb.Menu
	menus   []*pb.Menu
	updated *pb.Menu
	saved   *pb.Menu

	deletedID uint32
	pathReq   *pb.ExistMenuByPathRequest
	nameReq   *pb.ExistMenuByNameRequest
	existPath bool
	existName bool
}

func (r *menuRepoServiceStub) Save(_ context.Context, menu *pb.Menu) (*pb.Menu, error) {
	r.saved = menu
	return menu, nil
}
func (r *menuRepoServiceStub) Update(_ context.Context, menu *pb.Menu) (*pb.Menu, error) {
	r.updated = menu
	return menu, nil
}
func (r *menuRepoServiceStub) FindByID(context.Context, uint32) (*pb.Menu, error) {
	if r.current != nil {
		return cloneMenu(r.current), nil
	}
	return &pb.Menu{Id: 1}, nil
}
func (*menuRepoServiceStub) CountMenus(context.Context, ...listing.Option) (int32, error) {
	return 0, nil
}
func (r *menuRepoServiceStub) ListAll(context.Context) ([]*pb.Menu, error) {
	return r.menus, nil
}
func (*menuRepoServiceStub) ListMenus(context.Context, ...listing.Option) ([]*pb.Menu, error) {
	return nil, nil
}
func (r *menuRepoServiceStub) Delete(_ context.Context, id uint32) error {
	r.deletedID = id
	return nil
}
func (r *menuRepoServiceStub) ExistByName(_ context.Context, req *pb.ExistMenuByNameRequest) (bool, error) {
	r.nameReq = req
	return r.existName, nil
}
func (r *menuRepoServiceStub) ExistByPath(_ context.Context, req *pb.ExistMenuByPathRequest) (bool, error) {
	r.pathReq = req
	return r.existPath, nil
}

func cloneMenu(menu *pb.Menu) *pb.Menu {
	return proto.Clone(menu).(*pb.Menu) //nolint:errcheck // proto.Clone does not return error
}

func newMenuServiceForTest(repo *menuRepoServiceStub) *MenuServiceService {
	return NewMenuServiceService(biz.NewMenuUsecase(repo, log.NewStdLogger(io.Discard)), log.NewStdLogger(io.Discard))
}

func TestMenuServiceBuildsTree(t *testing.T) {
	t.Parallel()

	rootID := uint32(0)
	parentID := uint32(1)
	childParentID := uint32(2)
	repo := &menuRepoServiceStub{menus: []*pb.Menu{
		{Id: 1, ParentId: &rootID},
		{Id: 2, ParentId: &parentID},
		{Id: 3, ParentId: &childParentID},
	}}
	service := newMenuServiceForTest(repo)

	resp, err := service.ListMenusTree(context.Background(), &pb.ListMenusTreeRequest{ParentId: &rootID})
	if err != nil {
		t.Fatalf("ListMenusTree() error = %v", err)
	}
	if len(resp.GetItems()) != 1 || len(resp.GetItems()[0].GetChildren()) != 1 || len(resp.GetItems()[0].GetChildren()[0].GetChildren()) != 1 {
		t.Fatalf("tree = %v", resp.GetItems())
	}
}

func TestMenuServiceExistChecksShortCircuitEmptyValues(t *testing.T) {
	t.Parallel()

	repo := &menuRepoServiceStub{existPath: true, existName: true}
	service := newMenuServiceForTest(repo)

	pathResp, err := service.ExistMenuByPath(context.Background(), &pb.ExistMenuByPathRequest{})
	if err != nil {
		t.Fatalf("ExistMenuByPath(empty) error = %v", err)
	}
	nameResp, err := service.ExistMenuByName(context.Background(), &pb.ExistMenuByNameRequest{})
	if err != nil {
		t.Fatalf("ExistMenuByName(empty) error = %v", err)
	}
	if pathResp.GetExist() || nameResp.GetExist() || repo.pathReq != nil || repo.nameReq != nil {
		t.Fatalf("empty checks should short-circuit: path=%v name=%v pathReq=%v nameReq=%v",
			pathResp.GetExist(), nameResp.GetExist(), repo.pathReq, repo.nameReq)
	}
}

func TestMenuServiceExistChecksDelegateNonEmptyValues(t *testing.T) {
	t.Parallel()

	path := "/system/user"
	name := "用户管理"
	id := uint32(7)
	repo := &menuRepoServiceStub{existPath: true, existName: true}
	service := newMenuServiceForTest(repo)

	pathResp, err := service.ExistMenuByPath(context.Background(), &pb.ExistMenuByPathRequest{Path: path, Id: &id})
	if err != nil {
		t.Fatalf("ExistMenuByPath() error = %v", err)
	}
	nameResp, err := service.ExistMenuByName(context.Background(), &pb.ExistMenuByNameRequest{Name: name, Id: &id})
	if err != nil {
		t.Fatalf("ExistMenuByName() error = %v", err)
	}
	if !pathResp.GetExist() || repo.pathReq.GetPath() != path || repo.pathReq.GetId() != 7 {
		t.Fatalf("path response=%v req=%v", pathResp.GetExist(), repo.pathReq)
	}
	if !nameResp.GetExist() || repo.nameReq.GetName() != name || repo.nameReq.GetId() != 7 {
		t.Fatalf("name response=%v req=%v", nameResp.GetExist(), repo.nameReq)
	}
}

func TestMenuServiceUpdatePreservesFieldsOutsideMask(t *testing.T) {
	t.Parallel()

	oldName := "用户管理"
	newName := "用户中心"
	path := "/system/user"
	newPath := "/system/account"
	repo := &menuRepoServiceStub{current: &pb.Menu{Id: 7, Name: oldName, Path: &path}}
	service := newMenuServiceForTest(repo)

	if _, err := service.UpdateMenu(context.Background(), &pb.UpdateMenuRequest{
		Id:         7,
		Menu:       &pb.Menu{Name: newName, Path: &newPath},
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"path"}},
	}); err != nil {
		t.Fatalf("UpdateMenu() error = %v", err)
	}
	if repo.updated.GetId() != 7 || repo.updated.GetPath() != newPath {
		t.Fatalf("updated menu id=%d path=%q", repo.updated.GetId(), repo.updated.GetPath())
	}
	if !reflect.DeepEqual(repo.updated.GetName(), oldName) {
		t.Fatalf("name with empty mask = %q, want existing %q", repo.updated.GetName(), oldName)
	}
}
