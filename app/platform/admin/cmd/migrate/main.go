package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/go-kratos/kratos/v2/log"

	"backend-service/app/platform/admin/internal/data"
	"backend-service/app/platform/admin/internal/runtimeconfig"
)

var flagconf string

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf ./configs")
}

func main() {
	flag.Parse()
	logger := log.NewStdLogger(os.Stdout)
	if err := run(context.Background(), logger); err != nil {
		fmt.Fprintf(os.Stderr, "admin migration failed: %v\n", err)
		os.Exit(1) //nolint:forbidigo // CLI tool exit
	}
}

func run(ctx context.Context, logger log.Logger) error {
	bc, err := runtimeconfig.Load(flagconf)
	if err != nil {
		return err
	}
	return data.RunSchemaMigration(ctx, bc.Data, logger)
}
