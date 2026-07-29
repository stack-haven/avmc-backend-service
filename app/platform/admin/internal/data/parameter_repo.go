package data

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"

	"backend-service/api/common/enum"
	pb "backend-service/api/core/service/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/parameterdefinition"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenantparameteroverride"
	"backend-service/app/platform/admin/internal/data/ent/mixins"
	entviewer "backend-service/app/platform/admin/internal/data/ent/viewer"
)

const (
	parameterGlobalVersionKey = "platform:admin:parameter:global_version"
	parameterCacheTTL         = 30 * time.Minute
)

type parameterRepo struct{ BaseRepo }

func NewParameterRepo(data *Data, logger log.Logger) biz.ParameterRepo {
	return &parameterRepo{BaseRepo: NewBaseRepo(data, logger)}
}

func parameterDefinitionProto(row *gen.ParameterDefinition) *pb.ParameterDefinition {
	if row == nil {
		return nil
	}
	status := enum.Status(row.Status)
	sort := row.Sort
	description := row.Description
	return &pb.ParameterDefinition{
		Id:                row.ID,
		Key:               row.Key,
		Name:              row.Name,
		ValueType:         pb.ParameterValueType(row.ValueType),
		DefaultValue:      row.DefaultValue,
		Description:       &description,
		TenantOverridable: row.TenantOverridable,
		Status:            &status,
		Sort:              &sort,
		CreatedAt:         strTime(row.CreatedAt),
		UpdatedAt:         strTime(row.UpdatedAt),
	}
}

func (r *parameterRepo) ListDefinitions(ctx context.Context, req *pb.ListParameterDefinitionsRequest) ([]*pb.ParameterDefinition, int32, error) {
	query := r.Data.DB(ctx).ParameterDefinition.Query().Where(parameterdefinition.DeletedAtIsNil())
	if req.Key != nil {
		query.Where(parameterdefinition.KeyContains(*req.Key))
	}
	if req.Name != nil {
		query.Where(parameterdefinition.NameContains(*req.Name))
	}
	if req.ValueType != nil && req.GetValueType() != pb.ParameterValueType_PARAMETER_VALUE_TYPE_UNSPECIFIED {
		query.Where(parameterdefinition.ValueTypeEQ(int32(req.GetValueType())))
	}
	if req.Status != nil && req.GetStatus() != enum.Status_STATUS_UNSPECIFIED {
		query.Where(parameterdefinition.StatusEQ(int32(req.GetStatus())))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	size := int(req.GetPageSize())
	if size <= 0 || size > 100 {
		size = 20
	}
	offset, _ := strconv.Atoi(req.GetPageToken())
	rows, err := query.
		Order(gen.Asc(parameterdefinition.FieldSort), gen.Asc(parameterdefinition.FieldID)).
		Offset(offset).
		Limit(size).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	result := make([]*pb.ParameterDefinition, 0, len(rows))
	for _, row := range rows {
		result = append(result, parameterDefinitionProto(row))
	}
	return result, int32(total), nil
}

func (r *parameterRepo) GetDefinition(ctx context.Context, id uint32) (*pb.ParameterDefinition, error) {
	row, err := r.Data.DB(ctx).ParameterDefinition.Query().
		Where(parameterdefinition.IDEQ(id), parameterdefinition.DeletedAtIsNil()).
		Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("PARAMETER_NOT_FOUND", "参数定义不存在")
	}
	if err != nil {
		return nil, err
	}
	return parameterDefinitionProto(row), nil
}

func (r *parameterRepo) CreateDefinition(ctx context.Context, item *pb.ParameterDefinition) (*pb.ParameterDefinition, error) {
	row, err := r.Data.DB(ctx).ParameterDefinition.Create().
		SetKey(item.GetKey()).
		SetName(item.GetName()).
		SetValueType(int32(item.GetValueType())).
		SetDefaultValue(item.GetDefaultValue()).
		SetDescription(item.GetDescription()).
		SetTenantOverridable(item.GetTenantOverridable()).
		SetStatus(parameterStatus(item)).
		SetSort(item.GetSort()).
		Save(ctx)
	if gen.IsConstraintError(err) {
		return nil, errors.Conflict("PARAMETER_KEY_EXISTS", "参数键已存在")
	}
	if err != nil {
		return nil, err
	}
	r.bumpGlobalVersion(ctx)
	return parameterDefinitionProto(row), nil
}

func (r *parameterRepo) UpdateDefinition(ctx context.Context, item *pb.ParameterDefinition) (*pb.ParameterDefinition, error) {
	old, err := r.Data.DB(ctx).ParameterDefinition.Query().
		Where(parameterdefinition.IDEQ(item.GetId()), parameterdefinition.DeletedAtIsNil()).
		Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("PARAMETER_NOT_FOUND", "参数定义不存在")
	}
	if err != nil {
		return nil, err
	}
	if old.ValueType != int32(item.GetValueType()) {
		count, countErr := r.Data.DB(entviewer.NewSystemContext(ctx)).TenantParameterOverride.Query().
			Where(tenantparameteroverride.DefinitionIDEQ(old.ID)).
			Count(entviewer.NewSystemContext(ctx))
		if countErr != nil {
			return nil, countErr
		}
		if count > 0 {
			return nil, errors.Conflict("PARAMETER_TYPE_IN_USE", "已有租户覆盖值时不能修改参数类型")
		}
	}
	row, err := old.Update().
		SetKey(item.GetKey()).
		SetName(item.GetName()).
		SetValueType(int32(item.GetValueType())).
		SetDefaultValue(item.GetDefaultValue()).
		SetDescription(item.GetDescription()).
		SetTenantOverridable(item.GetTenantOverridable()).
		SetStatus(parameterStatus(item)).
		SetSort(item.GetSort()).
		Save(ctx)
	if gen.IsConstraintError(err) {
		return nil, errors.Conflict("PARAMETER_KEY_EXISTS", "参数键已存在")
	}
	if err != nil {
		return nil, err
	}
	r.bumpGlobalVersion(ctx)
	return parameterDefinitionProto(row), nil
}

func (r *parameterRepo) DeleteDefinition(ctx context.Context, id uint32) error {
	systemCtx := entviewer.NewSystemContext(ctx)
	count, err := r.Data.DB(systemCtx).TenantParameterOverride.Query().
		Where(tenantparameteroverride.DefinitionIDEQ(id)).
		Count(systemCtx)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.Conflict("PARAMETER_OVERRIDE_EXISTS", "存在租户覆盖值，不能删除参数定义")
	}
	deleteCtx := mixins.SkipSoftDelete(ctx)
	err = r.Data.DB(deleteCtx).ParameterDefinition.DeleteOneID(id).Exec(deleteCtx)
	if gen.IsNotFound(err) {
		return errors.NotFound("PARAMETER_NOT_FOUND", "参数定义不存在")
	}
	if err == nil {
		r.bumpGlobalVersion(ctx)
	}
	return err
}

func (r *parameterRepo) ListResolved(ctx context.Context, tenantID uint32, keyFilter string) ([]*pb.ResolvedParameter, error) {
	if strings.TrimSpace(keyFilter) == "" {
		if cached, ok := r.getResolvedCache(ctx, tenantID); ok {
			return cached, nil
		}
	}
	targetCtx := entviewer.NewTenantContext(ctx, tenantID)
	definitions, err := r.Data.DB(ctx).ParameterDefinition.Query().
		Where(
			parameterdefinition.DeletedAtIsNil(),
			parameterdefinition.StatusEQ(int32(enum.Status_STATUS_ENABLED)),
		).
		Order(gen.Asc(parameterdefinition.FieldSort), gen.Asc(parameterdefinition.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	overrides, err := r.Data.DB(targetCtx).TenantParameterOverride.Query().All(targetCtx)
	if err != nil {
		return nil, err
	}
	overrideByDefinition := make(map[uint32]*gen.TenantParameterOverride, len(overrides))
	for _, override := range overrides {
		overrideByDefinition[override.DefinitionID] = override
	}
	result := make([]*pb.ResolvedParameter, 0, len(definitions))
	for _, definition := range definitions {
		if keyFilter != "" && !strings.Contains(definition.Key, keyFilter) {
			continue
		}
		value := definition.DefaultValue
		source := pb.ParameterValueSource_PARAMETER_VALUE_SOURCE_PLATFORM_DEFAULT
		updatedAt := definition.UpdatedAt
		if override := overrideByDefinition[definition.ID]; override != nil {
			value = override.Value
			source = pb.ParameterValueSource_PARAMETER_VALUE_SOURCE_TENANT_OVERRIDE
			updatedAt = override.UpdatedAt
		}
		description := definition.Description
		result = append(result, &pb.ResolvedParameter{
			DefinitionId:      definition.ID,
			Key:               definition.Key,
			Name:              definition.Name,
			ValueType:         pb.ParameterValueType(definition.ValueType),
			Value:             value,
			Source:            source,
			TenantOverridable: definition.TenantOverridable,
			Description:       &description,
			UpdatedAt:         strTime(updatedAt),
		})
	}
	if keyFilter == "" {
		r.setResolvedCache(ctx, tenantID, result)
	}
	return result, nil
}

func (r *parameterRepo) SetOverride(ctx context.Context, tenantID uint32, key, value string, operatorID uint32) (*pb.ResolvedParameter, error) {
	definition, err := r.definitionByKey(ctx, key)
	if err != nil {
		return nil, err
	}
	if !definition.TenantOverridable {
		return nil, errors.Forbidden("PARAMETER_NOT_OVERRIDABLE", "该参数不允许租户覆盖")
	}
	if err := biz.ValidateParameterValue(pb.ParameterValueType(definition.ValueType), value); err != nil {
		return nil, err
	}
	targetCtx := entviewer.NewTenantContext(ctx, tenantID)
	existing, err := r.Data.DB(targetCtx).TenantParameterOverride.Query().
		Where(tenantparameteroverride.DefinitionIDEQ(definition.ID)).
		Only(targetCtx)
	switch {
	case err == nil:
		builder := existing.Update().SetValue(value)
		if operatorID > 0 {
			builder.SetUpdatedBy(operatorID)
		}
		if _, err = builder.Save(targetCtx); err != nil {
			return nil, err
		}
	case gen.IsNotFound(err):
		builder := r.Data.DB(targetCtx).TenantParameterOverride.Create().
			SetDefinitionID(definition.ID).
			SetValue(value)
		if operatorID > 0 {
			builder.SetUpdatedBy(operatorID)
		}
		if _, err = builder.Save(targetCtx); err != nil {
			return nil, err
		}
	default:
		return nil, err
	}
	r.bumpTenantVersion(ctx, tenantID)
	items, err := r.ListResolved(ctx, tenantID, key)
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if item.GetKey() == key {
			return item, nil
		}
	}
	return nil, errors.NotFound("PARAMETER_NOT_FOUND", "参数定义不存在")
}

func (r *parameterRepo) ResetOverride(ctx context.Context, tenantID uint32, key string) error {
	definition, err := r.definitionByKey(ctx, key)
	if err != nil {
		return err
	}
	targetCtx := entviewer.NewTenantContext(ctx, tenantID)
	_, err = r.Data.DB(targetCtx).TenantParameterOverride.Delete().
		Where(tenantparameteroverride.DefinitionIDEQ(definition.ID)).
		Exec(targetCtx)
	if err != nil {
		return err
	}
	r.bumpTenantVersion(ctx, tenantID)
	return nil
}

func (r *parameterRepo) definitionByKey(ctx context.Context, key string) (*gen.ParameterDefinition, error) {
	row, err := r.Data.DB(ctx).ParameterDefinition.Query().
		Where(
			parameterdefinition.KeyEQ(key),
			parameterdefinition.StatusEQ(int32(enum.Status_STATUS_ENABLED)),
			parameterdefinition.DeletedAtIsNil(),
		).
		Only(ctx)
	if gen.IsNotFound(err) {
		return nil, errors.NotFound("PARAMETER_NOT_FOUND", "参数定义不存在或已停用")
	}
	return row, err
}


func parameterStatus(item *pb.ParameterDefinition) int32 {
	if item.Status == nil || item.GetStatus() == enum.Status_STATUS_UNSPECIFIED {
		return int32(enum.Status_STATUS_ENABLED)
	}
	return int32(item.GetStatus())
}

func (r *parameterRepo) bumpGlobalVersion(ctx context.Context) {
	if r.Data.rdb != nil {
		if err := r.Data.rdb.Incr(ctx, parameterGlobalVersionKey).Err(); err != nil {
			r.Log.Warnf("bump parameter global cache version: %v", err)
		}
	}
}

func (r *parameterRepo) bumpTenantVersion(ctx context.Context, tenantID uint32) {
	if r.Data.rdb != nil {
		if err := r.Data.rdb.Incr(ctx, parameterTenantVersionKey(tenantID)).Err(); err != nil {
			r.Log.Warnf("bump tenant parameter cache version: %v", err)
		}
	}
}

func (r *parameterRepo) getResolvedCache(ctx context.Context, tenantID uint32) ([]*pb.ResolvedParameter, bool) {
	if r.Data.rdb == nil {
		return nil, false
	}
	key, err := r.resolvedCacheKey(ctx, tenantID)
	if err != nil {
		return nil, false
	}
	raw, err := r.Data.rdb.Get(ctx, key).Bytes()
	if err != nil {
		if err != redis.Nil {
			r.Log.Warnf("get resolved parameter cache: %v", err)
		}
		return nil, false
	}
	var result []*pb.ResolvedParameter
	if err := json.Unmarshal(raw, &result); err != nil {
		r.Log.Warnf("decode resolved parameter cache: %v", err)
		return nil, false
	}
	return result, true
}

func (r *parameterRepo) setResolvedCache(ctx context.Context, tenantID uint32, values []*pb.ResolvedParameter) {
	if r.Data.rdb == nil {
		return
	}
	key, err := r.resolvedCacheKey(ctx, tenantID)
	if err != nil {
		return
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return
	}
	if err := r.Data.rdb.Set(ctx, key, raw, parameterCacheTTL).Err(); err != nil {
		r.Log.Warnf("set resolved parameter cache: %v", err)
	}
}

func (r *parameterRepo) resolvedCacheKey(ctx context.Context, tenantID uint32) (string, error) {
	globalVersion, err := r.cacheVersion(ctx, parameterGlobalVersionKey)
	if err != nil {
		return "", err
	}
	tenantVersion, err := r.cacheVersion(ctx, parameterTenantVersionKey(tenantID))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("platform:admin:parameter:resolved:%d:%s:%s", tenantID, globalVersion, tenantVersion), nil
}

func (r *parameterRepo) cacheVersion(ctx context.Context, key string) (string, error) {
	value, err := r.Data.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		return "0", nil
	}
	return value, err
}

func parameterTenantVersionKey(tenantID uint32) string {
	return fmt.Sprintf("platform:admin:parameter:tenant:%d:version", tenantID)
}
