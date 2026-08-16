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

	{Parent: "EviePlatform", Name: "EvieDictionary", Title: "字典中心", Path: "/evie/dictionary", Component: "/evie/dictionary/list", Icon: "mdi:book-cog-outline", Type: 2, Sort: 10},
	{Parent: "EviePlatform", Name: "EvieHotword", Title: "热词管理", Path: "/evie/hotword", Component: "/evie/hotword/list", Icon: "mdi:tag-text-outline", Type: 2, Sort: 20},
	{Parent: "EviePlatform", Name: "EvieASR", Title: "语音识别", Path: "/evie/asr", Component: "/evie/asr/index", Icon: "mdi:microphone-outline", Type: 2, Sort: 30},
	{Parent: "EviePlatform", Name: "EvieProvider", Title: "供应商管理", Path: "/evie/provider", Component: "/evie/provider/list", Icon: "mdi:server-network-outline", Type: 2, Sort: 40},
	{Parent: "EviePlatform", Name: "EvieCorrection", Title: "纠错引擎", Path: "/evie/correction", Component: "/evie/correction/index", Icon: "mdi:auto-fix", Type: 2, Sort: 50},
}

// evieButtons Evie 产品服务权限按钮（引用 evie 生成的 Operation 常量）。
var evieButtons = []buttonSpec{
	// 字典中心
	{Parent: "EvieDictionary", Name: "EvieDictQuery", Title: "查询", Operation: eviev1.OperationDictionaryServiceListWords, Sort: 10},
	{Parent: "EvieDictionary", Name: "EvieDictGet", Title: "详情", Operation: eviev1.OperationDictionaryServiceGetWord, Sort: 20},
	{Parent: "EvieDictionary", Name: "EvieDictCreate", Title: "新增", Operation: eviev1.OperationDictionaryServiceCreateWord, Sort: 30},
	{Parent: "EvieDictionary", Name: "EvieDictEdit", Title: "编辑", Operation: eviev1.OperationDictionaryServiceUpdateWord, Sort: 40},
	{Parent: "EvieDictionary", Name: "EvieDictDelete", Title: "删除", Operation: eviev1.OperationDictionaryServiceDeleteWord, Sort: 50},

	// 热词管理
	{Parent: "EvieHotword", Name: "EvieHotwordQuery", Title: "查询", Operation: eviev1.OperationHotwordServiceListHotwords, Sort: 10},
	{Parent: "EvieHotword", Name: "EvieHotwordUpsert", Title: "新增/编辑", Operation: eviev1.OperationHotwordServiceUpsertHotword, Sort: 20},
	{Parent: "EvieHotword", Name: "EvieHotwordDelete", Title: "删除", Operation: eviev1.OperationHotwordServiceDeleteHotword, Sort: 30},

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

	// 纠错引擎
	{Parent: "EvieCorrection", Name: "EvieCorrectionCorrect", Title: "纠错引擎", Operation: eviev1.OperationCorrectionServiceCorrect, Sort: 10},
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
