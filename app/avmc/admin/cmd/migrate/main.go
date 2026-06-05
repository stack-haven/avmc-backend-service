package main

import (
	"context"
	"flag"
	"fmt"
	"math"
	"os"

	"backend-service/app/avmc/admin/internal/data"
	"backend-service/app/avmc/admin/internal/runtimeconfig"

	"github.com/go-kratos/kratos/v2/log"
)

var (
	flagconf     string
	legacyTenant uint64
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf ./configs")
	flag.Uint64Var(&legacyTenant, "legacy-tenant", 0, "assign all legacy Admin tenant data to this tenant before schema migration")
}

func main() {
	flag.Parse()
	logger := log.NewStdLogger(os.Stdout)
	if err := run(context.Background(), logger); err != nil {
		fmt.Fprintf(os.Stderr, "admin migration failed: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger log.Logger) error {
	bc, err := runtimeconfig.Load(flagconf)
	if err != nil {
		return err
	}
	if legacyTenant > 0 {
		if legacyTenant > math.MaxUint32 {
			return fmt.Errorf("legacy-tenant exceeds uint32 range")
		}
		if err := data.RunLegacyTenantBackfill(ctx, bc.Data, uint32(legacyTenant), logger); err != nil {
			return err
		}
	}
	return data.RunSchemaMigration(ctx, bc.Data, logger)
}
