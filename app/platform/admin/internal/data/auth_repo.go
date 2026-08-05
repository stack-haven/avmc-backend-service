package data

import (
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/menu"
	"backend-service/app/platform/admin/internal/data/ent/gen/user"
	entviewer "backend-service/app/platform/admin/internal/data/ent/viewer"
	"backend-service/pkg/auth"
	"backend-service/pkg/auth/loginattempt"
	"backend-service/pkg/utils/convert"
	"backend-service/pkg/utils/crypto"
	"context"
	"errors"
	"net/http"
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
	mpr   *tenantMenuPermissionGroupRepo
	rr    *roleRepo
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
		mpr:   NewTenantMenuPermissionGroupRepo(data, logger).(*tenantMenuPermissionGroupRepo),
		rr:    NewRoleRepo(data, logger).(*roleRepo),
		guard: guard,
		llr:   loginLogs,
	}
}

// LoginResponse 处理登陆统一返回
// 参数：ctx 上下文，res 用户实体，accessToken 访问令牌，refreshToken 刷新令牌，expires 过期时间
// 返回值：登录响应结构体
func (r *authRepo) LoginResponse(ctx context.Context, u *gen.User) (*pb.LoginResponse, error) {
	activeTenant, err := r.findActiveTenant(ctx, u.TenantID)
	if err != nil {
		return nil, err
	}
	accessToken, refreshToken, err := r.atr.GenerateToken(ctx, auth.AuthTokenInfo{
		UserId:           u.ID,
		Username:         convert.ToValue(u.Name),
		TenantID:         u.TenantID,
		PlatformOperator: activeTenant.IsPlatform,
		TenantExpiresAt:  activeTenant.ExpiresAt,
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
	}(minTokenExpiration(r.atr.Authenticator.Options().TokenExpiration, activeTenant.ExpiresAt)), time.RFC3339)
	return &pb.LoginResponse{
		Id:           u.ID,
		Name:         u.Name,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expires,
		SessionId:    claims.GetID(),
	}, nil
}

func minTokenExpiration(defaultExpiration time.Duration, expiresAt *time.Time) time.Duration {
	if expiresAt == nil {
		return defaultExpiration
	}
	remaining := time.Until(*expiresAt)
	if remaining > 0 && remaining < defaultExpiration {
		return remaining
	}
	return defaultExpiration
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
	_, err := r.findActiveTenant(ctx, tenantID)
	return err
}

type tenantInfo struct {
	IsPlatform bool
	ExpiresAt  *time.Time
}

func (r *authRepo) findActiveTenant(ctx context.Context, tenantID uint32) (*tenantInfo, error) {
	if tenantID == 0 {
		return nil, pb.ErrorUserIncorrectPassword("用户名或密码错误")
	}
	return &tenantInfo{IsPlatform: false, ExpiresAt: nil}, nil
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
	activeTenant, err := r.findActiveTenant(ctx, tenantID)
	if err != nil {
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
		UserId:           userID,
		Username:         convert.ToValue(res.Name),
		TenantID:         res.TenantID,
		PlatformOperator: activeTenant.IsPlatform,
		TenantExpiresAt:  activeTenant.ExpiresAt,
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
	if err := r.atr.RevokeSession(ctx, tenantID, sessionID); err != nil {
		r.log.Errorf("登出撤销会话失败: %v", err)
	}
	return nil
}

// Register 注册用户（已废弃，统一通过User管理）
func (r *authRepo) Register(_ context.Context, _ string, _ string) error {
	return pb.ErrorBadRequest("自助注册已关闭，请联系管理员")
}

// Profile 获取用户简介信息
func (r *authRepo) Profile(ctx context.Context, userID uint32) (*pb.ProfileResponse, error) {
	u, err := r.ur.FindByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	// Collect role info from user's role IDs
	var roleNames []string
	var primaryRole *pbCore.Role
	if u != nil && len(u.GetRoleIds()) > 0 {
		for _, roleID := range u.GetRoleIds() {
			role, roleErr := r.rr.FindByID(ctx, roleID)
			if roleErr != nil {
				continue
			}
			roleNames = append(roleNames, role.GetName())
			if primaryRole == nil {
				primaryRole = role
			}
		}
	}
	return &pb.ProfileResponse{
		User:  u,
		Role:  primaryRole,
		Roles: roleNames,
	}, nil
}

// Codes 获取用户权限码
func (r *authRepo) Codes(ctx context.Context, userId uint32) ([]string, error) {
	u, err := r.data.DB(ctx).User.Query().
		Where(user.IDEQ(userId), user.StatusEQ(int32(enum.Status_STATUS_ENABLED))).
		WithRoles(func(q *gen.RoleQuery) {
			q.WithMenus(func(q *gen.MenuQuery) {
				q.Select(menu.FieldAuthCode)
			})
		}).
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	seen := make(map[string]bool)
	var codes []string
	for _, rl := range u.Edges.Roles {
		for _, m := range rl.Edges.Menus {
			code := convert.ToValue(m.AuthCode)
			if code != "" && !seen[code] {
				seen[code] = true
				codes = append(codes, code)
			}
		}
	}
	return codes, nil
}

// Menus 获取用户菜单
func (r *authRepo) Menus(ctx context.Context, userId uint32) ([]*pbCore.Menu, error) {
	menus, err := r.data.DB(ctx).Menu.Query().
		Where(menu.StatusEQ(int32(enum.Status_STATUS_ENABLED))).
		Order(gen.Asc(menu.FieldSort), gen.Desc(menu.FieldID)).
		All(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	protoMenus := convert.SliceToAny(menus, r.mr.convertProto)
	return buildMenuTree(protoMenus), nil
}

func (r *authRepo) TenantMenuPermissionGroups() *tenantMenuPermissionGroupRepo {
	if r.mpr != nil {
		return r.mpr
	}
	if r.mr == nil {
		r.mr = &menuRepo{BaseRepo: BaseRepo{Data: r.data, Log: r.log}}
	}
	r.mpr = &tenantMenuPermissionGroupRepo{
		BaseRepo: BaseRepo{Data: r.data, Log: r.log},
		mr:       r.mr,
	}
	return r.mpr
}
