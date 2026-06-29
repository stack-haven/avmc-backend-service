package data

import (
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/menu"
	"backend-service/app/platform/admin/internal/data/ent/gen/role"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenant"
	"backend-service/app/platform/admin/internal/data/ent/gen/user"
	entviewer "backend-service/app/platform/admin/internal/data/ent/viewer"
	"backend-service/pkg/auth"
	"backend-service/pkg/auth/loginattempt"
	"backend-service/pkg/utils/convert"
	"backend-service/pkg/utils/crypto"
	"context"
	"errors"
	"net/http"
	"sort"
	"time"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport"
	"go.opentelemetry.io/otel/trace"

	"backend-service/api/common/enum"
	pb "backend-service/api/platform/admin/v1"

	pbCore "backend-service/api/core/service/v1"
	iputil "backend-service/pkg/utils/ip"
)

// AuthRepo 数据仓库结构体
// 包含日志记录器
type authRepo struct {
	data  *Data
	log   *log.Helper
	atr   *auth.AuthToken
	ur    *userRepo
	mr    *menuRepo
	mpr   *menuPermissionGroupRepo
	guard loginattempt.Guard
	llr   biz.LoginLogRepo
}

// NewAuthRepo 创建新的用户数据仓库实例
// 参数：logger 日志记录器
// 返回值：用户数据仓库实例指针
func NewAuthRepo(data *Data, atr *auth.AuthToken, guard loginattempt.Guard, loginLogs biz.LoginLogRepo, logger log.Logger) biz.AuthRepo {
	return &authRepo{
		data:  data,
		log:   log.NewHelper(logger),
		atr:   atr,
		ur:    NewUserRepo(data, logger).(*userRepo),
		mr:    NewMenuRepo(data, logger).(*menuRepo),
		mpr:   NewMenuPermissionGroupRepo(data, logger).(*menuPermissionGroupRepo),
		guard: guard,
		llr:   loginLogs,
	}
}

// LoginResponse 处理登陆统一返回
// 参数：ctx 上下文，res 用户实体，accessToken 访问令牌，refreshToken 刷新令牌，expires 过期时间
// 返回值：登录响应结构体
func (r *authRepo) LoginResponse(ctx context.Context, u *gen.User) (*pb.LoginResponse, error) {
	accessToken, refreshToken, err := r.atr.GenerateToken(ctx, auth.AuthTokenInfo{
		UserId:   u.ID,
		Username: convert.ToValue(u.Name),
		TenantID: u.TenantID,
	})
	if err != nil {
		r.log.Errorf("登录数据操作失败，Token生成错误错误：%v", err)
		return nil, err
	}
	claims, err := r.atr.ValidateToken(ctx, accessToken)
	if err != nil {
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
		SessionId:    claims.GetID(),
	}, nil
}

// LoginByUsername 处理后台用户名登录数据操作
// 参数：ctx 上下文，name 用户名，password 密码
// 返回值：登录响应结构体，错误信息
func (r *authRepo) LoginByUsername(ctx context.Context, name, password string, tenantID uint32) (resp *pb.LoginResponse, err error) {
	defer func() { r.recordLogin(ctx, "username", name, tenantID, resp, err) }()
	if err := r.checkLogin(ctx, "username", name, tenantID); err != nil {
		return nil, err
	}
	if err := r.requireActiveTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	ctx = entviewer.NewTenantContext(ctx, tenantID)
	res, err := r.data.DB(ctx).User.Query().
		Select(user.FieldPassword, user.FieldName, user.FieldTenantID).
		Where(
			user.NameEQ(name),
			user.StatusEQ(int32(enum.Status_STATUS_ENABLED)),
		).
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, r.loginFailed(ctx, "username", name, tenantID)
		}
		r.log.Errorf("用户名登录查询失败，tenant_id=%d，错误：%v", tenantID, err)
		return nil, err
	}
	if res.Password == nil {
		return nil, r.loginFailed(ctx, "username", name, tenantID)
	}
	if !crypto.CheckPasswordHash(password, *res.Password) {
		return nil, r.loginFailed(ctx, "username", name, tenantID)
	}
	r.loginSucceeded(ctx, "username", name, tenantID)
	return r.LoginResponse(ctx, res)
}

// LoginByEmail 处理后台邮箱登录数据操作
// 参数：ctx email 邮箱，password 密码
// 返回值：登录响应结构体，错误信息
func (r *authRepo) LoginByEmail(ctx context.Context, email, password string, tenantID uint32) (resp *pb.LoginResponse, err error) {
	defer func() { r.recordLogin(ctx, "email", email, tenantID, resp, err) }()
	if err := r.checkLogin(ctx, "email", email, tenantID); err != nil {
		return nil, err
	}
	if err := r.requireActiveTenant(ctx, tenantID); err != nil {
		return nil, err
	}
	ctx = entviewer.NewTenantContext(ctx, tenantID)
	res, err := r.data.DB(ctx).User.Query().
		Select(user.FieldPassword, user.FieldName, user.FieldTenantID).
		Where(
			user.EmailEQ(email),
			user.StatusEQ(int32(enum.Status_STATUS_ENABLED)),
		).
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, r.loginFailed(ctx, "email", email, tenantID)
		}
		r.log.Errorf("邮箱登录查询失败，tenant_id=%d，错误：%v", tenantID, err)
		return nil, err
	}
	if res.Password == nil {
		return nil, r.loginFailed(ctx, "email", email, tenantID)
	}
	if !crypto.CheckPasswordHash(password, *res.Password) {
		return nil, r.loginFailed(ctx, "email", email, tenantID)
	}
	r.loginSucceeded(ctx, "email", email, tenantID)
	return r.LoginResponse(ctx, res)
}

func (r *authRepo) requireActiveTenant(ctx context.Context, tenantID uint32) error {
	if tenantID == 0 {
		return pb.ErrorUserIncorrectPassword("用户名或密码错误")
	}
	systemCtx := entviewer.NewSystemContext(ctx)
	exists, err := r.data.DB(systemCtx).Tenant.Query().
		Where(
			tenant.IDEQ(tenantID),
			tenant.StatusEQ(int32(enum.Status_STATUS_ENABLED)),
			tenant.LifecycleStatusEQ(int32(pbCore.TenantLifecycleStatus_TENANT_LIFECYCLE_STATUS_ACTIVE)),
			tenant.Or(tenant.ExpiresAtIsNil(), tenant.ExpiresAtGT(time.Now())),
			tenant.DeletedAtIsNil(),
		).
		Exist(systemCtx)
	if err != nil {
		return err
	}
	if !exists {
		return pb.ErrorUserIncorrectPassword("用户名或密码错误")
	}
	return nil
}

func (r *authRepo) recordLogin(ctx context.Context, loginType, identity string, tenantID uint32, resp *pb.LoginResponse, loginErr error) {
	if r.llr == nil || tenantID == 0 {
		return
	}
	result := "success"
	failureReason := ""
	var userID *uint32
	sessionID := ""
	if resp != nil && resp.GetId() > 0 {
		id := resp.GetId()
		userID = &id
		sessionID = resp.GetSessionId()
	}
	if loginErr != nil {
		result = "failure"
		if pb.IsUserTooManyLoginAttempts(loginErr) {
			result = "locked"
		}
		if serviceErr := kerrors.FromError(loginErr); serviceErr != nil {
			failureReason = serviceErr.Message
		}
	}
	ip := iputil.FormContext(ctx)
	userAgent := ""
	if info, ok := transport.FromServerContext(ctx); ok {
		userAgent = info.RequestHeader().Get("User-Agent")
		if carrier, ok := info.(interface{ Request() *http.Request }); ok {
			if request := carrier.Request(); request != nil {
				if requestIP, requestErr := iputil.GetIP(request); requestErr == nil {
					ip = requestIP
				}
			}
		}
	}
	traceID := trace.SpanContextFromContext(ctx).TraceID().String()
	logCtx := entviewer.NewTenantContext(ctx, tenantID)
	if err := r.llr.Append(logCtx, &pbCore.LoginLog{
		TenantId:      tenantID,
		UserId:        userID,
		Identity:      identity,
		LoginType:     loginType,
		Result:        result,
		FailureReason: &failureReason,
		Ip:            &ip,
		UserAgent:     &userAgent,
		TraceId:       &traceID,
		SessionId:     &sessionID,
	}); err != nil {
		r.log.Errorf("记录登录安全日志失败，tenant_id=%d: %v", tenantID, err)
	}
}

func (r *authRepo) checkLogin(ctx context.Context, scope, identity string, tenantID uint32) error {
	if r.guard == nil {
		return nil
	}
	err := r.guard.Check(ctx, scope, identity, tenantID)
	if errors.Is(err, loginattempt.ErrLocked) {
		return pb.ErrorUserTooManyLoginAttempts("登录失败次数过多，请稍后重试")
	}
	if err != nil {
		r.log.Warnf("登录保护检查失败，tenant_id=%d，错误：%v", tenantID, err)
	}
	return nil
}

func (r *authRepo) loginFailed(ctx context.Context, scope, identity string, tenantID uint32) error {
	if r.guard != nil {
		err := r.guard.Failure(ctx, scope, identity, tenantID)
		if errors.Is(err, loginattempt.ErrLocked) {
			return pb.ErrorUserTooManyLoginAttempts("登录失败次数过多，请稍后重试")
		}
		if err != nil {
			r.log.Warnf("记录登录失败次数失败，tenant_id=%d，错误：%v", tenantID, err)
		}
	}
	return pb.ErrorUserIncorrectPassword("用户名或密码错误")
}

func (r *authRepo) loginSucceeded(ctx context.Context, scope, identity string, tenantID uint32) {
	if r.guard == nil {
		return
	}
	if err := r.guard.Success(ctx, scope, identity, tenantID); err != nil {
		r.log.Warnf("重置登录失败次数失败，tenant_id=%d，错误：%v", tenantID, err)
	}
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
	tenantID := convert.StringToUnit32(claims.GetTenant())
	if userID == 0 {
		return nil, pb.ErrorAuthInvalidToken("刷新令牌主体无效")
	}
	if tenantID == 0 {
		return nil, pb.ErrorAuthInvalidToken("刷新令牌域无效")
	}
	ctx = entviewer.NewTenantContext(ctx, tenantID)
	if err := r.requireActiveTenant(ctx, tenantID); err != nil {
		return nil, pb.ErrorAuthInvalidToken("刷新令牌租户无效")
	}
	res, err := r.data.DB(ctx).User.Query().
		Select(user.FieldName, user.FieldTenantID).
		Where(
			user.IDEQ(userID),
			user.StatusEQ(int32(enum.Status_STATUS_ENABLED)),
		).
		Only(ctx)
	if err != nil {
		r.log.Errorf("刷新令牌用户查询失败，用户ID：%d，错误：%v", userID, err)
		if gen.IsNotFound(err) {
			return nil, pb.ErrorAuthInvalidToken("刷新令牌用户无效")
		}
		return nil, err
	}
	sessionID := claims.GetID()
	accessToken, newRefreshToken, err := r.atr.RotateSessionToken(ctx, auth.AuthTokenInfo{
		UserId:   userID,
		Username: convert.ToValue(res.Name),
		TenantID: res.TenantID,
	}, sessionID)
	if err != nil {
		r.log.Errorf("刷新令牌重新签发失败，用户ID：%d，错误：%v", userID, err)
		return nil, err
	}
	return &pb.RefreshTokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		SessionId:    sessionID,
	}, nil
}

// Logout 处理后台登出数据操作
// 参数：ctx 上下文
// 返回值：错误信息
func (r *authRepo) Logout(ctx context.Context, sessionID string) error {
	tenantID, err := requireTenantID(ctx)
	if err != nil {
		return err
	}
	r.log.Infof("尝试登出会话：%s", sessionID)
	return r.atr.RevokeSession(ctx, tenantID, sessionID)
}

// Register 处理注册数据操作
// 参数：ctx 上下文，name 用户名，password 密码
// 返回值：错误信息
func (r *authRepo) Register(ctx context.Context, name, password string) error {
	if err := biz.ValidatePassword(password); err != nil {
		return err
	}
	if _, err := requireTenantID(ctx); err != nil {
		return err
	}
	r.log.Infof("尝试注册数据操作")
	hashPassword, err := crypto.HashPassword(password)
	if err != nil {
		return err
	}
	_, err = r.data.DB(ctx).User.Create().SetName(name).SetPassword(hashPassword).Save(ctx)
	if err != nil {
		r.log.Errorf("注册数据操作失败：%v", err)
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
	menus, err = withMenuAncestors(ctx, r.data.DB(ctx), menus)
	if err != nil {
		return nil, err
	}
	return buildMenuTree(convert.SliceToAny(menus, r.mr.convertProto)), nil
}

func (r *authRepo) userMenus(ctx context.Context, userId uint32) ([]*gen.Menu, error) {
	tenantID, err := requireTenantID(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := r.data.DB(ctx).User.Query().
		Select(user.FieldTenantID).
		Where(user.IDEQ(userId)).
		Only(ctx); err != nil {
		r.log.Errorf("查询用户域失败，用户ID：%d，错误：%v", userId, err)
		return nil, err
	}
	menus, err := r.data.DB(ctx).User.Query().
		Where(user.IDEQ(userId)).
		QueryRoles().
		Where(
			role.StatusEQ(int32(enum.Status_STATUS_ENABLED)),
		).
		QueryMenus().
		Where(menu.StatusEQ(int32(enum.Status_STATUS_ENABLED))).
		Order(gen.Asc(menu.FieldSort, menu.FieldID)).
		All(ctx)
	if err != nil {
		r.log.Errorf("查询用户菜单失败，用户ID：%d，错误：%v", userId, err)
		return nil, err
	}
	effectiveIDs, err := r.menuPermissionGroups().GetTenantEffectiveMenuIDs(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if len(effectiveIDs) == 0 {
		return nil, nil
	}
	allowed := make(map[uint32]struct{}, len(effectiveIDs))
	for _, id := range effectiveIDs {
		allowed[id] = struct{}{}
	}

	menuMap := make(map[uint32]*gen.Menu, len(menus))
	for _, m := range menus {
		if _, ok := allowed[m.ID]; !ok {
			continue
		}
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

func (r *authRepo) menuPermissionGroups() *menuPermissionGroupRepo {
	if r.mpr != nil {
		return r.mpr
	}
	if r.mr == nil {
		r.mr = &menuRepo{BaseRepo: BaseRepo{Data: r.data, Log: r.log}}
	}
	r.mpr = &menuPermissionGroupRepo{
		BaseRepo: BaseRepo{Data: r.data, Log: r.log},
		mr:       r.mr,
	}
	return r.mpr
}
