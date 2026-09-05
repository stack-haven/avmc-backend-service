package main

import (
	"context"
	"fmt"
	"path/filepath"

	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/conf"
)

func main() {
	dictAbs, _ := filepath.Abs("app/evie/tool/configs/dictionaries/system.json")
	vb, _ := biz.NewVocabularyBuilder(&conf.SystemDict{Path: dictAbs})
	ctx := context.Background()

	sysSnap := vb.Build(ctx, "")
	fmt.Printf("entries: %d, relations: %d\n", sysSnap.EntryCount(), sysSnap.RelationCount())
	fmt.Println("--- Relations ---")
	for rt, rs := range sysSnap.Relations {
		for _, r := range rs {
			fmt.Printf("  %q -> [%s] entry_id=%d target_id=%d\n",
				rt, r.RelationType, r.EntryID, r.TargetEntryID)
		}
	}
	fmt.Println("--- Entries ---")
	for k, e := range sysSnap.Entries {
		fmt.Printf("  [%d] %s\n", e.ID, k)
	}
}
