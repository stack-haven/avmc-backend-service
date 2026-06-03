package data

import (
	"backend-service/app/avmc/admin/internal/biz"
	"backend-service/app/avmc/admin/internal/data/ent/gen"
	"backend-service/app/avmc/admin/internal/data/ent/gen/menu"
	"backend-service/app/avmc/admin/internal/data/ent/gen/role"
	"backend-service/app/avmc/admin/internal/data/ent/gen/user"
	"backend-service/pkg/auth"
	"backend-service/pkg/utils/convert"
	"backend-service/pkg/utils/crypto"
	"context"
	"sort"
	"time"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/avmc/admin/v1"
	"backend-service/api/common/enum"

	pbCore "backend-service/api/core/service/v1"
)

// AuthRepo 数据仓库结构体
// 包含日志记录器
type authRepo struct {
	data *Data
	log  *log.Helper
	atr  *auth.AuthToken
	ur   *userRepo
	mr   *menuRepo
}

// NewAuthRepo 创建新的用户数据仓库实例
// 参数：logger 日志记录器
// 返回值：用户数据仓库实例指针
func NewAuthRepo(data *Data, atr *auth.AuthToken, logger log.Logger) biz.AuthRepo {
	return &authRepo{
		data: data,
		log:  log.NewHelper(logger),
		atr:  atr,
		ur:   NewUserRepo(data, logger).(*userRepo),
		mr:   NewMenuRepo(data, logger).(*menuRepo),
	}
}

// LoginResponse 处理登陆统一返回
// 参数：ctx 上下文，res 用户实体，accessToken 访问令牌，refreshToken 刷新令牌，expires 过期时间
// 返回值：登录响应结构体
func (r *authRepo) LoginResponse(ctx context.Context, u *gen.User) (*pb.LoginResponse, error) {
	accessToken, refreshToken, err := r.atr.GenerateToken(ctx, auth.AuthTokenInfo{
		UserId:   u.ID,
		Username: convert.ToValue(u.Name),
		DomainId: u.DomainID,
	})
	if err != nil {
		r.log.Errorf("登录数据操作失败，Token生成错误错误：%v", err)
		return nil, err
	}
	// 拼装具体过期时间
	expires := convert.TimeValueToString(func(exp time.Duration) *time.Time {
		t := time.Now().Add(exp)
		return &t
	}(r.atr.Authenticator.Options().TokenExpiration), time.RFC3339)
	return &pb.LoginResponse{
		Id:           u.ID,
		Name:         u.Name,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expires,
	}, nil
}

// LoginByUsername 处理后台用户名登录数据操作
// 参数：ctx 上下文，name 用户名，password 密码
// 返回值：登录响应结构体，错误信息
func (r *authRepo) LoginByUsername(ctx context.Context, name, password string, domainId uint32) (*pb.LoginResponse, error) {
	// 这里实现具体的登录数据操作
	r.log.Infof("尝试登录数据操作，用户名：%s", name)
	res, err := r.data.DB(ctx).User.Query().Select(user.FieldPassword, user.FieldName, user.FieldDomainID).Where(user.NameEQ(name), user.DomainIDEQ(domainId)).Only(ctx)
	if err != nil {
		r.log.Errorf("登录数据操作失败，用户名：%s，错误：%v", name, err)
		return nil, err
	}
	if res.Password == nil {
		r.log.Errorf("登录数据操作失败，用户名：%s，密码未设置", name)
		return nil, biz.ErrPasswordIncorrect
	}
	if !crypto.CheckPasswordHash(password, *res.Password) {
		r.log.Errorf("登录数据操作失败，用户名：%s，密码错误", name)
		return nil, biz.ErrPasswordIncorrect
	}
	return r.LoginResponse(ctx, res)
}

// LoginByEmail 处理后台邮箱登录数据操作
// 参数：ctx email 邮箱，password 密码
// 返回值：登录响应结构体，错误信息
func (r *authRepo) LoginByEmail(ctx context.Context, email, password string, domainId uint32) (*pb.LoginResponse, error) {
	// 这里实现具体的登录数据操作
	r.log.Infof("尝试登录数据操作，邮箱：%s", email)
	res, err := r.data.DB(ctx).User.Query().Select(user.FieldPassword, user.FieldName, user.FieldDomainID).Where(user.EmailEQ(email), user.DomainIDEQ(domainId)).Only(ctx)
	if err != nil {
		r.log.Errorf("登录数据操作失败，邮箱：%s，错误：%v", email, err)
		return nil, err
	}
	if res.Password == nil {
		r.log.Errorf("登录数据操作失败，邮箱：%s，密码未设置", email)
		return nil, biz.ErrPasswordIncorrect
	}
	if !crypto.CheckPasswordHash(password, *res.Password) {
		r.log.Errorf("登录数据操作失败，邮箱：%s，密码错误", email)
		return nil, biz.ErrPasswordIncorrect
	}
	return r.LoginResponse(ctx, res)
}

// RefreshToken 处理刷新令牌数据操作
// 参数：ctx 上下文，refreshToken 刷新令牌
// 返回值：刷新令牌响应结构体，错误信息
func (r *authRepo) RefreshToken(ctx context.Context, refreshToken string) (*pb.RefreshTokenResponse, error) {
	r.log.Infof("尝试刷新令牌数据操作")
	claims, err := r.atr.ValidateRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, pb.ErrorAuthInvalidToken("刷新令牌无效")
	}
	userID := convert.StringToUnit32(claims.GetSubject())
	domainID := convert.StringToUnit32(claims.GetDomain())
	if userID == 0 {
		return nil, pb.ErrorAuthInvalidToken("刷新令牌主体无效")
	}
	res, err := r.data.DB(ctx).User.Query().
		Select(user.FieldName, user.FieldDomainID).
		Where(user.IDEQ(userID)).
		Only(ctx)
	if err != nil {
		r.log.Errorf("刷新令牌用户查询失败，用户ID：%d，错误：%v", userID, err)
		return nil, err
	}
	if domainID != 0 && res.DomainID != domainID {
		return nil, pb.ErrorAuthInvalidToken("刷新令牌域无效")
	}
	accessToken, newRefreshToken, err := r.atr.GenerateToken(ctx, auth.AuthTokenInfo{
		UserId:   userID,
		Username: convert.ToValue(res.Name),
		DomainId: res.DomainID,
	})
	if err != nil {
		r.log.Errorf("刷新令牌重新签发失败，用户ID：%d，错误：%v", userID, err)
		return nil, err
	}
	return &pb.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

// Logout 处理后台登出数据操作
// 参数：ctx 上下文
// 返回值：错误信息
func (r *authRepo) Logout(ctx context.Context, userId uint32) error {
	// 这里实现具体的登出数据操作
	r.log.Infof("尝试登出数据操作，用户ID：%d", userId)
	return r.atr.RemoveToken(ctx, userId)
}

// Register 处理注册数据操作
// 参数：ctx 上下文，name 用户名，password 密码
// 返回值：错误信息
func (r *authRepo) Register(ctx context.Context, name, password string) error {
	// 这里实现具体的注册数据操作
	r.log.Infof("尝试注册数据操作，用户名：%s", name)
	hashPassword, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}
	_, err = r.data.DB(ctx).User.Create().SetName(name).SetPassword(hashPassword).Save(ctx)
	if err != nil {
		r.log.Errorf("注册数据操作失败，用户名：%s，错误：%v", name, err)
		return err
	}
	return nil
}

// Profile 获取用户简介信息
// 参数：ctx 上下文，userId 用户ID
// 返回值：用户简介信息响应结构体，错误信息
func (r *authRepo) Profile(ctx context.Context, userId uint32) (*pb.ProfileResponse, error) {
	// 这里实现具体的获取用户简介信息数据操作
	r.log.Infof("尝试获取用户简介信息数据操作，用户ID：%d", userId)
	user, err := r.ur.FindByID(ctx, userId)
	if err != nil {
		r.log.Errorf("获取用户简介信息数据操作失败，用户ID：%d，错误：%v", userId, err)
		return nil, err
	}
	return &pb.ProfileResponse{
		User: user,
	}, nil
}

// Codes 获取用户权限码
// 参数：ctx 上下文，userId 用户ID
// 返回值：用户权限码响应结构体，错误信息
func (r *authRepo) Codes(ctx context.Context, userId uint32) ([]string, error) {
	// 这里实现具体的获取用户权限码数据操作
	r.log.Infof("尝试获取用户权限码数据操作，用户ID：%d", userId)
	menus, err := r.userMenus(ctx, userId)
	if err != nil {
		return nil, err
	}
	codeSet := make(map[string]struct{}, len(menus))
	for _, m := range menus {
		if m.AuthCode == nil || *m.AuthCode == "" {
			continue
		}
		codeSet[*m.AuthCode] = struct{}{}
	}
	codes := make([]string, 0, len(codeSet))
	for code := range codeSet {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes, nil
}

// Menus 获取用户菜单
// 参数：ctx 上下文，userId 用户ID
// 返回值：用户菜单响应结构体，错误信息
func (r *authRepo) Menus(ctx context.Context, userId uint32) ([]*pbCore.Menu, error) {
	// 这里实现具体的获取用户菜单数据操作
	r.log.Infof("尝试获取用户菜单数据操作，用户ID：%d", userId)

	menus, err := r.userMenus(ctx, userId)
	if err != nil {
		return nil, err
	}
	menus, err = r.withMenuAncestors(ctx, menus)
	if err != nil {
		return nil, err
	}
	return buildMenuTree(convert.SliceToAny(menus, r.mr.convertProto)), nil
}

func (r *authRepo) userMenus(ctx context.Context, userId uint32) ([]*gen.Menu, error) {
	u, err := r.data.DB(ctx).User.Query().
		Select(user.FieldDomainID).
		Where(user.IDEQ(userId)).
		Only(ctx)
	if err != nil {
		r.log.Errorf("查询用户域失败，用户ID：%d，错误：%v", userId, err)
		return nil, err
	}
	menus, err := r.data.DB(ctx).User.Query().
		Where(user.IDEQ(userId)).
		QueryRoles().
		Where(
			role.StatusEQ(int32(enum.Status_STATUS_ENABLED)),
			role.DomainIDEQ(u.DomainID),
		).
		QueryMenus().
		Where(menu.StatusEQ(int32(enum.Status_STATUS_ENABLED))).
		Order(gen.Asc(menu.FieldSort, menu.FieldID)).
		All(ctx)
	if err != nil {
		r.log.Errorf("查询用户菜单失败，用户ID：%d，错误：%v", userId, err)
		return nil, err
	}

	menuMap := make(map[uint32]*gen.Menu, len(menus))
	for _, m := range menus {
		menuMap[m.ID] = m
	}
	ids := make([]uint32, 0, len(menuMap))
	for id := range menuMap {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		left, right := menuMap[ids[i]], menuMap[ids[j]]
		if convert.ToValue(left.Sort) == convert.ToValue(right.Sort) {
			return left.ID < right.ID
		}
		return convert.ToValue(left.Sort) < convert.ToValue(right.Sort)
	})

	result := make([]*gen.Menu, 0, len(ids))
	for _, id := range ids {
		result = append(result, menuMap[id])
	}
	return result, nil
}

func (r *authRepo) withMenuAncestors(ctx context.Context, menus []*gen.Menu) ([]*gen.Menu, error) {
	menuMap := make(map[uint32]*gen.Menu, len(menus))
	for _, m := range menus {
		if m != nil {
			menuMap[m.ID] = m
		}
	}
	for {
		parentIDs := make([]uint32, 0)
		seen := make(map[uint32]struct{})
		for _, m := range menuMap {
			parentID := convert.ToValue(m.ParentID)
			if parentID == 0 {
				continue
			}
			if _, ok := menuMap[parentID]; ok {
				continue
			}
			if _, ok := seen[parentID]; ok {
				continue
			}
			seen[parentID] = struct{}{}
			parentIDs = append(parentIDs, parentID)
		}
		if len(parentIDs) == 0 {
			break
		}
		parents, err := r.data.DB(ctx).Menu.Query().
			Where(
				menu.IDIn(parentIDs...),
				menu.StatusEQ(int32(enum.Status_STATUS_ENABLED)),
			).
			All(ctx)
		if err != nil {
			return nil, err
		}
		if len(parents) == 0 {
			break
		}
		for _, parent := range parents {
			menuMap[parent.ID] = parent
		}
	}

	ids := make([]uint32, 0, len(menuMap))
	for id := range menuMap {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		left, right := menuMap[ids[i]], menuMap[ids[j]]
		if convert.ToValue(left.Sort) == convert.ToValue(right.Sort) {
			return left.ID < right.ID
		}
		return convert.ToValue(left.Sort) < convert.ToValue(right.Sort)
	})
	result := make([]*gen.Menu, 0, len(ids))
	for _, id := range ids {
		result = append(result, menuMap[id])
	}
	return result, nil
}

func buildMenuTree(menus []*pbCore.Menu) []*pbCore.Menu {
	nodes := make(map[uint32]*pbCore.Menu, len(menus))
	roots := make([]*pbCore.Menu, 0, len(menus))
	for _, m := range menus {
		if m == nil || m.GetType() == pbCore.MenuType_MENU_TYPE_BUTTON {
			continue
		}
		m.Children = nil
		nodes[m.GetId()] = m
	}
	for _, m := range menus {
		if m == nil || m.GetType() == pbCore.MenuType_MENU_TYPE_BUTTON {
			continue
		}
		parentID := m.GetParentId()
		if parentID == 0 {
			roots = append(roots, m)
			continue
		}
		if parent, ok := nodes[parentID]; ok {
			parent.Children = append(parent.Children, m)
			continue
		}
		roots = append(roots, m)
	}
	return roots
}
