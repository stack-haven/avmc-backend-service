// Command evie-menu registers the Evie product service's menus into the
// platform menu system and syncs the corresponding Casbin authorization policies.
//
// It is the interim replacement for the not-yet-built "产品注册中心" (product
// registration center). Each product service owns a declarative menu manifest
// (menuSpec below); this command materializes it into system_menus, binds it to
// roles, and rebuilds role policies + user-role membership in Casbin.
//
// Usage:
//
//	cd app/platform/admin && go run ./cmd/evie-menu -conf ./configs
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"backend-service/app/platform/admin/internal/data"
	"backend-service/app/platform/admin/internal/data/ent/gen"
	"backend-service/app/platform/admin/internal/data/ent/gen/menu"
	"backend-service/app/platform/admin/internal/data/ent/gen/role"
	entviewer "backend-service/app/platform/admin/internal/data/ent/viewer"
	"backend-service/app/platform/admin/internal/runtimeconfig"
	authzEngine "backend-service/pkg/auth/authz"
	authzCasbin "backend-service/pkg/auth/authz/casbin"

	"github.com/go-kratos/kratos/v2/log"
	_ "github.com/go-sql-driver/mysql"
)

var flagconf string

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf ./configs")
}

// menuSpec is a declarative description of a menu node.
type menuSpec struct {
	name      string // unique name key
	title     string
	path      string // route path (type 1/2); empty for buttons
	component string
	icon      string
	typ       int32 // 1=dir 2=item 3=button
	sort      int32
	authCode  string // operation path for buttons, e.g. /evie.service.v1.UserService/ListUsers
	parent    string // parent name key, empty for top-level
}

// evieMenus is the Evie product's menu manifest. Extend this list as Evie
// exposes more product modules (dictionary, hotword, ASR, correction...).
var evieMenus = []menuSpec{
	{name: "EviePlatform", title: "语音智能引擎", path: "/evie", component: "BasicLayout", icon: "mdi:microphone-message-outline", typ: 1, sort: 60},

	{name: "EvieDictionary", title: "字典中心", path: "/evie/dictionary", component: "/evie/dictionary/list", icon: "mdi:book-cog-outline", typ: 2, sort: 10, parent: "EviePlatform"},

	{name: "EvieDictQuery", title: "查询", authCode: "/evie.service.v1.DictionaryService/ListWords", typ: 3, sort: 10, parent: "EvieDictionary"},
	{name: "EvieDictGet", title: "详情", authCode: "/evie.service.v1.DictionaryService/GetWord", typ: 3, sort: 20, parent: "EvieDictionary"},
	{name: "EvieDictCreate", title: "新增", authCode: "/evie.service.v1.DictionaryService/CreateWord", typ: 3, sort: 30, parent: "EvieDictionary"},
	{name: "EvieDictEdit", title: "编辑", authCode: "/evie.service.v1.DictionaryService/UpdateWord", typ: 3, sort: 40, parent: "EvieDictionary"},
	{name: "EvieDictDelete", title: "删除", authCode: "/evie.service.v1.DictionaryService/DeleteWord", typ: 3, sort: 50, parent: "EvieDictionary"},

	{name: "EvieHotword", title: "热词管理", path: "/evie/hotword", component: "/evie/hotword/list", icon: "mdi:tag-text-outline", typ: 2, sort: 20, parent: "EviePlatform"},

	{name: "EvieHotwordQuery", title: "查询", authCode: "/evie.service.v1.HotwordService/ListHotwords", typ: 3, sort: 10, parent: "EvieHotword"},
	{name: "EvieHotwordUpsert", title: "新增/编辑", authCode: "/evie.service.v1.HotwordService/UpsertHotword", typ: 3, sort: 20, parent: "EvieHotword"},
	{name: "EvieHotwordDelete", title: "删除", authCode: "/evie.service.v1.HotwordService/DeleteHotword", typ: 3, sort: 30, parent: "EvieHotword"},

	{name: "EvieASR", title: "语音识别", path: "/evie/asr", component: "/evie/asr/index", icon: "mdi:microphone-outline", typ: 2, sort: 30, parent: "EviePlatform"},

	{name: "EvieASRRecognize", title: "识别", authCode: "/evie.service.v1.ASRService/Recognize", typ: 3, sort: 10, parent: "EvieASR"},
	{name: "EvieASRRecognizeAndCorrect", title: "识别+纠错", authCode: "/evie.service.v1.ASRService/RecognizeAndCorrect", typ: 3, sort: 15, parent: "EvieASR"},
	{name: "EvieASRRecordQuery", title: "记录查询", authCode: "/evie.service.v1.ASRService/ListAsrRecords", typ: 3, sort: 20, parent: "EvieASR"},
	{name: "EvieASRRecordGet", title: "记录详情", authCode: "/evie.service.v1.ASRService/GetAsrRecord", typ: 3, sort: 30, parent: "EvieASR"},
	{name: "EvieASRRecordAudio", title: "音频预览", authCode: "/evie.service.v1.ASRService/GetAsrRecordAudio", typ: 3, sort: 40, parent: "EvieASR"},
	{name: "EvieASRReRecognize", title: "重新识别", authCode: "/evie.service.v1.ASRService/ReRecognize", typ: 3, sort: 50, parent: "EvieASR"},

	{name: "EvieProvider", title: "供应商管理", path: "/evie/provider", component: "/evie/provider/list", icon: "mdi:server-network-outline", typ: 2, sort: 40, parent: "EviePlatform"},

	{name: "EvieProviderList", title: "可用列表", authCode: "/evie.service.v1.ProviderService/ListAvailableProviders", typ: 3, sort: 10, parent: "EvieProvider"},
	{name: "EvieProviderConfigQuery", title: "租户配置查询", authCode: "/evie.service.v1.ProviderService/GetTenantConfig", typ: 3, sort: 20, parent: "EvieProvider"},
	{name: "EvieProviderConfigUpdate", title: "配置更新", authCode: "/evie.service.v1.ProviderService/UpdateTenantConfig", typ: 3, sort: 30, parent: "EvieProvider"},

	{name: "EvieCorrection", title: "纠错引擎", path: "/evie/correction", component: "/evie/correction/index", icon: "mdi:auto-fix", typ: 2, sort: 50, parent: "EviePlatform"},

	{name: "EvieCorrectionCorrect", title: "纠错", authCode: "/evie.service.v1.CorrectionService/Correct", typ: 3, sort: 10, parent: "EvieCorrection"},
}

func main() {
	flag.Parse()
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "evie menu registration failed: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	ctx = entviewer.NewSystemContext(ctx)
	bc, err := runtimeconfig.Load(flagconf)
	if err != nil {
		return err
	}
	client, err := data.NewEntClient(bc.Data, log.DefaultLogger)
	if err != nil {
		return err
	}
	defer client.Close()

	authorizer, err := authzCasbin.NewProvider().NewAuthorizer(
		ctx,
		authzEngine.WithAdapterType(authzEngine.AdapterMySQL),
		authzEngine.WithAdapterDSN(bc.Data.Database.Source),
	)
	if err != nil {
		return err
	}
	defer authorizer.Close()

	if err := registerMenus(ctx, client); err != nil {
		return err
	}
	if err := bindAndSync(ctx, client, authorizer); err != nil {
		return err
	}

	fmt.Println("evie menus registered and policies synced. restart platform/admin or reload Casbin before testing.")
	return nil
}

// registerMenus materializes the declarative manifest into system_menus.
func registerMenus(ctx context.Context, client *gen.Client) error {
	ids := make(map[string]uint32, len(evieMenus))
	for _, s := range evieMenus {
		var parentID uint32
		if s.parent != "" {
			pid, ok := ids[s.parent]
			if !ok {
				return fmt.Errorf("menu %s references unknown parent %s", s.name, s.parent)
			}
			parentID = pid
		}
		id, err := ensureMenu(ctx, client, s, parentID)
		if err != nil {
			return err
		}
		ids[s.name] = id
	}
	return nil
}

func ensureMenu(ctx context.Context, c *gen.Client, s menuSpec, parentID uint32) (uint32, error) {
	ex, err := c.Menu.Query().Where(menu.NameEQ(s.name)).Only(ctx)
	if err != nil {
		m, err := c.Menu.Create().
			SetName(s.name).
			SetTitle(s.title).
			SetPath(s.path).
			SetComponent(s.component).
			SetIcon(s.icon).
			SetType(s.typ).
			SetSort(s.sort).
			SetNillableAuthCode(&s.authCode).
			SetNillableParentID(&parentID).
			Save(ctx)
		if err != nil {
			return 0, fmt.Errorf("create menu %s: %w", s.name, err)
		}
		return m.ID, nil
	}
	if _, err := c.Menu.UpdateOneID(ex.ID).
		SetTitle(s.title).
		SetPath(s.path).
		SetComponent(s.component).
		SetIcon(s.icon).
		SetType(s.typ).
		SetSort(s.sort).
		SetNillableAuthCode(&s.authCode).
		SetNillableParentID(&parentID).
		Save(ctx); err != nil {
		return 0, fmt.Errorf("update menu %s: %w", s.name, err)
	}
	return ex.ID, nil
}

// bindAndSync binds evie menus to tenant-1 roles, rebuilds role policies, and
// repairs missing Casbin user→role memberships.
func bindAndSync(ctx context.Context, client *gen.Client, authorizer authzEngine.Authorizer) error {
	// All evie menu IDs (dir + item + buttons).
	evieIDs := make([]uint32, 0, len(evieMenus))
	for _, s := range evieMenus {
		m, err := client.Menu.Query().Where(menu.NameEQ(s.name)).Only(ctx)
		if err != nil {
			return fmt.Errorf("query evie menu %s: %w", s.name, err)
		}
		evieIDs = append(evieIDs, m.ID)
	}

	// Bind to tenant-1 roles: 超级管理员 (1) and 普通用户 (2).
	roles, err := client.Role.Query().Where(role.TenantIDEQ(1)).All(ctx)
	if err != nil {
		return fmt.Errorf("query tenant-1 roles: %w", err)
	}
	for _, r := range roles {
		if err := client.Role.UpdateOneID(r.ID).AddMenuIDs(evieIDs...).Exec(ctx); err != nil {
			return fmt.Errorf("bind evie menus to role %d: %w", r.ID, err)
		}
		if err := data.SyncRolePolicies(ctx, client, authorizer, 1, r.ID); err != nil {
			return fmt.Errorf("sync policies for role %d: %w", r.ID, err)
		}
		name := ""
		if r.Name != nil {
			name = *r.Name
		}
		fmt.Printf("synced evie policies for role id=%d name=%s\n", r.ID, name)
	}

	// Repair missing Casbin g rules: users 2 (vben) and 3 (jack) are bound to
	// role 1 in system_user_roles but lack the corresponding Casbin g rule.
	for _, uid := range []uint32{2, 3} {
		if err := data.SyncUserRole(ctx, authorizer, 1, uid, 1, true); err != nil {
			return fmt.Errorf("sync g rule for user %d: %w", uid, err)
		}
		fmt.Printf("repaired g rule user=%d role=1 tenant=1\n", uid)
	}
	return nil
}
