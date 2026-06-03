package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/aip-go/ents"
	"github.com/go-kratos/kratos/v2/log"

	"backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/data/ent/gen"
	"backend-service/app/avmc/admin/internal/data/ent/gen/menu"
	"backend-service/pkg/utils/convert"
)

var _ biz.MenuRepo = (*menuRepo)(nil)

type menuRepo struct {
	BaseRepo
}

func NewMenuRepo(data *Data, logger log.Logger) biz.MenuRepo {
	return &menuRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func (r *menuRepo) convertProto(res *gen.Menu) *pbCore.Menu {
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
		Status:    convert.EmptyToNil(enum.Status(*res.Status)),
		CreatedAt: convert.TimeValueToString(&res.CreatedAt, time.DateTime),
		UpdatedAt: convert.TimeValueToString(&res.UpdatedAt, time.DateTime),
	}
}

func (r *menuRepo) convertEnt(g *pbCore.Menu) *gen.Menu {
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
		Title:              g.Meta.Title,
		ActiveIcon:         g.Meta.ActiveIcon,
		ActivePath:         g.Meta.ActivePath,
		AffixTab:           g.Meta.AffixTab,
		AffixTabOrder:      g.Meta.AffixTabOrder,
		Badge:              g.Meta.Badge,
		BadgeType:          convert.EmptyToNil(int32(g.Meta.GetBadgeType())),
		BadgeVariants:      convert.EmptyToNil(int32(g.Meta.GetBadgeVariants())),
		HideChildrenInMenu: g.Meta.HideChildrenInMenu,
		HideInBreadcrumb:   g.Meta.HideInBreadcrumb,
		HideInMenu:         g.Meta.HideInMenu,
		HideInTab:          g.Meta.HideInTab,
		Icon:               g.Meta.Icon,
		IframeSrc:          g.Meta.IframeSrc,
		KeepAlive:          g.Meta.KeepAlive,
		Link:               g.Meta.Link,
		MaxNumOfOpenTab:    g.Meta.MaxNumOfOpenTab,
		NoBasicLayout:      g.Meta.NoBasicLayout,
		OpenInNewWindow:    g.Meta.OpenInNewWindow,
		Sort:               g.Meta.Order,
		Query:              g.Meta.Query,
	}
}

func (r *menuRepo) Save(ctx context.Context, g *pbCore.Menu) (*pbCore.Menu, error) {
	r.Log.Infof("保存菜单: %v", g.Name)
	entMenu := r.convertEnt(g)

	if exist, _ := r.ExistByName(ctx, &pbCore.ExistMenuByNameRequest{
		Name: g.Name,
		Id:   &g.Id,
	}); exist {
		return nil, fmt.Errorf("menu name already exists")
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
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *menuRepo) Update(ctx context.Context, g *pbCore.Menu) (*pbCore.Menu, error) {
	entMenu := r.convertEnt(g)
	if exist, _ := r.ExistByName(ctx, &pbCore.ExistMenuByNameRequest{
		Name: g.Name,
		Id:   &g.Id,
	}); exist {
		return nil, fmt.Errorf("menu name already exists")
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
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *menuRepo) FindByID(ctx context.Context, id uint32) (*pbCore.Menu, error) {
	res, err := r.Data.DB(ctx).Menu.Query().Where(menu.IDEQ(id)).Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, errors.New("查询数据不存在")
		}
		return nil, err
	}
	return r.convertProto(res), nil
}

func (r *menuRepo) Delete(ctx context.Context, id uint32) error {
	return r.Data.DB(ctx).Menu.DeleteOneID(id).Exec(ctx)
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

func (r *menuRepo) CountMenus(ctx context.Context, opts ...biz.ListOption) (int32, error) {
	o := biz.ListOptions{}
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

func (r *menuRepo) ListMenus(ctx context.Context, opts ...biz.ListOption) ([]*pbCore.Menu, error) {
	o := biz.ListOptions{Limit: 20}
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
