//go:build mock

package main

import (
	"context"

	"backend-service/app/platform/service/internal/data/ent/gen"
	"backend-service/app/platform/service/internal/data/ent/gen/dictionaryitem"
	"backend-service/app/platform/service/internal/data/ent/gen/dictionarytype"
	"backend-service/app/platform/service/internal/data/ent/gen/notificationprovider"
	"backend-service/app/platform/service/internal/data/ent/gen/notificationtemplate"
	"backend-service/app/platform/service/internal/data/ent/gen/parameterdefinition"
	"backend-service/app/platform/service/internal/data/ent/gen/storageprovider"
)

// 本文件 seed 技术中台基座配置数据：数据字典、参数定义、通知模板/渠道、存储渠道。
// 均按唯一键幂等 upsert，可多次运行增量维护。

// ── 数据字典 ─────────────────────────────────────────────

// dictSeed 定义字典类型及其字典项。
type dictSeed struct {
	Code   string
	Name   string
	Sort   int32
	Remark string
	Items  []dictItemSeed
}

type dictItemSeed struct {
	Label  string
	Value  string
	Sort   int32
	Color  string
	Remark string
}

// platformDicts 技术中台管理租户（tenant_id=1）的数据字典。
var platformDicts = []dictSeed{
	{
		Code: "gender", Name: "性别", Sort: 10, Remark: "用户性别",
		Items: []dictItemSeed{
			{Label: "未知", Value: "0", Sort: 10},
			{Label: "男", Value: "1", Sort: 20},
			{Label: "女", Value: "2", Sort: 30},
		},
	},
	{
		Code: "common_status", Name: "通用状态", Sort: 20, Remark: "通用启用/禁用状态",
		Items: []dictItemSeed{
			{Label: "启用", Value: "1", Sort: 10, Color: "green"},
			{Label: "禁用", Value: "0", Sort: 20, Color: "red"},
		},
	},
}

// ── 参数定义 ─────────────────────────────────────────────

// paramSeed 定义参数配置中心的参数项。
// ValueType: 1字符串 2整数 3布尔 4JSON
type paramSeed struct {
	Key               string
	Name              string
	ValueType         int32
	DefaultValue      string
	Description       string
	TenantOverridable bool
	Sort              int32
}

// platformParams 平台级参数定义（tenant_overridable 表示租户是否可覆盖）。
var platformParams = []paramSeed{
	{Key: "file.upload.max_size", Name: "文件上传大小上限(字节)", ValueType: 2, DefaultValue: "10485760", Description: "单个文件上传大小上限，默认 10MB", TenantOverridable: true, Sort: 10},
	{Key: "auth.login.max_attempts", Name: "登录失败锁定阈值", ValueType: 2, DefaultValue: "5", Description: "连续登录失败达到该次数后锁定 15 分钟", TenantOverridable: true, Sort: 20},
	{Key: "notify.in_app.enable", Name: "站内信开关", ValueType: 3, DefaultValue: "true", Description: "是否启用站内信通知", TenantOverridable: true, Sort: 30},
}

// ── 通知模板 / 渠道 ─────────────────────────────────────

// notifTemplateSeed 定义通知模板（租户级）。
// Channel: 1=站内信 2=邮件 3=短信 4=Webhook
type notifTemplateSeed struct {
	Code    string
	Name    string
	Channel int32
	Title   string
	Content string
	Remark  string
}

// platformNotifTemplates 技术中台管理租户（tenant_id=1）的通知模板。
var platformNotifTemplates = []notifTemplateSeed{
	{
		Code: "system.welcome", Name: "系统欢迎", Channel: 1,
		Title:   "欢迎使用技术中台",
		Content: "您好，{{userName}}，欢迎使用技术中台，您的账号已开通。",
		Remark:  "新用户开通欢迎站内信",
	},
	{
		Code: "auth.password_reset", Name: "密码重置", Channel: 1,
		Title:   "密码重置成功",
		Content: "您好，{{userName}}，您的密码已于 {{time}} 重置。",
		Remark:  "管理员重置用户密码后通知",
	},
}

// notifProviderSeed 定义通知渠道配置（平台级）。
type notifProviderSeed struct {
	Code         string
	Name         string
	Channel      string // in-app / sms / email / webhook / push
	ProviderType string // aliyun-sms / yunpian / jpush / getui
	IsDefault    bool
	Remark       string
}

// platformNotifProviders 平台级通知渠道。
var platformNotifProviders = []notifProviderSeed{
	{Code: "in-app-default", Name: "站内信", Channel: "in-app", ProviderType: "", IsDefault: true, Remark: "站内信默认渠道"},
	{Code: "aliyun-sms", Name: "阿里云短信", Channel: "sms", ProviderType: "aliyun-sms", IsDefault: false, Remark: "短信通知渠道（密钥待配置）"},
}

// ── 存储渠道 ─────────────────────────────────────────────

// storageProviderSeed 定义存储渠道（平台级）。
type storageProviderSeed struct {
	Code          string
	Name          string
	Type          string // local / s3-compatible
	IsDefault     bool
	LocalBasePath string
	DefaultBucket string
	Remark        string
}

// platformStorageProviders 平台级存储渠道。
var platformStorageProviders = []storageProviderSeed{
	{Code: "local-default", Name: "本地存储", Type: "local", IsDefault: true, LocalBasePath: "./data/files", DefaultBucket: "tenant-files", Remark: "本地文件存储默认渠道"},
}

// seedConfig 幂等维护基座配置数据（字典/参数/通知/存储）。
func seedConfig(ctx context.Context, c *gen.Client) error {
	if err := seedDicts(ctx, c); err != nil {
		return err
	}
	if err := seedParams(ctx, c); err != nil {
		return err
	}
	if err := seedNotifTemplates(ctx, c); err != nil {
		return err
	}
	if err := seedNotifProviders(ctx, c); err != nil {
		return err
	}
	if err := seedStorageProviders(ctx, c); err != nil {
		return err
	}
	return nil
}

// ── 幂等 upsert 实现 ────────────────────────────────────

func seedDicts(ctx context.Context, c *gen.Client) error {
	for _, d := range platformDicts {
		t := ensureDictType(ctx, c, 1, d.Code, d.Name, d.Sort, d.Remark)
		for _, item := range d.Items {
			ensureDictItem(ctx, c, 1, t.ID, item.Label, item.Value, item.Sort, item.Color, item.Remark)
		}
	}
	return nil
}

func ensureDictType(ctx context.Context, c *gen.Client, tid uint32, code, name string, sort int32, remark string) *gen.DictionaryType {
	ex, err := c.DictionaryType.Query().Where(dictionarytype.TenantIDEQ(tid), dictionarytype.CodeEQ(code)).Only(ctx)
	if err != nil {
		return c.DictionaryType.Create().SetTenantID(tid).SetName(name).SetCode(code).SetSort(sort).SetRemark(remark).SetStatus(1).SaveX(ctx)
	}
	c.DictionaryType.UpdateOneID(ex.ID).SetName(name).SetSort(sort).SetRemark(remark).SetStatus(1).Exec(ctx)
	return ex
}

func ensureDictItem(ctx context.Context, c *gen.Client, tid, typeID uint32, label, value string, sort int32, color, remark string) {
	ex, err := c.DictionaryItem.Query().
		Where(
			dictionaryitem.TenantIDEQ(tid),
			dictionaryitem.TypeIDEQ(typeID),
			dictionaryitem.ValueEQ(value),
		).Only(ctx)
	if err != nil {
		c.DictionaryItem.Create().SetTenantID(tid).SetTypeID(typeID).SetLabel(label).SetValue(value).SetSort(sort).SetColor(color).SetRemark(remark).SetStatus(1).SaveX(ctx)
		return
	}
	c.DictionaryItem.UpdateOneID(ex.ID).SetLabel(label).SetSort(sort).SetColor(color).SetRemark(remark).SetStatus(1).Exec(ctx)
}

func seedParams(ctx context.Context, c *gen.Client) error {
	for _, p := range platformParams {
		ensureParamDef(ctx, c, p.Key, p.Name, p.ValueType, p.DefaultValue, p.Description, p.TenantOverridable, p.Sort)
	}
	return nil
}

func ensureParamDef(ctx context.Context, c *gen.Client, key, name string, valueType int32, defaultValue, description string, overridable bool, sort int32) {
	ex, err := c.ParameterDefinition.Query().Where(parameterdefinition.KeyEQ(key)).Only(ctx)
	if err != nil {
		c.ParameterDefinition.Create().SetKey(key).SetName(name).SetValueType(valueType).SetDefaultValue(defaultValue).SetDescription(description).SetTenantOverridable(overridable).SetStatus(1).SetSort(sort).SaveX(ctx)
		return
	}
	c.ParameterDefinition.UpdateOneID(ex.ID).SetName(name).SetValueType(valueType).SetDefaultValue(defaultValue).SetDescription(description).SetTenantOverridable(overridable).SetStatus(1).SetSort(sort).Exec(ctx)
}

func seedNotifTemplates(ctx context.Context, c *gen.Client) error {
	for _, t := range platformNotifTemplates {
		ensureNotifTemplate(ctx, c, 1, t.Code, t.Name, t.Channel, t.Title, t.Content, t.Remark)
	}
	return nil
}

func ensureNotifTemplate(ctx context.Context, c *gen.Client, tid uint32, code, name string, channel int32, title, content, remark string) {
	ex, err := c.NotificationTemplate.Query().Where(notificationtemplate.TenantIDEQ(tid), notificationtemplate.CodeEQ(code)).Only(ctx)
	if err != nil {
		c.NotificationTemplate.Create().SetTenantID(tid).SetCode(code).SetName(name).SetChannel(channel).SetTitle(title).SetContent(content).SetRemark(remark).SetStatus(1).SaveX(ctx)
		return
	}
	c.NotificationTemplate.UpdateOneID(ex.ID).SetName(name).SetChannel(channel).SetTitle(title).SetContent(content).SetRemark(remark).SetStatus(1).Exec(ctx)
}

func seedNotifProviders(ctx context.Context, c *gen.Client) error {
	for _, p := range platformNotifProviders {
		ensureNotifProvider(ctx, c, p.Code, p.Name, p.Channel, p.ProviderType, p.IsDefault, p.Remark)
	}
	return nil
}

func ensureNotifProvider(ctx context.Context, c *gen.Client, code, name, channel, providerType string, isDefault bool, remark string) {
	ex, err := c.NotificationProvider.Query().Where(notificationprovider.CodeEQ(code)).Only(ctx)
	if err != nil {
		c.NotificationProvider.Create().SetCode(code).SetName(name).SetChannel(channel).SetProviderType(providerType).SetIsDefault(isDefault).SetRemark(remark).SetStatus(1).SaveX(ctx)
		return
	}
	c.NotificationProvider.UpdateOneID(ex.ID).SetName(name).SetChannel(channel).SetProviderType(providerType).SetIsDefault(isDefault).SetRemark(remark).SetStatus(1).Exec(ctx)
}

func seedStorageProviders(ctx context.Context, c *gen.Client) error {
	for _, p := range platformStorageProviders {
		ensureStorageProvider(ctx, c, p.Code, p.Name, p.Type, p.IsDefault, p.LocalBasePath, p.DefaultBucket, p.Remark)
	}
	return nil
}

func ensureStorageProvider(ctx context.Context, c *gen.Client, code, name, typ string, isDefault bool, localBasePath, defaultBucket, remark string) {
	ex, err := c.StorageProvider.Query().Where(storageprovider.CodeEQ(code)).Only(ctx)
	if err != nil {
		c.StorageProvider.Create().SetCode(code).SetName(name).SetType(typ).SetIsDefault(isDefault).SetLocalBasePath(localBasePath).SetDefaultBucket(defaultBucket).SetRemark(remark).SetStatus(1).SaveX(ctx)
		return
	}
	c.StorageProvider.UpdateOneID(ex.ID).SetName(name).SetType(typ).SetIsDefault(isDefault).SetLocalBasePath(localBasePath).SetDefaultBucket(defaultBucket).SetRemark(remark).SetStatus(1).Exec(ctx)
}
