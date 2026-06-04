package main

import (
	"context"
	"flag"
	"math"
	"os"

	"backend-service/app/avmc/admin/internal/data"
	"backend-service/app/avmc/admin/internal/runtimeconfig"

	"github.com/go-kratos/kratos/v2/log"
)

var (
	flagconf     string
	legacyDomain uint64
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf ./configs")
	flag.Uint64Var(&legacyDomain, "legacy-domain", 0, "assign all legacy Admin tenant data to this domain before schema migration")
}

func main() {
	flag.Parse()
	logger := log.NewStdLogger(os.Stdout)

	bc, err := runtimeconfig.Load(flagconf)
	if err != nil {
		panic(err)
	}
	if legacyDomain > 0 {
		if legacyDomain > math.MaxUint32 {
			panic("legacy-domain exceeds uint32 range")
		}
		if err := data.RunLegacyTenantBackfill(context.Background(), bc.Data, uint32(legacyDomain), logger); err != nil {
			panic(err)
		}
	}
	if err := data.RunSchemaMigration(context.Background(), bc.Data, logger); err != nil {
		panic(err)
	}
}
