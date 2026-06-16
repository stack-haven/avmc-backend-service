package data

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/aip-go/ents"
	"github.com/go-kratos/kratos/v2/log"

	"backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/menu"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"
)

var _ biz.MenuRepo = (*menuRepo)(nil)

type menuRepo struct {
	BaseRepo
}

func NewMenuRepo(data *Data, logger log.Logger) biz.MenuRepo {
	return &menuRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func (r *menuRepo) validateParent(ctx context.Context, menuID, parentID uint32) error {
	if parentID == 0 {
		return nil
	}
	if menuID != 0 && parentID == menuID {
		return pb.ErrorBadRequest("菜单不能以自身作为上级菜单")
	}
	seen := map[uint32]struct{}{menuID: {}}
	currentID := parentID
	for currentID != 0 {
		if _, ok := seen[currentID]; ok {
			return pb.ErrorBadRequest("菜单层级不能形成循环")
		}
		seen[currentID] = struct{}{}
		parent, err := r.Data.DB(ctx).Menu.Query().
			Where(menu.IDEQ(currentID)).
			Select(menu.FieldID, menu.FieldParentID).
			Only(ctx)
		if err != nil {
			if gen.IsNotFound(err) {
				return pb.ErrorBadRequest("上级菜单不存在")
			}
			return err
		}
		currentID = 0
		if parent.ParentID != nil {
			currentID = *parent.ParentID
		}
	}
	return nil
}

func (r *menuRepo) convertProto(res *gen.Menu) *pbCore.Menu {
	status := enum.Status(0)
	if res.Status != nil {
		status = enum.Status(*res.Status)
	}
	return &pbCore.Menu{
		Id:        res.ID,
		Name:      res.Name,
		ParentId:  res.ParentID,
		Path:      res.Path,
		Component: res.Component,
		Redirect:  res.Redirect,
		Type:      (*pbCore.MenuType)(res.Type),
		AuthCode:  res.AuthCode,
		Meta: &pbCore.MenuMeta{
			Title:              res.Title,
			ActiveIcon:         res.ActiveIcon,
			ActivePath:         res.ActivePath,
			AffixTab:           res.AffixTab,
			AffixTabOrder:      res.AffixTabOrder,
			Badge:              res.Badge,
			BadgeType:          (*pbCore.BadgeType)(res.BadgeType),
			BadgeVariants:      (*pbCore.BadgeVariants)(res.BadgeVariants),
			HideChildrenInMenu: res.HideChildrenInMenu,
			HideInBreadcrumb:   res.HideInBreadcrumb,
			HideInMenu:         res.HideInMenu,
			HideInTab:          res.HideInTab,
			Icon:               res.Icon,
			IframeSrc:          res.IframeSrc,
			KeepAlive:          res.KeepAlive,
			Link:               res.Link,
			MaxNumOfOpenTab:    res.MaxNumOfOpenTab,
			NoBasicLayout:      res.NoBasicLayout,
			OpenInNewWindow:    res.OpenInNewWindow,
			Order:              res.Sort,
			Query:              res.Query,
		},
		Status:    convert.EmptyToNil(status),
		CreatedAt: convert.TimeValueToString(&res.CreatedAt, time.DateTime),
		UpdatedAt: convert.TimeValueToString(&res.UpdatedAt, time.DateTime),
	}
}

func (r *menuRepo) convertEnt(g *pbCore.Menu) *gen.Menu {
	meta := g.GetMeta()
	if meta == nil {
		meta = &pbCore.MenuMeta{}
	}
	return &gen.Menu{
		ID:                 g.GetId(),
		Name:               g.Name,
		ParentID:           g.ParentId,
		Path:               g.Path,
		Component:          g.Component,
		Redirect:           g.Redirect,
		Type:               convert.ToPointer(int32(g.GetType())),
		AuthCode:           g.AuthCode,
		Status:             convert.ToPointer(int32(g.GetStatus())),
		Title:              meta.Title,
		ActiveIcon:         meta.ActiveIcon,
		ActivePath:         meta.ActivePath,
		AffixTab:           meta.AffixTab,
		AffixTabOrder:      meta.AffixTabOrder,
		Badge:              meta.Badge,
		BadgeType:          convert.EmptyToNil(int32(meta.GetBadgeType())),
		BadgeVariants:      convert.EmptyToNil(int32(meta.GetBadgeVariants())),
		HideChildrenInMenu: meta.HideChildrenInMenu,
		HideInBreadcrumb:   meta.HideInBreadcrumb,
		HideInMenu:         meta.HideInMenu,
		HideInTab:          meta.HideInTab,
		Icon:               meta.Icon,
		IframeSrc:          meta.IframeSrc,
		KeepAlive:          meta.KeepAlive,
		Link:               meta.Link,
		MaxNumOfOpenTab:    meta.MaxNumOfOpenTab,
		NoBasicLayout:      meta.NoBasicLayout,
		OpenInNewWindow:    meta.OpenInNewWindow,
		Sort:               meta.Order,
		Query:              meta.Query,
	}
}

func (r *menuRepo) Save(ctx context.Context, g *pbCore.Menu) (*pbCore.Menu, error) {
	if g == nil || g.GetName() == "" {
		return nil, pb.ErrorMenuNameCannotBeEmpty("菜单名称不能为空")
	}
	r.Log.Infof("保存菜单: %v", g.Name)
	entMenu := r.convertEnt(g)
	if err := r.validateParent(ctx, 0, g.GetParentId()); err != nil {
		return nil, err
	}

	exist, err := r.ExistByName(ctx, &pbCore.ExistMenuByNameRequest{
		Name: g.Name,
		Id:   &g.Id,
	})
	if err != nil {
		return nil, fmt.Errorf("checking menu name uniqueness: %w", err)
	}
	if exist {
		return nil, pb.ErrorMenuNameAlreadyExists("菜单名称已存在")
	}
	if g.GetPath() != "" {
		exist, err = r.ExistByPath(ctx, &pbCore.ExistMenuByPathRequest{Path: g.GetPath(), Id: &g.Id})
		if err != nil {
			return nil, fmt.Errorf("checking menu path uniqueness: %w", err)
		}
		if exist {
			return nil, pb.ErrorMenuPathAlreadyExists("菜单路径已存在")
		}
	}

	res, err := r.Data.DB(ctx).Menu.Create().
		SetName(entMenu.Name).
		SetNillableTitle(entMenu.Title).
		SetNillableParentID(entMenu.ParentID).
		SetNillablePath(entMenu.Path).
		SetNillableComponent(entMenu.Component).
		SetNillableRedirect(entMenu.Redirect).
		SetNillableType(entMenu.Type).
		SetNillableStatus(entMenu.Status).
		SetNillableAuthCode(entMenu.AuthCode).
		SetNillableActiveIcon(entMenu.ActiveIcon).
		SetNillableActivePath(entMenu.ActivePath).
		SetNillableAffixTab(entMenu.AffixTab).
		SetNillableAffixTabOrder(entMenu.AffixTabOrder).
		SetNillableBadge(entMenu.Badge).
		SetNillableBadgeType(entMenu.BadgeType).
		SetNillableBadgeVariants(entMenu.BadgeVariants).
		SetNillableHideChildrenInMenu(entMenu.HideChildrenInMenu).
		SetNillableHideInBreadcrumb(entMenu.HideInBreadcrumb).
		SetNillableHideInMenu(entMenu.HideInMenu).
		SetNillableHideInTab(entMenu.HideInTab).
		SetNillableIcon(entMenu.Icon).
		SetNillableIframeSrc(entMenu.IframeSrc).
		SetNillableKeepAlive(entMenu.KeepAlive).
		SetNillableLink(entMenu.Link).
		SetNillableMaxNumOfOpenTab(entMenu.MaxNumOfOpenTab).
		SetNillableNoBasicLayout(entMenu.NoBasicLayout).
		SetNillableOpenInNewWindow(entMenu.OpenInNewWindow).
		SetNillableSort(entMenu.Sort).
		SetNillableQuery(entMenu.Query).
		Save(ctx)
	if err != nil {
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorMenuAlreadyExists("菜单名称或路径已存在")
		}
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *menuRepo) Update(ctx context.Context, g *pbCore.Menu) (*pbCore.Menu, error) {
	if g == nil || g.GetId() == 0 || g.GetName() == "" {
		return nil, pb.ErrorMenuInvalidId("菜单ID和名称不能为空")
	}
	entMenu := r.convertEnt(g)
	if err := r.validateParent(ctx, g.GetId(), g.GetParentId()); err != nil {
		return nil, err
	}
	exist, err := r.ExistByName(ctx, &pbCore.ExistMenuByNameRequest{
		Name: g.Name,
		Id:   &g.Id,
	})
	if err != nil {
		return nil, fmt.Errorf("checking menu name uniqueness: %w", err)
	}
	if exist {
		return nil, pb.ErrorMenuNameAlreadyExists("菜单名称已存在")
	}
	if g.GetPath() != "" {
		exist, err = r.ExistByPath(ctx, &pbCore.ExistMenuByPathRequest{Path: g.GetPath(), Id: &g.Id})
		if err != nil {
			return nil, fmt.Errorf("checking menu path uniqueness: %w", err)
		}
		if exist {
			return nil, pb.ErrorMenuPathAlreadyExists("菜单路径已存在")
		}
	}

	res, err := r.Data.DB(ctx).Menu.UpdateOneID(g.GetId()).
		SetName(entMenu.Name).
		SetNillableTitle(entMenu.Title).
		SetNillableParentID(entMenu.ParentID).
		SetNillablePath(entMenu.Path).
		SetNillableComponent(entMenu.Component).
		SetNillableRedirect(entMenu.Redirect).
		SetNillableType(entMenu.Type).
		SetNillableStatus(entMenu.Status).
		SetNillableAuthCode(entMenu.AuthCode).
		SetNillableActiveIcon(entMenu.ActiveIcon).
		SetNillableActivePath(entMenu.ActivePath).
		SetNillableAffixTab(entMenu.AffixTab).
		SetNillableAffixTabOrder(entMenu.AffixTabOrder).
		SetNillableBadge(entMenu.Badge).
		SetNillableBadgeType(entMenu.BadgeType).
		SetNillableBadgeVariants(entMenu.BadgeVariants).
		SetNillableHideChildrenInMenu(entMenu.HideChildrenInMenu).
		SetNillableHideInBreadcrumb(entMenu.HideInBreadcrumb).
		SetNillableHideInMenu(entMenu.HideInMenu).
		SetNillableHideInTab(entMenu.HideInTab).
		SetNillableIcon(entMenu.Icon).
		SetNillableIframeSrc(entMenu.IframeSrc).
		SetNillableKeepAlive(entMenu.KeepAlive).
		SetNillableLink(entMenu.Link).
		SetNillableMaxNumOfOpenTab(entMenu.MaxNumOfOpenTab).
		SetNillableNoBasicLayout(entMenu.NoBasicLayout).
		SetNillableOpenInNewWindow(entMenu.OpenInNewWindow).
		SetNillableSort(entMenu.Sort).
		SetNillableQuery(entMenu.Query).
		Save(ctx)
	if err != nil {
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorMenuAlreadyExists("菜单名称或路径已存在")
		}
		if gen.IsNotFound(err) {
			return nil, pb.ErrorMenuNotFound("菜单不存在")
		}
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *menuRepo) FindByID(ctx context.Context, id uint32) (*pbCore.Menu, error) {
	res, err := r.Data.DB(ctx).Menu.Query().Where(menu.IDEQ(id)).Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorMenuNotFound("菜单不存在")
		}
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *menuRepo) Delete(ctx context.Context, id uint32) error {
	hasChildren, err := r.Data.DB(ctx).Menu.Query().Where(menu.ParentIDEQ(id)).Exist(ctx)
	if err != nil {
		return err
	}
	if hasChildren {
		return pb.ErrorMenuCannotDeleteWithChildren("存在子菜单，无法删除")
	}
	err = r.Data.DB(ctx).Menu.DeleteOneID(id).Exec(ctx)
	if gen.IsNotFound(err) {
		return pb.ErrorMenuNotFound("菜单不存在")
	}
	return err
}

func (r *menuRepo) ListByName(ctx context.Context, name string) ([]*pbCore.Menu, error) {
	res, err := r.Data.DB(ctx).Menu.Query().Where(menu.NameContains(name)).All(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return convert.SliceToAny(res, r.convertProto), nil
}

func (r *menuRepo) ListAllSimple(ctx context.Context) ([]*pbCore.Menu, error) {
	res, err := r.Data.DB(ctx).Menu.Query().Select(menu.FieldID, menu.FieldName).Order(gen.Desc(menu.FieldID)).All(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return convert.SliceToAny(res, r.convertProto), nil
}

func (r *menuRepo) CountMenus(ctx context.Context, opts ...listing.Option) (int32, error) {
	o := listing.Options{}
	for _, opt := range opts {
		opt(&o)
	}
	count, err := r.Data.DB(ctx).Menu.Query().
		Select(menu.FieldID).
		Where(ents.ApplyFilter(o.Filter)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

func (r *menuRepo) ListAll(ctx context.Context) ([]*pbCore.Menu, error) {
	res, err := r.Data.DB(ctx).Menu.Query().Order(gen.Desc(menu.FieldSort, menu.FieldID)).All(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return convert.SliceToAny(res, r.convertProto), nil
}

func (r *menuRepo) ListMenus(ctx context.Context, opts ...listing.Option) ([]*pbCore.Menu, error) {
	o := listing.Options{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	pos, err := r.Data.DB(ctx).Menu.Query().
		Select(menu.FieldID, menu.FieldName, menu.FieldTitle, menu.FieldParentID, menu.FieldPath, menu.FieldComponent, menu.FieldType, menu.FieldStatus, menu.FieldCreatedAt, menu.FieldUpdatedAt).
		Where(ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(pos, r.convertProto), nil
}

func (r *menuRepo) ExistByPath(ctx context.Context, req *pbCore.ExistMenuByPathRequest) (bool, error) {
	builder := r.Data.DB(ctx).Menu.Query()
	if req.GetId() != 0 {
		builder = builder.Where(menu.Not(menu.IDEQ(req.GetId())))
	}
	_, err := builder.Select(menu.FieldID).Where(menu.Path(req.GetPath())).First(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *menuRepo) ExistByName(ctx context.Context, req *pbCore.ExistMenuByNameRequest) (bool, error) {
	builder := r.Data.DB(ctx).Menu.Query()
	if req.GetId() != 0 {
		builder = builder.Where(menu.IDNotIn(req.GetId()))
	}
	_, err := builder.Select(menu.FieldID).Where(menu.Name(req.GetName())).First(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
