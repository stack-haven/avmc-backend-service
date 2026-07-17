package data

import (
	pbEnum "backend-service/api/common/enum"
	pbCore "backend-service/api/core/service/v1"
	pb "backend-service/api/platform/admin/v1"
	"backend-service/app/platform/admin/internal/biz"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/menu"
	"backend-service/app/platform/admin/internal/data/ent/gen/menupermissiongroup"
	"backend-service/app/platform/admin/internal/data/ent/gen/menupermissiongroupversion"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenant"
	"backend-service/app/platform/admin/internal/data/ent/gen/tenantpermissiongroup"
	"backend-service/pkg/aip/listing"
	"backend-service/pkg/utils/convert"
	"context"
	"reflect"
	"sort"
	"strconv"
	"time"

	"github.com/go-kratos/aip-go/ents"
	kratosErrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

var _ biz.MenuPermissionGroupRepo = (*menuPermissionGroupRepo)(nil)

type menuPermissionGroupRepo struct {
	BaseRepo
	mr *menuRepo
}

func NewMenuPermissionGroupRepo(data *Data, logger log.Logger) biz.MenuPermissionGroupRepo {
	return &menuPermissionGroupRepo{
		BaseRepo: NewBaseRepo(data, logger),
		mr:       NewMenuRepo(data, logger).(*menuRepo),
	}
}

func (r *menuPermissionGroupRepo) validateMenuIDs(ctx context.Context, menuIDs []uint32) error {
	ids := uniquePositiveIDs(menuIDs)
	if len(ids) == 0 {
		return nil
	}
	count, err := r.Data.DB(ctx).Menu.Query().Where(menu.IDIn(ids...)).Count(ctx)
	if err != nil {
		return err
	}
	if count != len(ids) {
		return pb.ErrorBadRequest("存在无效菜单ID")
	}
	return nil
}

func (r *menuPermissionGroupRepo) entToProto(e *gen.MenuPermissionGroup) *pbCore.MenuPermissionGroup {
	if e == nil {
		return nil
	}
	status := pbEnum.Status(0)
	if e.Status != nil {
		status = pbEnum.Status(*e.Status)
	}
	result := &pbCore.MenuPermissionGroup{
		Id:             e.ID,
		Name:           convert.ToPointer(e.Name),
		Code:           convert.ToPointer(e.Code),
		Status:         &status,
		IsSystem:       e.IsSystem,
		Sort:           e.Sort,
		Description:    e.Description,
		Remark:         e.Remark,
		MenuIds:        menuPermissionGroupMenuIDs(e),
		ApiPermissions: normalizeStringList(e.APIPermissions),
		FeatureFlags:   copyBoolMap(e.FeatureFlags),
		ResourceQuotas: copyInt64Map(e.ResourceQuotas),
		TenantCount:    convert.EmptyToNil(int32(len(e.Edges.TenantBindings))),
		CreatedAt:      convert.TimeValueToString(&e.CreatedAt, time.DateTime),
		UpdatedAt:      convert.TimeValueToString(&e.UpdatedAt, time.DateTime),
	}
	if current, err := e.Edges.CurrentVersionOrErr(); err == nil {
		result.CurrentVersionId = &current.ID
		result.CurrentVersion = &current.Version
	}
	return result
}

func menuPermissionGroupVersionToProto(e *gen.MenuPermissionGroupVersion) *pbCore.MenuPermissionGroupVersion {
	if e == nil {
		return nil
	}
	return &pbCore.MenuPermissionGroupVersion{
		Id:             e.ID,
		GroupId:        e.GroupID,
		Version:        e.Version,
		State:          pbCore.MenuPermissionGroupVersionState(e.State),
		MenuIds:        menuPermissionGroupVersionMenuIDs(e),
		ApiPermissions: normalizeStringList(e.APIPermissions),
		FeatureFlags:   copyBoolMap(e.FeatureFlags),
		ResourceQuotas: copyInt64Map(e.ResourceQuotas),
		ChangeSummary:  e.ChangeSummary,
		CreatedBy:      e.CreatedBy,
		PublishedBy:    e.PublishedBy,
		EffectiveAt:    convert.TimeValueToString(e.EffectiveAt, time.DateTime),
		PublishedAt:    convert.TimeValueToString(e.PublishedAt, time.DateTime),
		CreatedAt:      convert.TimeValueToString(&e.CreatedAt, time.DateTime),
	}
}

func menuPermissionGroupVersionMenuIDs(e *gen.MenuPermissionGroupVersion) []uint32 {
	if e == nil {
		return nil
	}
	ids := make([]uint32, 0, len(e.Edges.Menus))
	for _, item := range e.Edges.Menus {
		if item != nil {
			ids = append(ids, item.ID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func (r *menuPermissionGroupRepo) Save(ctx context.Context, g *pbCore.MenuPermissionGroup) (*pbCore.MenuPermissionGroup, error) {
	if g == nil || g.GetName() == "" || g.GetCode() == "" {
		return nil, pb.ErrorBadRequest("权限组名称和编码不能为空")
	}
	if err := r.validateMenuIDs(ctx, g.GetMenuIds()); err != nil {
		return nil, err
	}
	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, r.Log)
	builder := tx.MenuPermissionGroup.Create().
		SetName(g.GetName()).
		SetCode(g.GetCode()).
		SetNillableStatus((*int32)(g.Status)).
		SetNillableIsSystem(g.IsSystem).
		SetNillableSort(g.Sort).
		SetNillableDescription(g.Description).
		SetNillableRemark(g.Remark).
		SetAPIPermissions(normalizeStringList(g.GetApiPermissions())).
		SetFeatureFlags(copyBoolMap(g.GetFeatureFlags())).
		SetResourceQuotas(copyInt64Map(g.GetResourceQuotas()))
	if len(g.GetMenuIds()) > 0 {
		builder.AddMenuIDs(uniquePositiveIDs(g.GetMenuIds())...)
	}
	res, err := builder.Save(ctx)
	if err != nil {
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorBadRequest("权限组名称或编码已存在")
		}
		return nil, err
	}
	version, err := r.createVersionTx(ctx, tx, res.ID, 1, groupVersionSnapshotFromGroup(g), "初始版本", 0)
	if err != nil {
		return nil, err
	}
	if _, err = tx.MenuPermissionGroup.UpdateOneID(res.ID).SetCurrentVersionID(version.ID).Save(ctx); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, res.ID)
}

func (r *menuPermissionGroupRepo) Update(ctx context.Context, g *pbCore.MenuPermissionGroup) (*pbCore.MenuPermissionGroup, error) {
	if g == nil || g.GetId() == 0 || g.GetName() == "" || g.GetCode() == "" {
		return nil, pb.ErrorBadRequest("权限组ID、名称和编码不能为空")
	}
	if g.MenuIds != nil {
		if err := r.validateMenuIDs(ctx, g.GetMenuIds()); err != nil {
			return nil, err
		}
	}
	current, err := r.FindByID(ctx, g.GetId())
	if err != nil {
		return nil, err
	}
	targetMenuIDs := current.GetMenuIds()
	if g.MenuIds != nil {
		targetMenuIDs = g.GetMenuIds()
	}
	targetAPIPermissions := current.GetApiPermissions()
	if g.ApiPermissions != nil {
		targetAPIPermissions = g.GetApiPermissions()
	}
	targetFeatureFlags := current.GetFeatureFlags()
	if g.FeatureFlags != nil {
		targetFeatureFlags = g.GetFeatureFlags()
	}
	targetResourceQuotas := current.GetResourceQuotas()
	if g.ResourceQuotas != nil {
		targetResourceQuotas = g.GetResourceQuotas()
	}
	menuChanged := current.GetCurrentVersionId() == 0 ||
		(g.MenuIds != nil && !sameUint32Set(current.GetMenuIds(), targetMenuIDs))
	capabilityChanged := g.ApiPermissions != nil && !sameStringSet(current.GetApiPermissions(), targetAPIPermissions) ||
		g.FeatureFlags != nil && !reflect.DeepEqual(copyBoolMap(current.GetFeatureFlags()), copyBoolMap(targetFeatureFlags)) ||
		g.ResourceQuotas != nil && !reflect.DeepEqual(copyInt64Map(current.GetResourceQuotas()), copyInt64Map(targetResourceQuotas))
	versionChanged := menuChanged || capabilityChanged
	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, r.Log)
	builder := tx.MenuPermissionGroup.UpdateOneID(g.GetId()).
		SetName(g.GetName()).
		SetCode(g.GetCode()).
		SetNillableIsSystem(g.IsSystem).
		SetNillableSort(g.Sort).
		SetNillableDescription(g.Description).
		SetNillableRemark(g.Remark)
	if g.ApiPermissions != nil {
		builder.SetAPIPermissions(normalizeStringList(targetAPIPermissions))
	}
	if g.FeatureFlags != nil {
		builder.SetFeatureFlags(copyBoolMap(targetFeatureFlags))
	}
	if g.ResourceQuotas != nil {
		builder.SetResourceQuotas(copyInt64Map(targetResourceQuotas))
	}
	if menuChanged {
		builder.ClearMenus()
		if len(targetMenuIDs) > 0 {
			builder.AddMenuIDs(uniquePositiveIDs(targetMenuIDs)...)
		}
	}
	res, err := builder.Save(ctx)
	if err != nil {
		if gen.IsConstraintError(err) {
			return nil, pb.ErrorBadRequest("权限组名称或编码已存在")
		}
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("权限组不存在")
		}
		return nil, err
	}
	var affectedTenantIDs []uint32
	if versionChanged {
		version, publishErr := r.publishVersionTx(ctx, tx, g.GetId(), &pbCore.MenuPermissionGroupVersion{
			MenuIds:        targetMenuIDs,
			ApiPermissions: targetAPIPermissions,
			FeatureFlags:   targetFeatureFlags,
			ResourceQuotas: targetResourceQuotas,
		}, "通过套餐编辑发布", 0)
		if publishErr != nil {
			return nil, publishErr
		}
		affectedTenantIDs, err = r.advanceAutoUpgradeBindingsTx(ctx, tx, g.GetId(), version.ID)
		if err != nil {
			return nil, err
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	r.bumpTenantsPackageVersion(ctx, affectedTenantIDs)
	return r.FindByID(ctx, res.ID)
}

func (r *menuPermissionGroupRepo) FindByID(ctx context.Context, id uint32) (*pbCore.MenuPermissionGroup, error) {
	res, err := r.Data.DB(ctx).MenuPermissionGroup.Query().
		Where(menupermissiongroup.IDEQ(id)).
		WithMenus(func(q *gen.MenuQuery) { q.Select(menu.FieldID) }).
		WithCurrentVersion().
		WithTenantBindings().
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("权限组不存在")
		}
		return nil, err
	}
	return r.entToProto(res), nil
}

func (r *menuPermissionGroupRepo) CountMenuPermissionGroups(ctx context.Context, opts ...listing.Option) (int32, error) {
	o := listing.Options{}
	for _, opt := range opts {
		opt(&o)
	}
	count, err := r.Data.DB(ctx).MenuPermissionGroup.Query().
		Where(ents.ApplyFilter(o.Filter)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

func (r *menuPermissionGroupRepo) ListMenuPermissionGroups(ctx context.Context, opts ...listing.Option) ([]*pbCore.MenuPermissionGroup, error) {
	o := listing.Options{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	res, err := r.Data.DB(ctx).MenuPermissionGroup.Query().
		WithMenus(func(q *gen.MenuQuery) { q.Select(menu.FieldID) }).
		WithCurrentVersion().
		WithTenantBindings().
		Where(ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return ConvertSlice(res, r.entToProto), nil
}

func (r *menuPermissionGroupRepo) Delete(ctx context.Context, id uint32) error {
	group, err := r.Data.DB(ctx).MenuPermissionGroup.Query().
		Where(menupermissiongroup.IDEQ(id)).
		WithTenantBindings().
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return pb.ErrorResourceNotFound("权限组不存在")
		}
		return err
	}
	if convert.ToValue(group.IsSystem) {
		return pb.ErrorBadRequest("系统内置权限组不能删除")
	}
	if len(group.Edges.TenantBindings) > 0 {
		return pb.ErrorBadRequest("权限组仍被租户绑定，不能删除")
	}
	return r.Data.DB(ctx).MenuPermissionGroup.DeleteOneID(id).Exec(ctx)
}

func (r *menuPermissionGroupRepo) UpdateStatus(ctx context.Context, id uint32, status pbEnum.Status) (*pbCore.MenuPermissionGroup, error) {
	affectedTenantIDs, err := r.tenantIDsByGroup(ctx, id)
	if err != nil {
		return nil, err
	}
	res, err := r.Data.DB(ctx).MenuPermissionGroup.UpdateOneID(id).
		SetStatus(int32(status)).
		Save(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("权限组不存在")
		}
		return nil, err
	}
	r.bumpTenantsPackageVersion(ctx, affectedTenantIDs)
	return r.FindByID(ctx, res.ID)
}

func (r *menuPermissionGroupRepo) ListVersions(ctx context.Context, groupID uint32) ([]*pbCore.MenuPermissionGroupVersion, error) {
	if groupID == 0 {
		return nil, pb.ErrorBadRequest("套餐ID不能为空")
	}
	exists, err := r.Data.DB(ctx).MenuPermissionGroup.Query().Where(menupermissiongroup.IDEQ(groupID)).Exist(ctx)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, pb.ErrorResourceNotFound("套餐不存在")
	}
	rows, err := r.Data.DB(ctx).MenuPermissionGroupVersion.Query().
		Where(menupermissiongroupversion.GroupIDEQ(groupID)).
		WithMenus(func(q *gen.MenuQuery) { q.Select(menu.FieldID) }).
		Order(gen.Desc(menupermissiongroupversion.FieldVersion)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return ConvertSlice(rows, menuPermissionGroupVersionToProto), nil
}

func (r *menuPermissionGroupRepo) PublishVersion(ctx context.Context, groupID uint32, input *pbCore.MenuPermissionGroupVersion, summary string, operatorID uint32, effectiveAt string) (*pbCore.MenuPermissionGroupVersion, error) {
	if input == nil {
		input = &pbCore.MenuPermissionGroupVersion{}
	}
	if err := r.validateMenuIDs(ctx, input.GetMenuIds()); err != nil {
		return nil, err
	}
	now := time.Now()
	if effectiveAt != "" {
		value, err := time.Parse(time.DateTime, effectiveAt)
		if err != nil {
			return nil, pb.ErrorBadRequest("生效时间格式必须为 YYYY-MM-DD HH:mm:ss")
		}
		if value.After(now.Add(time.Minute)) {
			return nil, pb.ErrorBadRequest("定时发布将在统一异步任务中心完成后开放")
		}
	}
	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return nil, err
	}
	defer rollback(tx, r.Log)
	current, err := tx.MenuPermissionGroup.Query().
		Where(menupermissiongroup.IDEQ(groupID)).
		WithMenus(func(q *gen.MenuQuery) { q.Select(menu.FieldID) }).
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("套餐不存在")
		}
		return nil, err
	}
	snapshot := mergeVersionSnapshot(current, input)
	if _, err = tx.MenuPermissionGroup.UpdateOneID(groupID).SetUpdatedAt(now).Save(ctx); err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("套餐不存在")
		}
		return nil, err
	}
	version, err := r.publishVersionTx(ctx, tx, groupID, snapshot, summary, operatorID)
	if err != nil {
		return nil, err
	}
	affectedTenantIDs, err := r.advanceAutoUpgradeBindingsTx(ctx, tx, groupID, version.ID)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	r.bumpTenantsPackageVersion(ctx, affectedTenantIDs)
	published, err := r.Data.DB(ctx).MenuPermissionGroupVersion.Query().
		Where(menupermissiongroupversion.IDEQ(version.ID)).
		WithMenus(func(q *gen.MenuQuery) { q.Select(menu.FieldID) }).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	return menuPermissionGroupVersionToProto(published), nil
}

func (r *menuPermissionGroupRepo) RollbackVersion(ctx context.Context, groupID, sourceVersionID uint32, summary string, operatorID uint32) (*pbCore.MenuPermissionGroupVersion, error) {
	source, err := r.Data.DB(ctx).MenuPermissionGroupVersion.Query().
		Where(
			menupermissiongroupversion.IDEQ(sourceVersionID),
			menupermissiongroupversion.GroupIDEQ(groupID),
		).
		WithMenus(func(q *gen.MenuQuery) { q.Select(menu.FieldID) }).
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("回滚来源版本不存在")
		}
		return nil, err
	}
	if summary == "" {
		summary = "回滚至版本 " + strconv.Itoa(int(source.Version))
	}
	return r.PublishVersion(ctx, groupID, menuPermissionGroupVersionToProto(source), summary, operatorID, "")
}

func (r *menuPermissionGroupRepo) createVersionTx(ctx context.Context, tx *gen.Tx, groupID uint32, version int32, snapshot *pbCore.MenuPermissionGroupVersion, summary string, operatorID uint32) (*gen.MenuPermissionGroupVersion, error) {
	if snapshot == nil {
		snapshot = &pbCore.MenuPermissionGroupVersion{}
	}
	now := time.Now()
	builder := tx.MenuPermissionGroupVersion.Create().
		SetGroupID(groupID).
		SetVersion(version).
		SetState(int32(pbCore.MenuPermissionGroupVersionState_MENU_PERMISSION_GROUP_VERSION_STATE_PUBLISHED)).
		SetChangeSummary(summary).
		SetEffectiveAt(now).
		SetPublishedAt(now).
		SetAPIPermissions(normalizeStringList(snapshot.GetApiPermissions())).
		SetFeatureFlags(copyBoolMap(snapshot.GetFeatureFlags())).
		SetResourceQuotas(copyInt64Map(snapshot.GetResourceQuotas()))
	if operatorID > 0 {
		builder.SetCreatedBy(operatorID).SetPublishedBy(operatorID)
	}
	if ids := uniquePositiveIDs(snapshot.GetMenuIds()); len(ids) > 0 {
		builder.AddMenuIDs(ids...)
	}
	return builder.Save(ctx)
}

func (r *menuPermissionGroupRepo) publishVersionTx(ctx context.Context, tx *gen.Tx, groupID uint32, snapshot *pbCore.MenuPermissionGroupVersion, summary string, operatorID uint32) (*gen.MenuPermissionGroupVersion, error) {
	latest, err := tx.MenuPermissionGroupVersion.Query().
		Where(menupermissiongroupversion.GroupIDEQ(groupID)).
		Order(gen.Desc(menupermissiongroupversion.FieldVersion)).
		First(ctx)
	nextVersion := int32(1)
	if err == nil {
		nextVersion = latest.Version + 1
	} else if !gen.IsNotFound(err) {
		return nil, err
	}
	if _, err = tx.MenuPermissionGroupVersion.Update().
		Where(
			menupermissiongroupversion.GroupIDEQ(groupID),
			menupermissiongroupversion.StateEQ(int32(pbCore.MenuPermissionGroupVersionState_MENU_PERMISSION_GROUP_VERSION_STATE_PUBLISHED)),
		).
		SetState(int32(pbCore.MenuPermissionGroupVersionState_MENU_PERMISSION_GROUP_VERSION_STATE_SUPERSEDED)).
		Save(ctx); err != nil {
		return nil, err
	}
	version, err := r.createVersionTx(ctx, tx, groupID, nextVersion, snapshot, summary, operatorID)
	if err != nil {
		if gen.IsConstraintError(err) {
			return nil, kratosErrors.Conflict("MENU_PERMISSION_GROUP_VERSION_CONFLICT", "套餐版本发布冲突，请重试")
		}
		return nil, err
	}
	builder := tx.MenuPermissionGroup.UpdateOneID(groupID).
		SetCurrentVersionID(version.ID).
		ClearMenus().
		SetAPIPermissions(normalizeStringList(snapshot.GetApiPermissions())).
		SetFeatureFlags(copyBoolMap(snapshot.GetFeatureFlags())).
		SetResourceQuotas(copyInt64Map(snapshot.GetResourceQuotas()))
	if ids := uniquePositiveIDs(snapshot.GetMenuIds()); len(ids) > 0 {
		builder.AddMenuIDs(ids...)
	}
	if _, err = builder.Save(ctx); err != nil {
		return nil, err
	}
	return version, nil
}

func (r *menuPermissionGroupRepo) advanceAutoUpgradeBindingsTx(ctx context.Context, tx *gen.Tx, groupID, versionID uint32) ([]uint32, error) {
	bindings, err := tx.TenantPermissionGroup.Query().
		Where(
			tenantpermissiongroup.GroupIDEQ(groupID),
			tenantpermissiongroup.AutoUpgradeEQ(true),
			tenantpermissiongroup.EnabledEQ(true),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uint32, 0, len(bindings))
	bindingIDs := make([]uint32, 0, len(bindings))
	for _, binding := range bindings {
		ids = append(ids, binding.TenantID)
		bindingIDs = append(bindingIDs, binding.ID)
	}
	if len(bindingIDs) > 0 {
		if _, err = tx.TenantPermissionGroup.Update().
			Where(tenantpermissiongroup.IDIn(bindingIDs...)).
			SetVersionID(versionID).
			Save(ctx); err != nil {
			return nil, err
		}
	}
	return uniquePositiveIDs(ids), nil
}

func sameUint32Set(left, right []uint32) bool {
	a := uniquePositiveIDs(left)
	b := uniquePositiveIDs(right)
	if len(a) != len(b) {
		return false
	}
	sort.Slice(a, func(i, j int) bool { return a[i] < a[j] })
	sort.Slice(b, func(i, j int) bool { return b[i] < b[j] })
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func sameStringSet(left, right []string) bool {
	a := normalizeStringList(left)
	b := normalizeStringList(right)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func normalizeStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		set[value] = struct{}{}
	}
	if len(set) == 0 {
		return nil
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func copyBoolMap(values map[string]bool) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]bool, len(values))
	for key, value := range values {
		if key == "" {
			continue
		}
		result[key] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func copyInt64Map(values map[string]int64) map[string]int64 {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]int64, len(values))
	for key, value := range values {
		if key == "" {
			continue
		}
		result[key] = value
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func mergeFeatureFlags(target map[string]bool, source map[string]bool) {
	for key, value := range copyBoolMap(source) {
		if value {
			target[key] = true
			continue
		}
		if _, ok := target[key]; !ok {
			target[key] = false
		}
	}
}

func mergeResourceQuotas(target map[string]int64, source map[string]int64) {
	for key, value := range copyInt64Map(source) {
		current, ok := target[key]
		if !ok || value > current {
			target[key] = value
		}
	}
}

func groupVersionSnapshotFromGroup(group *pbCore.MenuPermissionGroup) *pbCore.MenuPermissionGroupVersion {
	if group == nil {
		return &pbCore.MenuPermissionGroupVersion{}
	}
	return &pbCore.MenuPermissionGroupVersion{
		MenuIds:        uniquePositiveIDs(group.GetMenuIds()),
		ApiPermissions: normalizeStringList(group.GetApiPermissions()),
		FeatureFlags:   copyBoolMap(group.GetFeatureFlags()),
		ResourceQuotas: copyInt64Map(group.GetResourceQuotas()),
	}
}

func mergeVersionSnapshot(current *gen.MenuPermissionGroup, input *pbCore.MenuPermissionGroupVersion) *pbCore.MenuPermissionGroupVersion {
	if input == nil {
		input = &pbCore.MenuPermissionGroupVersion{}
	}
	snapshot := &pbCore.MenuPermissionGroupVersion{
		MenuIds:        menuPermissionGroupMenuIDs(current),
		ApiPermissions: normalizeStringList(current.APIPermissions),
		FeatureFlags:   copyBoolMap(current.FeatureFlags),
		ResourceQuotas: copyInt64Map(current.ResourceQuotas),
	}
	if input.MenuIds != nil {
		snapshot.MenuIds = uniquePositiveIDs(input.GetMenuIds())
	}
	if input.ApiPermissions != nil {
		snapshot.ApiPermissions = normalizeStringList(input.GetApiPermissions())
	}
	if input.FeatureFlags != nil {
		snapshot.FeatureFlags = copyBoolMap(input.GetFeatureFlags())
	}
	if input.ResourceQuotas != nil {
		snapshot.ResourceQuotas = copyInt64Map(input.GetResourceQuotas())
	}
	return snapshot
}

func (r *menuPermissionGroupRepo) GetTenantGroups(ctx context.Context, tenantID uint32) ([]*pbCore.MenuPermissionGroup, error) {
	if tenantID == 0 {
		return nil, pb.ErrorBadRequest("租户ID不能为空")
	}
	bindings, err := r.Data.DB(ctx).TenantPermissionGroup.Query().
		Where(tenantpermissiongroup.TenantIDEQ(tenantID), tenantpermissiongroup.EnabledEQ(true)).
		WithGroup(func(q *gen.MenuPermissionGroupQuery) {
			q.WithMenus(func(mq *gen.MenuQuery) { mq.Select(menu.FieldID) })
			q.WithCurrentVersion()
			q.WithTenantBindings()
		}).
		All(ctx)
	if err != nil {
		return nil, err
	}
	groups := make([]*pbCore.MenuPermissionGroup, 0, len(bindings))
	for _, binding := range bindings {
		group, err := binding.Edges.GroupOrErr()
		if err != nil {
			continue
		}
		groups = append(groups, r.entToProto(group))
	}
	return groups, nil
}

func (r *menuPermissionGroupRepo) GetTenantGroupBindings(ctx context.Context, tenantID uint32) ([]*pbCore.TenantPermissionGroupBinding, error) {
	if tenantID == 0 {
		return nil, pb.ErrorBadRequest("租户ID不能为空")
	}
	rows, err := r.Data.DB(ctx).TenantPermissionGroup.Query().
		Where(tenantpermissiongroup.TenantIDEQ(tenantID), tenantpermissiongroup.EnabledEQ(true)).
		WithVersion().
		Order(gen.Asc(tenantpermissiongroup.FieldGroupID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]*pbCore.TenantPermissionGroupBinding, 0, len(rows))
	for _, row := range rows {
		item := &pbCore.TenantPermissionGroupBinding{
			TenantId:    row.TenantID,
			GroupId:     row.GroupID,
			Enabled:     row.Enabled,
			BoundBy:     row.BoundBy,
			BoundAt:     convert.TimeValueToString(&row.CreatedAt, time.DateTime),
			VersionId:   row.VersionID,
			AutoUpgrade: &row.AutoUpgrade,
		}
		if version, edgeErr := row.Edges.VersionOrErr(); edgeErr == nil {
			item.Version = &version.Version
		}
		result = append(result, item)
	}
	return result, nil
}

func (r *menuPermissionGroupRepo) GetTenantCapabilities(ctx context.Context, tenantID uint32) (*pbCore.GetCurrentTenantCapabilitiesResponse, error) {
	if tenantID == 0 {
		return nil, pb.ErrorBadRequest("租户ID不能为空")
	}
	rows, err := r.Data.DB(ctx).TenantPermissionGroup.Query().
		Where(tenantpermissiongroup.TenantIDEQ(tenantID), tenantpermissiongroup.EnabledEQ(true)).
		WithGroup(func(q *gen.MenuPermissionGroupQuery) {
			q.Where(menupermissiongroup.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED))).
				Select(
					menupermissiongroup.FieldID,
					menupermissiongroup.FieldAPIPermissions,
					menupermissiongroup.FieldFeatureFlags,
					menupermissiongroup.FieldResourceQuotas,
				)
		}).
		WithVersion(func(q *gen.MenuPermissionGroupVersionQuery) {
			q.Select(
				menupermissiongroupversion.FieldID,
				menupermissiongroupversion.FieldVersion,
				menupermissiongroupversion.FieldAPIPermissions,
				menupermissiongroupversion.FieldFeatureFlags,
				menupermissiongroupversion.FieldResourceQuotas,
			)
		}).
		Order(gen.Asc(tenantpermissiongroup.FieldGroupID)).
		All(ctx)
	if err != nil {
		return nil, err
	}

	apiSet := make(map[string]struct{})
	featureFlags := make(map[string]bool)
	resourceQuotas := make(map[string]int64)
	groupIDs := make([]uint32, 0, len(rows))
	bindings := make([]*pbCore.TenantPermissionGroupBinding, 0, len(rows))
	for _, row := range rows {
		group, edgeErr := row.Edges.GroupOrErr()
		if edgeErr != nil {
			continue
		}
		groupIDs = append(groupIDs, row.GroupID)
		apiPermissions := group.APIPermissions
		flags := group.FeatureFlags
		quotas := group.ResourceQuotas

		item := &pbCore.TenantPermissionGroupBinding{
			TenantId:    row.TenantID,
			GroupId:     row.GroupID,
			Enabled:     row.Enabled,
			BoundBy:     row.BoundBy,
			BoundAt:     convert.TimeValueToString(&row.CreatedAt, time.DateTime),
			VersionId:   row.VersionID,
			AutoUpgrade: &row.AutoUpgrade,
		}
		if version, versionErr := row.Edges.VersionOrErr(); versionErr == nil {
			apiPermissions = version.APIPermissions
			flags = version.FeatureFlags
			quotas = version.ResourceQuotas
			item.Version = &version.Version
		}
		for _, permission := range normalizeStringList(apiPermissions) {
			apiSet[permission] = struct{}{}
		}
		mergeFeatureFlags(featureFlags, flags)
		mergeResourceQuotas(resourceQuotas, quotas)
		bindings = append(bindings, item)
	}

	apiPermissions := make([]string, 0, len(apiSet))
	for permission := range apiSet {
		apiPermissions = append(apiPermissions, permission)
	}
	sort.Strings(apiPermissions)
	groupIDs = uniquePositiveIDs(groupIDs)
	sort.Slice(groupIDs, func(i, j int) bool { return groupIDs[i] < groupIDs[j] })

	return &pbCore.GetCurrentTenantCapabilitiesResponse{
		TenantId:       tenantID,
		ApiPermissions: apiPermissions,
		FeatureFlags:   copyBoolMap(featureFlags),
		ResourceQuotas: copyInt64Map(resourceQuotas),
		GroupIds:       groupIDs,
		Bindings:       bindings,
	}, nil
}

func (r *menuPermissionGroupRepo) UpdateTenantGroups(ctx context.Context, tenantID uint32, groupIDs []uint32, operatorID uint32) error {
	if tenantID == 0 {
		return pb.ErrorBadRequest("租户ID不能为空")
	}
	exists, err := r.Data.DB(ctx).Tenant.Query().Where(tenant.IDEQ(tenantID)).Exist(ctx)
	if err != nil {
		return err
	}
	if !exists {
		return pb.ErrorResourceNotFound("租户不存在")
	}
	ids := uniquePositiveIDs(groupIDs)
	if len(ids) > 0 {
		count, err := r.Data.DB(ctx).MenuPermissionGroup.Query().
			Where(
				menupermissiongroup.IDIn(ids...),
				menupermissiongroup.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED)),
			).
			Count(ctx)
		if err != nil {
			return err
		}
		if count != len(ids) {
			return pb.ErrorBadRequest("存在无效权限组ID")
		}
	}
	tx, err := r.Data.DB(ctx).Tx(ctx)
	if err != nil {
		return err
	}
	defer rollback(tx, r.Log)
	existing, err := tx.TenantPermissionGroup.Query().
		Where(tenantpermissiongroup.TenantIDEQ(tenantID)).
		All(ctx)
	if err != nil {
		return err
	}
	byGroup := make(map[uint32]*gen.TenantPermissionGroup, len(existing))
	for _, item := range existing {
		byGroup[item.GroupID] = item
	}
	for _, id := range ids {
		if item, ok := byGroup[id]; ok {
			builder := tx.TenantPermissionGroup.UpdateOneID(item.ID).SetEnabled(true)
			if operatorID > 0 {
				builder.SetBoundBy(operatorID)
			}
			if _, err = builder.Save(ctx); err != nil {
				return err
			}
			delete(byGroup, id)
			continue
		}
		group, err := tx.MenuPermissionGroup.Get(ctx, id)
		if err != nil {
			return err
		}
		builder := tx.TenantPermissionGroup.Create().
			SetTenantID(tenantID).
			SetGroupID(id).
			SetEnabled(true).
			SetAutoUpgrade(true).
			SetNillableVersionID(group.CurrentVersionID)
		if operatorID > 0 {
			builder.SetBoundBy(operatorID)
		}
		if _, err := builder.Save(ctx); err != nil {
			return err
		}
	}
	for _, item := range byGroup {
		if err = tx.TenantPermissionGroup.DeleteOneID(item.ID).Exec(ctx); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	r.bumpTenantPackageVersion(ctx, tenantID)
	return nil
}

func (r *menuPermissionGroupRepo) GetTenantEffectiveMenuIDs(ctx context.Context, tenantID uint32) ([]uint32, error) {
	if tenantID == 0 {
		return nil, pb.ErrorBadRequest("租户ID不能为空")
	}
	if ids, ok := r.getTenantEffectiveMenuIDsCache(ctx, tenantID); ok {
		return ids, nil
	}
	bindings, err := r.Data.DB(ctx).TenantPermissionGroup.Query().
		Where(tenantpermissiongroup.TenantIDEQ(tenantID), tenantpermissiongroup.EnabledEQ(true)).
		WithGroup(func(q *gen.MenuPermissionGroupQuery) {
			q.Where(menupermissiongroup.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED)))
			q.WithMenus(func(mq *gen.MenuQuery) {
				mq.Where(menu.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED)))
				mq.Select(menu.FieldID)
			})
		}).
		WithVersion(func(vq *gen.MenuPermissionGroupVersionQuery) {
			vq.WithMenus(func(mq *gen.MenuQuery) {
				mq.Where(menu.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED))).Select(menu.FieldID)
			})
		}).
		All(ctx)
	if err != nil {
		return nil, err
	}
	idSet := make(map[uint32]struct{})
	for _, binding := range bindings {
		group, err := binding.Edges.GroupOrErr()
		if err != nil {
			continue
		}
		version, versionErr := binding.Edges.VersionOrErr()
		menus := group.Edges.Menus
		if versionErr == nil {
			menus = version.Edges.Menus
		}
		for _, m := range menus {
			if m != nil {
				idSet[m.ID] = struct{}{}
			}
		}
	}
	ids := make([]uint32, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	r.setTenantEffectiveMenuIDsCache(ctx, tenantID, ids)
	return ids, nil
}

func (r *menuPermissionGroupRepo) UpdateTenantGroupVersion(ctx context.Context, tenantID, groupID, versionID uint32, autoUpgrade bool, operatorID uint32) (*pbCore.TenantPermissionGroupBinding, error) {
	binding, err := r.Data.DB(ctx).TenantPermissionGroup.Query().
		Where(
			tenantpermissiongroup.TenantIDEQ(tenantID),
			tenantpermissiongroup.GroupIDEQ(groupID),
			tenantpermissiongroup.EnabledEQ(true),
		).
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("租户未绑定该套餐")
		}
		return nil, err
	}
	if autoUpgrade {
		group, err := r.Data.DB(ctx).MenuPermissionGroup.Get(ctx, groupID)
		if err != nil {
			return nil, err
		}
		if group.CurrentVersionID == nil {
			return nil, pb.ErrorBadRequest("套餐尚无已发布版本")
		}
		versionID = *group.CurrentVersionID
	} else {
		if versionID == 0 {
			return nil, pb.ErrorBadRequest("固定版本时必须指定版本ID")
		}
		exists, err := r.Data.DB(ctx).MenuPermissionGroupVersion.Query().
			Where(
				menupermissiongroupversion.IDEQ(versionID),
				menupermissiongroupversion.GroupIDEQ(groupID),
			).
			Exist(ctx)
		if err != nil {
			return nil, err
		}
		if !exists {
			return nil, pb.ErrorBadRequest("指定版本不属于该套餐")
		}
	}
	builder := r.Data.DB(ctx).TenantPermissionGroup.UpdateOneID(binding.ID).
		SetVersionID(versionID).
		SetAutoUpgrade(autoUpgrade)
	if operatorID > 0 {
		builder.SetBoundBy(operatorID)
	}
	updated, err := builder.Save(ctx)
	if err != nil {
		return nil, err
	}
	r.bumpTenantPackageVersion(ctx, tenantID)
	version, err := r.Data.DB(ctx).MenuPermissionGroupVersion.Get(ctx, versionID)
	if err != nil {
		return nil, err
	}
	return &pbCore.TenantPermissionGroupBinding{
		TenantId:    updated.TenantID,
		GroupId:     updated.GroupID,
		Enabled:     updated.Enabled,
		BoundBy:     updated.BoundBy,
		BoundAt:     convert.TimeValueToString(&updated.CreatedAt, time.DateTime),
		VersionId:   updated.VersionID,
		Version:     &version.Version,
		AutoUpgrade: &updated.AutoUpgrade,
	}, nil
}

func (r *menuPermissionGroupRepo) tenantIDsByGroup(ctx context.Context, groupID uint32) ([]uint32, error) {
	if groupID == 0 {
		return nil, nil
	}
	bindings, err := r.Data.DB(ctx).TenantPermissionGroup.Query().
		Where(tenantpermissiongroup.GroupIDEQ(groupID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]uint32, 0, len(bindings))
	for _, binding := range bindings {
		if binding != nil && binding.TenantID > 0 {
			ids = append(ids, binding.TenantID)
		}
	}
	return uniquePositiveIDs(ids), nil
}

func (r *menuPermissionGroupRepo) bumpTenantsPackageVersion(ctx context.Context, tenantIDs []uint32) {
	for _, tenantID := range uniquePositiveIDs(tenantIDs) {
		r.bumpTenantPackageVersion(ctx, tenantID)
	}
}

func (r *menuPermissionGroupRepo) GetTenantEffectiveMenus(ctx context.Context, tenantID uint32, parentID uint32) ([]*pbCore.Menu, error) {
	ids, err := r.GetTenantEffectiveMenuIDs(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}
	menus, err := r.Data.DB(ctx).Menu.Query().
		Where(menu.IDIn(ids...), menu.StatusEQ(int32(pbEnum.Status_STATUS_ENABLED))).
		Order(gen.Asc(menu.FieldSort, menu.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	menus, err = withMenuAncestors(ctx, r.Data.DB(ctx), menus)
	if err != nil {
		return nil, err
	}
	tree := buildMenuTree(convert.SliceToAny(menus, r.mr.convertProto))
	if parentID == 0 {
		return tree, nil
	}
	return filterMenuTreeByParent(tree, parentID), nil
}

func (r *menuPermissionGroupRepo) ValidateTenantMenuIDs(ctx context.Context, menuIDs []uint32) error {
	ids := uniquePositiveIDs(menuIDs)
	if len(ids) == 0 {
		return nil
	}
	tenantID, err := requireTenantID(ctx)
	if err != nil {
		return err
	}
	effectiveIDs, err := r.GetTenantEffectiveMenuIDs(ctx, tenantID)
	if err != nil {
		return err
	}
	allowed := make(map[uint32]struct{}, len(effectiveIDs))
	for _, id := range effectiveIDs {
		allowed[id] = struct{}{}
	}
	for _, id := range ids {
		if _, ok := allowed[id]; !ok {
			return pb.ErrorRolePermissionInvalid("角色菜单超出租户权限组授权范围")
		}
	}
	return nil
}

func uniquePositiveIDs(ids []uint32) []uint32 {
	if len(ids) == 0 {
		return nil
	}
	seen := make(map[uint32]struct{}, len(ids))
	result := make([]uint32, 0, len(ids))
	for _, id := range ids {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func menuPermissionGroupMenuIDs(e *gen.MenuPermissionGroup) []uint32 {
	if e == nil || len(e.Edges.Menus) == 0 {
		return nil
	}
	ids := make([]uint32, 0, len(e.Edges.Menus))
	for _, m := range e.Edges.Menus {
		if m != nil {
			ids = append(ids, m.ID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func filterMenuTreeByParent(items []*pbCore.Menu, parentID uint32) []*pbCore.Menu {
	for _, item := range items {
		if item == nil {
			continue
		}
		if item.GetId() == parentID {
			return item.Children
		}
		if children := filterMenuTreeByParent(item.Children, parentID); len(children) > 0 {
			return children
		}
	}
	return nil
}
