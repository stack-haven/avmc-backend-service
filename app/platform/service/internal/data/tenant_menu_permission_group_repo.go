package data

import (
	pbEnum "backend-service/api/common/enum"
	pb "backend-service/api/platform/service/v1"
	"backend-service/app/platform/service/internal/biz"
	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/app/platform/service/internal/data/ent/gen/menu"
	"backend-service/app/platform/service/internal/data/ent/gen/role"
	"backend-service/app/platform/service/internal/data/ent/gen/tenant"
	"backend-service/app/platform/service/internal/data/ent/gen/tenantmenupermissiongroup"
	"backend-service/app/platform/service/internal/data/ent/gen/tenantmenupermissiongroupversion"
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

var _ biz.TenantMenuPermissionGroupRepo = (*tenantMenuPermissionGroupRepo)(nil)

type tenantMenuPermissionGroupRepo struct {
	BaseRepo
	mr *menuRepo
}

func NewTenantMenuPermissionGroupRepo(data *Data, logger log.Logger) biz.TenantMenuPermissionGroupRepo {
	return &tenantMenuPermissionGroupRepo{
		BaseRepo: NewBaseRepo(data, logger),
		mr:       NewMenuRepo(data, logger).(*menuRepo),
	}
}

func (r *tenantMenuPermissionGroupRepo) validateMenuIDs(ctx context.Context, menuIDs []uint32) error {
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

func (r *tenantMenuPermissionGroupRepo) entToProto(e *gen.TenantMenuPermissionGroup) *pb.TenantMenuPermissionGroup {
	if e == nil {
		return nil
	}
	status := pbEnum.Status(0)
	if e.Status != nil {
		status = pbEnum.Status(*e.Status)
	}
	result := &pb.TenantMenuPermissionGroup{
		Id:             e.ID,
		Name:           convert.ToPointer(e.Name),
		Code:           convert.ToPointer(e.Code),
		Status:         &status,
		IsSystem:       e.IsSystem,
		Sort:           e.Sort,
		Description:    e.Description,
		Remark:         e.Remark,
		MenuIds:        TenantMenuPermissionGroupMenuIDs(e),
		ApiPermissions: normalizeStringList(e.APIPermissions),
		FeatureFlags:   copyBoolMap(e.FeatureFlags),
		ResourceQuotas: copyInt64Map(e.ResourceQuotas),
		TenantCount:    nil,
		CreatedAt:      convert.TimeValueToString(&e.CreatedAt, time.DateTime),
		UpdatedAt:      convert.TimeValueToString(&e.UpdatedAt, time.DateTime),
	}
	if current, err := e.Edges.CurrentVersionOrErr(); err == nil {
		result.CurrentVersionId = &current.ID
		result.CurrentVersion = &current.Version
	}
	return result
}

func TenantMenuPermissionGroupVersionToProto(e *gen.TenantMenuPermissionGroupVersion) *pb.TenantMenuPermissionGroupVersion {
	if e == nil {
		return nil
	}
	return &pb.TenantMenuPermissionGroupVersion{
		Id:             e.ID,
		GroupId:        e.GroupID,
		Version:        e.Version,
		State:          pb.TenantMenuPermissionGroupVersionState(e.State),
		MenuIds:        TenantMenuPermissionGroupVersionMenuIDs(e),
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

func TenantMenuPermissionGroupVersionMenuIDs(e *gen.TenantMenuPermissionGroupVersion) []uint32 {
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

func (r *tenantMenuPermissionGroupRepo) Save(ctx context.Context, g *pb.TenantMenuPermissionGroup) (*pb.TenantMenuPermissionGroup, error) {
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
	builder := tx.TenantMenuPermissionGroup.Create().
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
	if _, err = tx.TenantMenuPermissionGroup.UpdateOneID(res.ID).SetCurrentVersionID(version.ID).Save(ctx); err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, res.ID)
}

func (r *tenantMenuPermissionGroupRepo) Update(ctx context.Context, g *pb.TenantMenuPermissionGroup) (*pb.TenantMenuPermissionGroup, error) {
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
	builder := tx.TenantMenuPermissionGroup.UpdateOneID(g.GetId()).
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
	if versionChanged {
		_, publishErr := r.publishVersionTx(ctx, tx, g.GetId(), &pb.TenantMenuPermissionGroupVersion{
			MenuIds:        targetMenuIDs,
			ApiPermissions: targetAPIPermissions,
			FeatureFlags:   targetFeatureFlags,
			ResourceQuotas: targetResourceQuotas,
		}, "通过套餐编辑发布", 0)
		if publishErr != nil {
			return nil, publishErr
		}
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	return r.FindByID(ctx, res.ID)
}

func (r *tenantMenuPermissionGroupRepo) FindByID(ctx context.Context, id uint32) (*pb.TenantMenuPermissionGroup, error) {
	res, err := r.Data.DB(ctx).TenantMenuPermissionGroup.Query().
		Where(tenantmenupermissiongroup.IDEQ(id)).
		WithMenus(func(q *gen.MenuQuery) { q.Select(menu.FieldID) }).
		WithCurrentVersion().
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("权限组不存在")
		}
		return nil, err
	}
	return r.entToProto(res), nil
}

func (r *tenantMenuPermissionGroupRepo) CountTenantMenuPermissionGroups(ctx context.Context, opts ...listing.Option) (int32, error) {
	o := listing.Options{}
	for _, opt := range opts {
		opt(&o)
	}
	count, err := r.Data.DB(ctx).TenantMenuPermissionGroup.Query().
		Where(ents.ApplyFilter(o.Filter)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int32(count), nil
}

func (r *tenantMenuPermissionGroupRepo) ListTenantMenuPermissionGroups(ctx context.Context, opts ...listing.Option) ([]*pb.TenantMenuPermissionGroup, error) {
	o := listing.Options{Limit: 20}
	for _, opt := range opts {
		opt(&o)
	}
	res, err := r.Data.DB(ctx).TenantMenuPermissionGroup.Query().
		WithMenus(func(q *gen.MenuQuery) { q.Select(menu.FieldID) }).
		WithCurrentVersion().
		Where(ents.ApplyFilter(o.Filter)).
		Order(ents.ApplyOrderBy(o.OrderBy)).
		Offset(o.Offset).Limit(o.Limit).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(res, r.entToProto), nil
}

func (r *tenantMenuPermissionGroupRepo) Delete(ctx context.Context, id uint32) error {
	group, err := r.Data.DB(ctx).TenantMenuPermissionGroup.Query().
		Where(tenantmenupermissiongroup.IDEQ(id)).
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
	return r.Data.DB(ctx).TenantMenuPermissionGroup.DeleteOneID(id).Exec(ctx)
}

func (r *tenantMenuPermissionGroupRepo) UpdateStatus(ctx context.Context, id uint32, status pbEnum.Status) (*pb.TenantMenuPermissionGroup, error) {
	res, err := r.Data.DB(ctx).TenantMenuPermissionGroup.UpdateOneID(id).
		SetStatus(int32(status)).
		Save(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("权限组不存在")
		}
		return nil, err
	}
	return r.FindByID(ctx, res.ID)
}

func (r *tenantMenuPermissionGroupRepo) ListVersions(ctx context.Context, groupID uint32) ([]*pb.TenantMenuPermissionGroupVersion, error) {
	if groupID == 0 {
		return nil, pb.ErrorBadRequest("套餐ID不能为空")
	}
	exists, err := r.Data.DB(ctx).TenantMenuPermissionGroup.Query().Where(tenantmenupermissiongroup.IDEQ(groupID)).Exist(ctx)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, pb.ErrorResourceNotFound("套餐不存在")
	}
	rows, err := r.Data.DB(ctx).TenantMenuPermissionGroupVersion.Query().
		Where(tenantmenupermissiongroupversion.GroupIDEQ(groupID)).
		WithMenus(func(q *gen.MenuQuery) { q.Select(menu.FieldID) }).
		Order(gen.Desc(tenantmenupermissiongroupversion.FieldVersion)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return convert.SliceToAny(rows, TenantMenuPermissionGroupVersionToProto), nil
}

func (r *tenantMenuPermissionGroupRepo) PublishVersion(ctx context.Context, groupID uint32, input *pb.TenantMenuPermissionGroupVersion, summary string, operatorID uint32, effectiveAt string) (*pb.TenantMenuPermissionGroupVersion, error) {
	if input == nil {
		input = &pb.TenantMenuPermissionGroupVersion{}
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
	current, err := tx.TenantMenuPermissionGroup.Query().
		Where(tenantmenupermissiongroup.IDEQ(groupID)).
		WithMenus(func(q *gen.MenuQuery) { q.Select(menu.FieldID) }).
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("套餐不存在")
		}
		return nil, err
	}
	snapshot := mergeVersionSnapshot(current, input)
	if _, err = tx.TenantMenuPermissionGroup.UpdateOneID(groupID).SetUpdatedAt(now).Save(ctx); err != nil {
		if gen.IsNotFound(err) {
			return nil, pb.ErrorResourceNotFound("套餐不存在")
		}
		return nil, err
	}
	version, err := r.publishVersionTx(ctx, tx, groupID, snapshot, summary, operatorID)
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(); err != nil {
		return nil, err
	}
	published, err := r.Data.DB(ctx).TenantMenuPermissionGroupVersion.Query().
		Where(tenantmenupermissiongroupversion.IDEQ(version.ID)).
		WithMenus(func(q *gen.MenuQuery) { q.Select(menu.FieldID) }).
		Only(ctx)
	if err != nil {
		return nil, err
	}
	r.cascadeTenantRoleMenus(ctx, groupID)
	return TenantMenuPermissionGroupVersionToProto(published), nil
}

func (r *tenantMenuPermissionGroupRepo) RollbackVersion(ctx context.Context, groupID, sourceVersionID uint32, summary string, operatorID uint32) (*pb.TenantMenuPermissionGroupVersion, error) {
	source, err := r.Data.DB(ctx).TenantMenuPermissionGroupVersion.Query().
		Where(
			tenantmenupermissiongroupversion.IDEQ(sourceVersionID),
			tenantmenupermissiongroupversion.GroupIDEQ(groupID),
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
	return r.PublishVersion(ctx, groupID, TenantMenuPermissionGroupVersionToProto(source), summary, operatorID, "")
}

func (r *tenantMenuPermissionGroupRepo) createVersionTx(ctx context.Context, tx *gen.Tx, groupID uint32, version int32, snapshot *pb.TenantMenuPermissionGroupVersion, summary string, operatorID uint32) (*gen.TenantMenuPermissionGroupVersion, error) {
	if snapshot == nil {
		snapshot = &pb.TenantMenuPermissionGroupVersion{}
	}
	now := time.Now()
	builder := tx.TenantMenuPermissionGroupVersion.Create().
		SetGroupID(groupID).
		SetVersion(version).
		SetState(int32(pb.TenantMenuPermissionGroupVersionState_MENU_PERMISSION_GROUP_VERSION_STATE_PUBLISHED)).
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

func (r *tenantMenuPermissionGroupRepo) publishVersionTx(ctx context.Context, tx *gen.Tx, groupID uint32, snapshot *pb.TenantMenuPermissionGroupVersion, summary string, operatorID uint32) (*gen.TenantMenuPermissionGroupVersion, error) {
	latest, err := tx.TenantMenuPermissionGroupVersion.Query().
		Where(tenantmenupermissiongroupversion.GroupIDEQ(groupID)).
		Order(gen.Desc(tenantmenupermissiongroupversion.FieldVersion)).
		First(ctx)
	nextVersion := int32(1)
	if err == nil {
		nextVersion = latest.Version + 1
	} else if !gen.IsNotFound(err) {
		return nil, err
	}
	if _, err = tx.TenantMenuPermissionGroupVersion.Update().
		Where(
			tenantmenupermissiongroupversion.GroupIDEQ(groupID),
			tenantmenupermissiongroupversion.StateEQ(int32(pb.TenantMenuPermissionGroupVersionState_MENU_PERMISSION_GROUP_VERSION_STATE_PUBLISHED)),
		).
		SetState(int32(pb.TenantMenuPermissionGroupVersionState_MENU_PERMISSION_GROUP_VERSION_STATE_SUPERSEDED)).
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
	builder := tx.TenantMenuPermissionGroup.UpdateOneID(groupID).
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

func groupVersionSnapshotFromGroup(group *pb.TenantMenuPermissionGroup) *pb.TenantMenuPermissionGroupVersion {
	if group == nil {
		return &pb.TenantMenuPermissionGroupVersion{}
	}
	return &pb.TenantMenuPermissionGroupVersion{
		MenuIds:        uniquePositiveIDs(group.GetMenuIds()),
		ApiPermissions: normalizeStringList(group.GetApiPermissions()),
		FeatureFlags:   copyBoolMap(group.GetFeatureFlags()),
		ResourceQuotas: copyInt64Map(group.GetResourceQuotas()),
	}
}

func mergeVersionSnapshot(current *gen.TenantMenuPermissionGroup, input *pb.TenantMenuPermissionGroupVersion) *pb.TenantMenuPermissionGroupVersion {
	if input == nil {
		input = &pb.TenantMenuPermissionGroupVersion{}
	}
	snapshot := &pb.TenantMenuPermissionGroupVersion{
		MenuIds:        TenantMenuPermissionGroupMenuIDs(current),
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

func TenantMenuPermissionGroupMenuIDs(e *gen.TenantMenuPermissionGroup) []uint32 {
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

// ValidateTenantMenuIDs checks that all given menu IDs fall within the current
// tenant's effective menu set (union of all bound package current versions).
func (r *tenantMenuPermissionGroupRepo) ValidateTenantMenuIDs(ctx context.Context, menuIDs []uint32) error {
	if len(menuIDs) == 0 {
		return nil
	}
	tenantID, err := requireTenantID(ctx)
	if err != nil {
		return err
	}
	effective, err := r.GetTenantEffectiveMenuIDs(ctx, tenantID)
	if err != nil {
		return err
	}
	if len(effective) == 0 {
		return pb.ErrorBadRequest("当前租户未绑定任何包含菜单的套餐")
	}
	effectiveSet := make(map[uint32]bool, len(effective))
	for _, id := range effective {
		effectiveSet[id] = true
	}
	for _, id := range menuIDs {
		if !effectiveSet[id] {
			return pb.ErrorBadRequest("所选菜单不在当前租户套餐范围内")
		}
	}
	return nil
}

// GetTenantEffectiveMenuIDs returns the union of menu IDs from all packages
// bound to the given tenant (using each package's current version).
func (r *tenantMenuPermissionGroupRepo) GetTenantEffectiveMenuIDs(ctx context.Context, tenantID uint32) ([]uint32, error) {
	t, err := r.Data.DB(ctx).Tenant.Query().
		Where(tenant.IDEQ(tenantID)).
		WithGroups(func(q *gen.TenantMenuPermissionGroupQuery) {
			q.WithCurrentVersion(func(vq *gen.TenantMenuPermissionGroupVersionQuery) {
				vq.WithMenus(func(mq *gen.MenuQuery) { mq.Select(menu.FieldID) })
			})
		}).
		Only(ctx)
	if err != nil {
		if gen.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	menuSet := make(map[uint32]struct{})
	for _, pkg := range t.Edges.Groups {
		if pkg.Edges.CurrentVersion != nil {
			for _, m := range pkg.Edges.CurrentVersion.Edges.Menus {
				menuSet[m.ID] = struct{}{}
			}
		}
	}
	ids := make([]uint32, 0, len(menuSet))
	for id := range menuSet {
		ids = append(ids, id)
	}
	return ids, nil
}

// cascadeTenantRoleMenus trims role menus for all tenants bound to a package
// after the package's menu set changes (publish/rollback). Out-of-scope menus
// are removed and Casbin policies re-synced.
func (r *tenantMenuPermissionGroupRepo) cascadeTenantRoleMenus(ctx context.Context, groupID uint32) {
	// Find all tenants bound to this package
	tenants, err := r.Data.DB(ctx).Tenant.Query().
		Where(tenant.HasGroupsWith(tenantmenupermissiongroup.IDEQ(groupID))).
		All(ctx)
	if err != nil {
		return
	}
	for _, t := range tenants {
		effective, err := r.GetTenantEffectiveMenuIDs(ctx, t.ID)
		if err != nil || len(effective) == 0 {
			continue
		}
		effectiveSet := make(map[uint32]bool, len(effective))
		for _, id := range effective {
			effectiveSet[id] = true
		}
		roles, err := r.Data.DB(ctx).Role.Query().
			Where(role.TenantIDEQ(t.ID)).
			WithMenus(func(q *gen.MenuQuery) { q.Select(menu.FieldID) }).
			All(ctx)
		if err != nil {
			continue
		}
		for _, rl := range roles {
			newMenus := make([]uint32, 0, len(rl.Edges.Menus))
			changed := false
			for _, m := range rl.Edges.Menus {
				if effectiveSet[m.ID] {
					newMenus = append(newMenus, m.ID)
				} else {
					changed = true
				}
			}
			if changed {
				_, _ = r.Data.DB(ctx).Role.UpdateOneID(rl.ID).
					ClearMenus().AddMenuIDs(newMenus...).Save(ctx)
				SyncRolePolicies(ctx, r.Data.DB(ctx), r.Data.authorizer, t.ID, rl.ID)
			}
		}
	}
}

// GetTenantEffectiveMenus is a no-op — tenant system removed.
func (r *tenantMenuPermissionGroupRepo) GetTenantEffectiveMenus(_ context.Context, _ uint32, _ uint32) ([]*pb.Menu, error) {
	return nil, nil
}
