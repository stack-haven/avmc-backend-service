//go:build mock

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"backend-service/app/evie/service/internal/data"
	"backend-service/app/evie/service/internal/data/ent/gen"
	"backend-service/app/evie/service/internal/data/ent/gen/dictionaryconflict"
	"backend-service/app/evie/service/internal/data/ent/gen/dictionaryentry"
	"backend-service/app/evie/service/internal/data/ent/gen/dictionaryversion"
	"backend-service/app/evie/service/internal/data/ent/gen/enhancementpolicy"
	"backend-service/app/evie/service/internal/data/ent/gen/enhancementlog"
	entviewer "backend-service/app/evie/service/internal/data/ent/viewer"
	"backend-service/app/evie/service/internal/runtimeconfig"

	"github.com/go-kratos/kratos/v2/log"
	_ "github.com/go-sql-driver/mysql"
)

var flagconf = "../../configs"

// bulk 数据规模（稳定性测试用，幂等：已存在则跳过）。
const (
	dictPerTenant1 = 10
	dictPerTenant2 = 5
	entryPerDict1  = 60
	entryPerDict2  = 40
	relationPer1   = 3
	relationPer2   = 2
	versionPerDict = 2
	conflictCount1 = 100
	policyCount1   = 5
	policyCount2   = 3
	profilePerPol1 = 2
	profilePerPol2 = 1
	logCount1      = 100
)

func main() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path")
	flag.Parse()
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "evie mock failed: %v\n", err)
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

	if err := seed(ctx, client); err != nil {
		return err
	}
	fmt.Println("evie mock data ready: bulk dictionary/enhancement data seeded")
	return nil
}

// seed 幂等生成大量词库中心 + 文本增强数据。
func seed(ctx context.Context, client *gen.Client) error {
	if err := seedDictionaries(ctx, client); err != nil {
		return err
	}
	if err := seedVersions(ctx, client); err != nil {
		return err
	}
	if err := seedConflicts(ctx, client); err != nil {
		return err
	}
	if err := seedPoliciesAndProfiles(ctx, client); err != nil {
		return err
	}
	if err := seedLogs(ctx, client); err != nil {
		return err
	}
	return nil
}

// hasBulkData 判断是否已生成过 bulk 数据（避免重复）。
func hasBulkData(ctx context.Context, client *gen.Client) bool {
	cnt, _ := client.DictionaryEntry.Query().
		Where(dictionaryentry.TenantIDEQ(1)).
		Count(ctx)
	return cnt >= entryPerDict1*dictPerTenant1
}

// seedDictionaries 创建词库 + 词条 + 关系。
func seedDictionaries(ctx context.Context, client *gen.Client) error {
	if hasBulkData(ctx, client) {
		fmt.Println("  bulk dictionary data already exists, skip")
		return nil
	}

	// 租户 1
	for d := 1; d <= dictPerTenant1; d++ {
		dictName := fmt.Sprintf("企业词库-%02d", d)
		dict, err := client.Dictionary.Create().
			SetTenantID(1).
			SetName(dictName).
			SetScope("TENANT").
			SetSource("MANUAL").
			SetDescription(fmt.Sprintf("租户 1 稳定性测试词库 %d", d)).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create dictionary %s: %w", dictName, err)
		}
		if err := seedEntries(ctx, client, 1, dict.ID, entryPerDict1, relationPer1); err != nil {
			return err
		}
	}

	// 租户 2
	for d := 1; d <= dictPerTenant2; d++ {
		dictName := fmt.Sprintf("客户B词库-%02d", d)
		dict, err := client.Dictionary.Create().
			SetTenantID(2).
			SetName(dictName).
			SetScope("TENANT").
			SetSource("IMPORT").
			SetDescription(fmt.Sprintf("租户 2 稳定性测试词库 %d", d)).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create dictionary %s: %w", dictName, err)
		}
		if err := seedEntries(ctx, client, 2, dict.ID, entryPerDict2, relationPer2); err != nil {
			return err
		}
	}

	fmt.Printf("  created %d dictionaries + %d entries + relations\n",
		dictPerTenant1+dictPerTenant2,
		dictPerTenant1*entryPerDict1+dictPerTenant2*entryPerDict2,
	)
	return nil
}

func seedEntries(ctx context.Context, client *gen.Client, tenantID, dictID uint32, count, relCount int) error {
	for i := 1; i <= count; i++ {
		text := fmt.Sprintf("标准词%d", i)
		entry, err := client.DictionaryEntry.Create().
			SetTenantID(tenantID).
			SetDictionaryID(dictID).
			SetStandardText(text).
			SetEntryType("WORD").
			SetCategory(categoryFor(i)).
			SetDescription("bulk mock").
			SetSource("MANUAL").
			SetPriority(int32(i % 10)).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create entry %s: %w", text, err)
		}
		for r := 1; r <= relCount; r++ {
			relType := "ALIAS"
			if r == 2 {
				relType = "CORRECTION"
			} else if r == 3 {
				relType = "RELATED"
			}
			relatedText := fmt.Sprintf("别名%d-%d-%d", i, r, dictID)
			if _, err := client.DictionaryRelation.Create().
				SetTenantID(tenantID).
				SetEntryID(entry.ID).
				SetRelationType(relType).
				SetRelatedText(relatedText).
				SetSource("MANUAL").
				Save(ctx); err != nil {
				return fmt.Errorf("create relation %s: %w", relatedText, err)
			}
		}
	}
	return nil
}

func categoryFor(i int) string {
	cats := []string{"PERSON", "ORGANIZATION", "PRODUCT", "TERM", "LOCATION", "INDUSTRY", "BRAND"}
	return cats[i%len(cats)]
}

// seedVersions 每个词库发布版本。
func seedVersions(ctx context.Context, client *gen.Client) error {
	dicts, err := client.Dictionary.Query().All(ctx)
	if err != nil {
		return err
	}
	created := 0
	for _, dict := range dicts {
		cnt, err := client.DictionaryVersion.Query().
			Where(dictionaryversion.DictionaryIDEQ(dict.ID)).
			Count(ctx)
		if err != nil {
			return err
		}
		for v := int(cnt) + 1; v <= versionPerDict; v++ {
			if _, err := client.DictionaryVersion.Create().
				SetTenantID(dict.TenantID).
				SetDictionaryID(dict.ID).
				SetVersionNo(int32(v)).
				SetDescription(fmt.Sprintf("版本 %d", v)).
				SetSnapshot(fmt.Sprintf("{\"dictionary\":%d,\"version\":%d}", dict.ID, v)).
				Save(ctx); err != nil {
				return fmt.Errorf("create version: %w", err)
			}
			created++
		}
	}
	if created > 0 {
		fmt.Printf("  created %d versions\n", created)
	}
	return nil
}

// seedConflicts 生成冲突记录。
func seedConflicts(ctx context.Context, client *gen.Client) error {
	cnt, err := client.DictionaryConflict.Query().
		Where(dictionaryconflict.TenantIDEQ(1)).
		Count(ctx)
	if err != nil {
		return err
	}
	created := 0
	for i := int(cnt) + 1; i <= conflictCount1; i++ {
		if _, err := client.DictionaryConflict.Create().
			SetTenantID(1).
			SetInput(fmt.Sprintf("冲突输入%d", i)).
			SetCandidate(fmt.Sprintf("候选结果%d", i)).
			SetSourceScope("TENANT").
			SetSourceDictionary("企业词库-01").
			SetPriority(int32(i % 10)).
			SetResolvedCandidate(fmt.Sprintf("解析结果%d", i)).
			Save(ctx); err != nil {
			return fmt.Errorf("create conflict: %w", err)
		}
		created++
	}
	if created > 0 {
		fmt.Printf("  created %d conflicts\n", created)
	}
	return nil
}

// seedPoliciesAndProfiles 创建增强策略 + 场景。
func seedPoliciesAndProfiles(ctx context.Context, client *gen.Client) error {
	modes := []string{"HIGH_PERFORMANCE", "STANDARD", "HIGH_ACCURACY"}
	for tid, count := range map[uint32]int{1: policyCount1, 2: policyCount2} {
		existing, err := client.EnhancementPolicy.Query().
			Where(enhancementpolicy.TenantIDEQ(tid)).
			Count(ctx)
		if err != nil {
			return err
		}
		createdPol := 0
		for i := existing + 1; i <= count; i++ {
			mode := modes[i%len(modes)]
			pol, err := client.EnhancementPolicy.Create().
				SetTenantID(tid).
				SetName(fmt.Sprintf("租户%d策略-%02d", tid, i)).
				SetMode(mode).
				SetTextCleaning(true).
				SetFillerRemoval(i%2 == 0).
				SetAliasResolution(true).
				SetDeterministicReplacement(true).
				SetPinyinCorrection(i%3 == 0).
				SetFuzzyMatching(i%2 != 0).
				SetContextCorrection(i%2 == 0).
				SetDescription("bulk mock policy").
				Save(ctx)
			if err != nil {
				return fmt.Errorf("create policy: %w", err)
			}
			profiles := profilePerPol1
			if tid == 2 {
				profiles = profilePerPol2
			}
			for p := 1; p <= profiles; p++ {
				if _, err := client.EnhancementProfile.Create().
					SetTenantID(tid).
					SetPolicyID(pol.ID).
					SetName(fmt.Sprintf("租户%d场景-%02d-%d", tid, i, p)).
					SetDescription("bulk mock profile").
					Save(ctx); err != nil {
					return fmt.Errorf("create profile: %w", err)
				}
			}
			createdPol++
		}
		if createdPol > 0 {
			fmt.Printf("  created %d policies for tenant %d\n", createdPol, tid)
		}
	}
	return nil
}

// seedLogs 生成增强日志（历史记录）。
func seedLogs(ctx context.Context, client *gen.Client) error {
	cnt, err := client.EnhancementLog.Query().
		Where(enhancementlog.TenantIDEQ(1)).
		Count(ctx)
	if err != nil {
		return err
	}
	created := 0
	for i := int(cnt) + 1; i <= logCount1; i++ {
		raw := fmt.Sprintf("历史识别文本%d", i)
		enhanced := raw
		if i%3 == 0 {
			enhanced = raw + "（增强）"
		}
		status := int32(1)
		if i%10 == 0 {
			status = 2 // 降级
		}
		if _, err := client.EnhancementLog.Create().
			SetTenantID(1).
			SetRequestID(fmt.Sprintf("req-%d", i)).
			SetSessionID(fmt.Sprintf("session-%d", i)).
			SetRawText(raw).
			SetEnhancedText(enhanced).
			SetStatus(status).
			SetProcessingTimeMs(int64(i%50)).
			SetCleaningTimeMs(int64(i%5)).
			SetAliasTimeMs(int64(i%3)).
			Save(ctx); err != nil {
			return fmt.Errorf("create log: %w", err)
		}
		created++
	}
	if created > 0 {
		fmt.Printf("  created %d enhancement logs\n", created)
	}
	return nil
}
