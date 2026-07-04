package biz

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"

	pb "backend-service/api/core/service/v1"
	"backend-service/pkg/auth/authn"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

type ParameterRepo interface {
	ListDefinitions(context.Context, *pb.ListParameterDefinitionsRequest) ([]*pb.ParameterDefinition, int32, error)
	GetDefinition(context.Context, uint32) (*pb.ParameterDefinition, error)
	CreateDefinition(context.Context, *pb.ParameterDefinition) (*pb.ParameterDefinition, error)
	UpdateDefinition(context.Context, *pb.ParameterDefinition) (*pb.ParameterDefinition, error)
	DeleteDefinition(context.Context, uint32) error
	ListResolved(context.Context, uint32, string) ([]*pb.ResolvedParameter, error)
	SetOverride(context.Context, uint32, string, string, uint32) (*pb.ResolvedParameter, error)
	ResetOverride(context.Context, uint32, string) error
}

type ParameterUsecase struct {
	repo ParameterRepo
	log  *log.Helper
}

func NewParameterUsecase(repo ParameterRepo, logger log.Logger) *ParameterUsecase {
	return &ParameterUsecase{repo: repo, log: log.NewHelper(logger)}
}

func (uc *ParameterUsecase) ListDefinitions(ctx context.Context, req *pb.ListParameterDefinitionsRequest) ([]*pb.ParameterDefinition, int32, error) {
	return uc.repo.ListDefinitions(ctx, req)
}

func (uc *ParameterUsecase) GetDefinition(ctx context.Context, id uint32) (*pb.ParameterDefinition, error) {
	return uc.repo.GetDefinition(ctx, id)
}

func (uc *ParameterUsecase) CreateDefinition(ctx context.Context, item *pb.ParameterDefinition) (*pb.ParameterDefinition, error) {
	if err := ValidateParameterDefinition(item); err != nil {
		return nil, err
	}
	return uc.repo.CreateDefinition(ctx, item)
}

func (uc *ParameterUsecase) UpdateDefinition(ctx context.Context, item *pb.ParameterDefinition) (*pb.ParameterDefinition, error) {
	if item == nil || item.GetId() == 0 {
		return nil, errors.BadRequest("PARAMETER_ID_REQUIRED", "参数ID不能为空")
	}
	if err := ValidateParameterDefinition(item); err != nil {
		return nil, err
	}
	return uc.repo.UpdateDefinition(ctx, item)
}

func (uc *ParameterUsecase) DeleteDefinition(ctx context.Context, id uint32) error {
	if id == 0 {
		return errors.BadRequest("PARAMETER_ID_REQUIRED", "参数ID不能为空")
	}
	return uc.repo.DeleteDefinition(ctx, id)
}

func (uc *ParameterUsecase) ListCurrent(ctx context.Context, key string) ([]*pb.ResolvedParameter, error) {
	tenantID := authn.GetAuthUserTenantID(ctx)
	if tenantID == 0 {
		return nil, errors.Forbidden("TENANT_CONTEXT_REQUIRED", "缺少有效租户上下文")
	}
	return uc.repo.ListResolved(ctx, tenantID, key)
}

func (uc *ParameterUsecase) SetCurrent(ctx context.Context, key, value string) (*pb.ResolvedParameter, error) {
	tenantID := authn.GetAuthUserTenantID(ctx)
	if tenantID == 0 {
		return nil, errors.Forbidden("TENANT_CONTEXT_REQUIRED", "缺少有效租户上下文")
	}
	return uc.repo.SetOverride(ctx, tenantID, key, value, authn.GetAuthUserID(ctx))
}

func (uc *ParameterUsecase) ResetCurrent(ctx context.Context, key string) error {
	tenantID := authn.GetAuthUserTenantID(ctx)
	if tenantID == 0 {
		return errors.Forbidden("TENANT_CONTEXT_REQUIRED", "缺少有效租户上下文")
	}
	return uc.repo.ResetOverride(ctx, tenantID, key)
}

func (uc *ParameterUsecase) ListTenant(ctx context.Context, tenantID uint32, key string) ([]*pb.ResolvedParameter, error) {
	if tenantID == 0 {
		return nil, errors.BadRequest("TENANT_ID_REQUIRED", "租户ID不能为空")
	}
	return uc.repo.ListResolved(ctx, tenantID, key)
}

func (uc *ParameterUsecase) SetTenant(ctx context.Context, tenantID uint32, key, value string) (*pb.ResolvedParameter, error) {
	if tenantID == 0 {
		return nil, errors.BadRequest("TENANT_ID_REQUIRED", "租户ID不能为空")
	}
	return uc.repo.SetOverride(ctx, tenantID, key, value, authn.GetAuthUserID(ctx))
}

func (uc *ParameterUsecase) ResetTenant(ctx context.Context, tenantID uint32, key string) error {
	if tenantID == 0 {
		return errors.BadRequest("TENANT_ID_REQUIRED", "租户ID不能为空")
	}
	return uc.repo.ResetOverride(ctx, tenantID, key)
}

var parameterKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9_-]*)+$`)

func ValidateParameterDefinition(item *pb.ParameterDefinition) error {
	if item == nil {
		return errors.BadRequest("PARAMETER_REQUIRED", "参数信息不能为空")
	}
	if !parameterKeyPattern.MatchString(item.GetKey()) {
		return errors.BadRequest("PARAMETER_KEY_INVALID", "参数键必须使用分段小写格式，例如 system.page_size")
	}
	if isSensitiveParameterKey(item.GetKey()) {
		return errors.BadRequest("SENSITIVE_PARAMETER_NOT_ALLOWED", "密钥、密码和令牌不能存入参数中心")
	}
	if strings.TrimSpace(item.GetName()) == "" || len(item.GetName()) > 100 {
		return errors.BadRequest("PARAMETER_NAME_INVALID", "参数名称不能为空且不能超过100个字符")
	}
	if item.GetValueType() == pb.ParameterValueType_PARAMETER_VALUE_TYPE_UNSPECIFIED {
		return errors.BadRequest("PARAMETER_TYPE_REQUIRED", "参数值类型不能为空")
	}
	return ValidateParameterValue(item.GetValueType(), item.GetDefaultValue())
}

func ValidateParameterValue(valueType pb.ParameterValueType, value string) error {
	if len(value) > 16384 {
		return errors.BadRequest("PARAMETER_VALUE_TOO_LARGE", "参数值不能超过16KiB")
	}
	switch valueType {
	case pb.ParameterValueType_PARAMETER_VALUE_TYPE_STRING:
		return nil
	case pb.ParameterValueType_PARAMETER_VALUE_TYPE_INTEGER:
		if _, err := strconv.ParseInt(value, 10, 64); err != nil {
			return errors.BadRequest("PARAMETER_VALUE_INVALID", "参数值必须是整数")
		}
	case pb.ParameterValueType_PARAMETER_VALUE_TYPE_BOOLEAN:
		if value != "true" && value != "false" {
			return errors.BadRequest("PARAMETER_VALUE_INVALID", "布尔参数值只能是 true 或 false")
		}
	case pb.ParameterValueType_PARAMETER_VALUE_TYPE_JSON:
		if !json.Valid([]byte(value)) {
			return errors.BadRequest("PARAMETER_VALUE_INVALID", "参数值必须是有效 JSON")
		}
	default:
		return errors.BadRequest("PARAMETER_TYPE_INVALID", "不支持的参数值类型")
	}
	return nil
}

func isSensitiveParameterKey(key string) bool {
	lower := strings.ToLower(key)
	for _, suffix := range []string{
		".password", ".passwd", ".secret", ".credential", ".private_key",
		".api_key", ".access_token", ".refresh_token",
	} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}
