//go:build mock

package main

import (
	"context"

	eviev1 "backend-service/api/evie/service/v1"
	"backend-service/app/platform/service/internal/data/ent/gen"
)

// 本文件 seed Evie 产品服务（语音智能引擎）的菜单与权限按钮。
// 这是“产品注册中心”建成前的临时替代：产品服务的菜单清单声明式集中于此，
// 每次运行 mock 即增量注册到平台菜单系统并绑定套餐/角色。

// evieMenus Evie 产品服务菜单树（目录 + 页面）。
// 一级目录 sort=70，位于平台“通知中心(60)”之后、“系统管理(999)”之前。
var evieMenus = []menuSeed{
	{Parent: "", Name: "EviePlatform", Title: "语音智能引擎", Path: "/evie", Component: "BasicLayout", Icon: "mdi:microphone-message-outline", Type: 1, Sort: 70},

	{Parent: "EviePlatform", Name: "EvieDictionary", Title: "词库中心", Path: "/evie/dictionary", Component: "", Icon: "mdi:book-cog-outline", Type: 1, Sort: 10},
	{Parent: "EvieDictionary", Name: "EvieDictionaryList", Title: "词库管理", Path: "/evie/dictionary/dictionaries", Component: "/evie/dictionary/dictionaries/list", Icon: "mdi:bookshelf", Type: 2, Sort: 10},
	{Parent: "EvieDictionary", Name: "EvieEntryList", Title: "词条管理", Path: "/evie/dictionary/entries", Component: "/evie/dictionary/entries/list", Icon: "mdi:format-list-bulleted", Type: 2, Sort: 20},
	{Parent: "EvieDictionary", Name: "EvieRelationList", Title: "关系管理", Path: "/evie/dictionary/relations", Component: "/evie/dictionary/relations/list", Icon: "mdi:source-branch", Type: 2, Sort: 30},
	{Parent: "EvieDictionary", Name: "EvieCategoryList", Title: "分类管理", Path: "/evie/dictionary/categories", Component: "/evie/dictionary/categories/list", Icon: "mdi:shape-outline", Type: 2, Sort: 40},
	{Parent: "EvieDictionary", Name: "EvieVersionList", Title: "版本管理", Path: "/evie/dictionary/versions", Component: "/evie/dictionary/versions/list", Icon: "mdi:history", Type: 2, Sort: 50},
	{Parent: "EvieDictionary", Name: "EvieConflictList", Title: "冲突记录", Path: "/evie/dictionary/conflicts", Component: "/evie/dictionary/conflicts/list", Icon: "mdi:alert-outline", Type: 2, Sort: 60},
	{Parent: "EviePlatform", Name: "EvieASR", Title: "语音识别", Path: "/evie/asr", Component: "/evie/asr/index", Icon: "mdi:microphone-outline", Type: 2, Sort: 30},
	{Parent: "EviePlatform", Name: "EvieProvider", Title: "供应商管理", Path: "/evie/provider", Component: "/evie/provider/list", Icon: "mdi:server-network-outline", Type: 2, Sort: 40},
	{Parent: "EviePlatform", Name: "EvieEnhancement", Title: "文本增强", Path: "/evie/enhancement", Component: "", Icon: "mdi:auto-fix", Type: 1, Sort: 50},
	{Parent: "EvieEnhancement", Name: "EviePolicyList", Title: "增强策略", Path: "/evie/enhancement/policies", Component: "/evie/enhancement/policies/list", Icon: "mdi:tune-variant", Type: 2, Sort: 10},
	{Parent: "EvieEnhancement", Name: "EvieProfileList", Title: "增强场景", Path: "/evie/enhancement/profiles", Component: "/evie/enhancement/profiles/list", Icon: "mdi:view-list-outline", Type: 2, Sort: 20},
	{Parent: "EvieEnhancement", Name: "EvieLogList", Title: "增强记录", Path: "/evie/enhancement/logs", Component: "/evie/enhancement/logs/list", Icon: "mdi:clipboard-text-clock-outline", Type: 2, Sort: 30},
}

// evieButtons Evie 产品服务权限按钮（引用 evie 生成的 Operation 常量）。
var evieButtons = []buttonSpec{
	// 词库管理
	{Parent: "EvieDictionaryList", Name: "EvieDictQuery", Title: "查询", Operation: eviev1.OperationDictionaryServiceListDictionaries, Sort: 10},
	{Parent: "EvieDictionaryList", Name: "EvieDictGet", Title: "详情", Operation: eviev1.OperationDictionaryServiceGetDictionary, Sort: 20},
	{Parent: "EvieDictionaryList", Name: "EvieDictCreate", Title: "新增", Operation: eviev1.OperationDictionaryServiceCreateDictionary, Sort: 30},
	{Parent: "EvieDictionaryList", Name: "EvieDictEdit", Title: "编辑", Operation: eviev1.OperationDictionaryServiceUpdateDictionary, Sort: 40},
	{Parent: "EvieDictionaryList", Name: "EvieDictDelete", Title: "删除", Operation: eviev1.OperationDictionaryServiceDeleteDictionary, Sort: 50},
	{Parent: "EvieDictionaryList", Name: "EvieDictStats", Title: "查看统计", Operation: eviev1.OperationDictionaryServiceGetDictionaryStats, Sort: 60},
	{Parent: "EvieDictionaryList", Name: "EvieDictRelationsAll", Title: "查询词库下所有关系", Operation: eviev1.OperationDictionaryServiceListRelationsByDictionary, Sort: 70},

	// 词条管理
	{Parent: "EvieEntryList", Name: "EvieEntryQuery", Title: "查询", Operation: eviev1.OperationDictionaryServiceListEntries, Sort: 10},
	{Parent: "EvieEntryList", Name: "EvieEntryGet", Title: "详情", Operation: eviev1.OperationDictionaryServiceGetEntry, Sort: 20},
	{Parent: "EvieEntryList", Name: "EvieEntryCreate", Title: "新增", Operation: eviev1.OperationDictionaryServiceCreateEntry, Sort: 30},
	{Parent: "EvieEntryList", Name: "EvieEntryEdit", Title: "编辑", Operation: eviev1.OperationDictionaryServiceUpdateEntry, Sort: 40},
	{Parent: "EvieEntryList", Name: "EvieEntryDelete", Title: "删除", Operation: eviev1.OperationDictionaryServiceDeleteEntry, Sort: 50},

	// 关系管理
	{Parent: "EvieRelationList", Name: "EvieRelationQuery", Title: "查询", Operation: eviev1.OperationDictionaryServiceListRelations, Sort: 10},
	{Parent: "EvieRelationList", Name: "EvieRelationGet", Title: "详情", Operation: eviev1.OperationDictionaryServiceGetRelation, Sort: 20},
	{Parent: "EvieRelationList", Name: "EvieRelationCreate", Title: "新增", Operation: eviev1.OperationDictionaryServiceCreateRelation, Sort: 30},
	{Parent: "EvieRelationList", Name: "EvieRelationEdit", Title: "编辑", Operation: eviev1.OperationDictionaryServiceUpdateRelation, Sort: 40},
	{Parent: "EvieRelationList", Name: "EvieRelationDelete", Title: "删除", Operation: eviev1.OperationDictionaryServiceDeleteRelation, Sort: 50},

	// 分类管理
	{Parent: "EvieCategoryList", Name: "EvieCategoryQuery", Title: "查询", Operation: eviev1.OperationDictionaryServiceListCategories, Sort: 10},
	{Parent: "EvieCategoryList", Name: "EvieCategoryCreate", Title: "新增", Operation: eviev1.OperationDictionaryServiceCreateCategory, Sort: 20},
	{Parent: "EvieCategoryList", Name: "EvieCategoryEdit", Title: "编辑", Operation: eviev1.OperationDictionaryServiceUpdateCategory, Sort: 30},
	{Parent: "EvieCategoryList", Name: "EvieCategoryDelete", Title: "删除", Operation: eviev1.OperationDictionaryServiceDeleteCategory, Sort: 40},

	// 版本管理
	{Parent: "EvieVersionList", Name: "EvieVersionQuery", Title: "查询", Operation: eviev1.OperationDictionaryServiceListVersions, Sort: 10},
	{Parent: "EvieVersionList", Name: "EvieVersionGet", Title: "详情", Operation: eviev1.OperationDictionaryServiceGetVersion, Sort: 20},
	{Parent: "EvieVersionList", Name: "EvieVersionPublish", Title: "发布", Operation: eviev1.OperationDictionaryServicePublishDictionary, Sort: 30},

	// 冲突记录
	{Parent: "EvieConflictList", Name: "EvieConflictQuery", Title: "查询", Operation: eviev1.OperationDictionaryServiceListConflicts, Sort: 10},

	// 语音识别
	{Parent: "EvieASR", Name: "EvieASRRecognize", Title: "识别", Operation: eviev1.OperationASRServiceRecognize, Sort: 10},
	{Parent: "EvieASR", Name: "EvieASRRecognizeAndCorrect", Title: "识别+纠错", Operation: eviev1.OperationASRServiceRecognizeAndCorrect, Sort: 15},
	{Parent: "EvieASR", Name: "EvieASRRecordQuery", Title: "记录查询", Operation: eviev1.OperationASRServiceListAsrRecords, Sort: 20},
	{Parent: "EvieASR", Name: "EvieASRRecordGet", Title: "记录详情", Operation: eviev1.OperationASRServiceGetAsrRecord, Sort: 30},
	{Parent: "EvieASR", Name: "EvieASRRecordAudio", Title: "音频预览", Operation: eviev1.OperationASRServiceGetAsrRecordAudio, Sort: 40},
	{Parent: "EvieASR", Name: "EvieASRReRecognize", Title: "重新识别", Operation: eviev1.OperationASRServiceReRecognize, Sort: 50},

	// 供应商管理
	{Parent: "EvieProvider", Name: "EvieProviderList", Title: "可用列表", Operation: eviev1.OperationProviderServiceListAvailableProviders, Sort: 10},
	{Parent: "EvieProvider", Name: "EvieProviderConfigQuery", Title: "租户配置查询", Operation: eviev1.OperationProviderServiceGetTenantConfig, Sort: 20},
	{Parent: "EvieProvider", Name: "EvieProviderConfigUpdate", Title: "配置更新", Operation: eviev1.OperationProviderServiceUpdateTenantConfig, Sort: 30},

	// 增强策略
	{Parent: "EviePolicyList", Name: "EviePolicyQuery", Title: "查询", Operation: eviev1.OperationEnhancementServiceListPolicies, Sort: 10},
	{Parent: "EviePolicyList", Name: "EviePolicyGet", Title: "详情", Operation: eviev1.OperationEnhancementServiceGetPolicy, Sort: 20},
	{Parent: "EviePolicyList", Name: "EviePolicyCreate", Title: "新增", Operation: eviev1.OperationEnhancementServiceCreatePolicy, Sort: 30},
	{Parent: "EviePolicyList", Name: "EviePolicyEdit", Title: "编辑", Operation: eviev1.OperationEnhancementServiceUpdatePolicy, Sort: 40},
	{Parent: "EviePolicyList", Name: "EviePolicyDelete", Title: "删除", Operation: eviev1.OperationEnhancementServiceDeletePolicy, Sort: 50},

	// 增强场景
	{Parent: "EvieProfileList", Name: "EvieProfileQuery", Title: "查询", Operation: eviev1.OperationEnhancementServiceListProfiles, Sort: 10},
	{Parent: "EvieProfileList", Name: "EvieProfileGet", Title: "详情", Operation: eviev1.OperationEnhancementServiceGetProfile, Sort: 20},
	{Parent: "EvieProfileList", Name: "EvieProfileCreate", Title: "新增", Operation: eviev1.OperationEnhancementServiceCreateProfile, Sort: 30},
	{Parent: "EvieProfileList", Name: "EvieProfileEdit", Title: "编辑", Operation: eviev1.OperationEnhancementServiceUpdateProfile, Sort: 40},
	{Parent: "EvieProfileList", Name: "EvieProfileDelete", Title: "删除", Operation: eviev1.OperationEnhancementServiceDeleteProfile, Sort: 50},

	// 增强记录
	{Parent: "EvieLogList", Name: "EvieLogQuery", Title: "查询", Operation: eviev1.OperationEnhancementServiceListLogs, Sort: 10},
	{Parent: "EvieLogList", Name: "EvieLogGet", Title: "详情", Operation: eviev1.OperationEnhancementServiceGetLog, Sort: 20},
	{Parent: "EvieEntryList", Name: "EviePinyinGenerate", Title: "拼音生成", Operation: eviev1.OperationEnhancementServiceGeneratePinyin, Sort: 60},

	// 纠错（兼容保留，Correct 走增强引擎）
	{Parent: "EvieEnhancement", Name: "EvieCorrectionCorrect", Title: "文本增强", Operation: eviev1.OperationCorrectionServiceCorrect, Sort: 30},
}

// seedEvie 幂等维护 Evie 产品服务的菜单树与按钮，返回 evie 菜单 name→ID 映射。
func seedEvie(ctx context.Context, c *gen.Client) (map[string]uint32, error) {
	menuMap, err := seedMenuTree(ctx, c, evieMenus)
	if err != nil {
		return nil, err
	}
	if err := seedButtonList(ctx, c, evieButtons, menuMap); err != nil {
		return nil, err
	}
	return menuMap, nil
}
