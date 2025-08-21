package data

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	"backend-service/api/common/enum"
	pbPagination "backend-service/api/common/pagination"
	pbCore "backend-service/api/core/service/v1"
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/data/ent/gen"
	"backend-service/app/avmc/admin/internal/data/ent/gen/menu"
	"backend-service/pkg/utils/convert"
)

var _ biz.MenuRepo = (*menuRepo)(nil)

type menuRepo struct {
	data *Data
	log  *log.Helper
}

// NewMenuRepo 创建新的菜单仓库实例
// 参数：data 数据访问层实例，logger 日志记录器
// 返回值：菜单仓库实例指针
func NewMenuRepo(data *Data, logger log.Logger) biz.MenuRepo {
	return &menuRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// toProto 转换gen.Menu为pbCore.Menu
func (r *menuRepo) toProto(res *gen.Menu) *pbCore.Menu {
	return &pbCore.Menu{
		Id:        res.ID,
		Name:      res.Name,
		Pid:       res.Pid,
		Path:      res.Path,
		Component: res.Component,
		Type:      res.Type,
		AuthCode:  res.AuthCode,
		Meta: &pbCore.MenuMeta{
			Title:              res.Title,
			ActiveIcon:         res.ActiveIcon,
			ActivePath:         res.ActivePath,
			AffixTab:           res.AffixTab,
			AffixTabOrder:      res.AffixTabOrder,
			Badge:              res.Badge,
			BadgeType:          res.BadgeType,
			BadgeVariants:      res.BadgeVariants,
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
		Status:    (*enum.Status)(res.Status),
		CreatedAt: convert.TimeValueToString(&res.CreatedAt, time.DateTime),
		UpdatedAt: convert.TimeValueToString(&res.UpdatedAt, time.DateTime),
	}
}

// toEnt 转换pbCore.Menu为gen.Menu
func (r *menuRepo) toEnt(g *pbCore.Menu) *gen.Menu {
	return &gen.Menu{
		ID:                 g.GetId(),
		Name:               g.Name,
		Pid:                g.Pid,
		Path:               g.Path,
		Component:          g.Component,
		Redirect:           g.Redirect,
		Type:               g.Type,
		Status:             (*int32)(g.Status),
		Title:              g.Meta.Title,
		AuthCode:           g.AuthCode,
		ActiveIcon:         g.Meta.ActiveIcon,
		ActivePath:         g.Meta.ActivePath,
		AffixTab:           g.Meta.AffixTab,
		AffixTabOrder:      g.Meta.AffixTabOrder,
		Badge:              g.Meta.Badge,
		BadgeType:          g.Meta.BadgeType,
		BadgeVariants:      g.Meta.BadgeVariants,
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

// Save 保存菜单信息
// 参数：ctx 上下文，g 菜单信息
// 返回值：菜单信息，错误信息
func (r *menuRepo) Save(ctx context.Context, g *pbCore.Menu) (*pbCore.Menu, error) {
	r.log.Infof("保存菜单，菜单信息：%v", g)
	entMenu := r.toEnt(g)
	builder := r.data.DB(ctx).Menu.Create()

	id, _ := r.GetMenuExistByName(ctx, entMenu.Name)
	if id > 0 {
		r.log.Errorf("菜单名称已存在，菜单信息：%v", g)
		return nil, fmt.Errorf("menu name already exists")
	}

	res, err := builder.SetName(entMenu.Name).
		SetNillableTitle(entMenu.Title).
		SetNillablePid(&entMenu.Pid).
		SetNillablePath(entMenu.Path).
		SetNillableComponent(entMenu.Component).
		SetNillableRedirect(entMenu.Redirect).
		SetNillableType(&entMenu.Type).
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
		r.log.Errorf("保存菜单失败，菜单信息：%v，错误：%v", g, err)
		return nil, err
	}
	return r.toProto(res), nil
}

// GetMenuExistByName 获取菜单名称是否存在
// 参数：ctx 上下文，name 菜单名称
// 返回值：菜单ID，错误信息
func (r *menuRepo) GetMenuExistByName(ctx context.Context, name string) (uint32, error) {
	r.log.Infof("获取菜单名称是否存在，菜单名称：%v", name)
	entMenu, err := r.data.DB(ctx).Menu.Query().Where(menu.Name(name)).Select(menu.FieldID).First(ctx)
	if err != nil {
		r.log.Errorf("获取菜单名称是否存在失败，菜单名称：%v，错误：%v", name, err)
		return 0, err
	}
	return entMenu.ID, nil
}

// Update 更新菜单信息
// 参数：ctx 上下文，g 菜单信息
// 返回值：菜单信息，错误信息
func (r *menuRepo) Update(ctx context.Context, g *pbCore.Menu) (*pbCore.Menu, error) {
	r.log.Infof("更新菜单，菜单信息：%v", g)
	entMenu := r.toEnt(g)
	builder := r.data.DB(ctx).Menu.UpdateOneID(g.GetId())
	id, _ := r.GetMenuExistByName(ctx, entMenu.Name)
	if id > 0 && id != g.GetId() {
		r.log.Errorf("菜单名称已存在，菜单信息：%v", g)
		return nil, fmt.Errorf("menu name already exists")
	}

	res, err := builder.
		SetName(entMenu.Name).
		SetNillableTitle(entMenu.Title).
		SetNillablePid(&entMenu.Pid).
		SetNillablePath(entMenu.Path).
		SetNillableComponent(entMenu.Component).
		SetNillableRedirect(entMenu.Redirect).
		SetNillableType(&entMenu.Type).
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
		r.log.Errorf("更新菜单失败，菜单信息：%v，错误：%v", g, err)
		return nil, err
	}
	return r.toProto(res), nil
}

// FindByID 通过ID查询菜单信息
// 参数：ctx 上下文，id 菜单ID
// 返回值：菜单信息，错误信息
func (r *menuRepo) FindByID(ctx context.Context, id uint32) (*pbCore.Menu, error) {
	r.log.Infof("通过ID查询菜单，ID：%d", id)
	res, err := r.data.DB(ctx).Menu.Query().
		Where(menu.IDEQ(id)).Only(ctx)
	if err != nil {
		r.log.Errorf("通过ID查询菜单失败，ID：%d，错误：%v", id, err)
		if gen.IsNotFound(err) {
			return nil, errors.New("查询数据不存在")
		}
		return nil, err
	}
	return r.toProto(res), nil
}

// Delete 删除菜单
// 参数：ctx 上下文，id 菜单ID
// 返回值：错误信息
func (r *menuRepo) Delete(ctx context.Context, id uint32) error {
	r.log.Infof("删除菜单，菜单ID：%d", id)
	err := r.data.DB(ctx).Menu.UpdateOneID(id).SetDeletedAt(time.Now()).Exec(ctx)
	if err != nil {
		r.log.Errorf("删除菜单失败，菜单ID：%d，错误：%v", id, err)
		return err
	}
	return nil
}

// ListByName 通过菜单名称查询菜单列表
// 参数：ctx 上下文，name 菜单名称
// 返回值：菜单列表，错误信息
func (r *menuRepo) ListByName(ctx context.Context, name string) ([]*pbCore.Menu, error) {
	r.log.Infof("通过菜单名称查询菜单，菜单名称：%s", name)
	res, err := r.data.DB(ctx).Menu.Query().Where(menu.NameContains(name)).All(ctx)
	if err != nil {
		r.log.Errorf("通过菜单名称查询菜单失败，菜单名称：%s，错误：%v", name, err)
		return nil, err
	}
	return convert.SliceToAny(res, r.toProto), nil
}

// ListAll 查询所有菜单列表
// 参数：ctx 上下文
// 返回值：菜单列表，错误信息
func (r *menuRepo) ListAllSimple(ctx context.Context) ([]*pbCore.Menu, error) {
	r.log.Infof("查询所有菜单列表")
	res, err := r.data.DB(ctx).Menu.Query().Select(menu.FieldID, menu.FieldName).Where().Order(gen.Desc(menu.FieldID)).All(ctx)
	if err != nil {
		r.log.Errorf("查询所有菜单列表失败，错误：%v", err)
		return nil, err
	}
	return convert.SliceToAny(res, r.toProto), nil
}

// ListAll 查询所有菜单列表
// 参数：ctx 上下文
// 返回值：菜单列表，错误信息
func (r *menuRepo) ListAll(ctx context.Context) ([]*pbCore.Menu, error) {
	r.log.Infof("查询所有菜单列表")
	res, err := r.data.DB(ctx).Menu.Query().Where().Order(gen.Desc(menu.FieldSort, menu.FieldID)).All(ctx)
	if err != nil {
		r.log.Errorf("查询所有菜单列表失败，错误：%v", err)
		return nil, err
	}
	return convert.SliceToAny(res, r.toProto), nil
}

// ListPage 查询菜单列表分页
// 参数：ctx 上下文，pagination 分页请求
// 返回值：菜单列表响应，错误信息
func (r *menuRepo) ListPage(ctx context.Context, pagination *pbPagination.PagingRequest) (*pbCore.ListMenuResponse, error) {
	r.log.Infof("查询菜单列表分页，分页请求：%v", pagination)
	count, err := r.data.DB(ctx).Menu.Query().Select(menu.FieldID).
		// Where(menu.DeletedAtIsNil()).
		Count(ctx)
	if err != nil {
		r.log.Errorf("查询所有菜单列表失败，错误：%v", err)
		return nil, err
	}
	res, err := r.data.DB(ctx).Menu.Query().
		Select(
			menu.FieldID,
			menu.FieldName,
			menu.FieldTitle,
			menu.FieldPid,
			menu.FieldPath,
			menu.FieldComponent,
			menu.FieldType,
			menu.FieldStatus,
			menu.FieldCreatedAt,
			menu.FieldUpdatedAt,
		).
		Offset(int((pagination.GetPage() - 1) * pagination.GetPageSize())).
		Limit(int(pagination.GetPageSize())).
		Order(gen.Desc(menu.FieldID)).
		All(ctx)
	if err != nil {
		r.log.Errorf("查询菜单列表分页失败，分页请求：%v，错误：%v", pagination, err)
		return nil, err
	}
	return &pbCore.ListMenuResponse{
		Items: convert.SliceToAny(res, r.toProto),
		Total: int32(count),
	}, nil
}
