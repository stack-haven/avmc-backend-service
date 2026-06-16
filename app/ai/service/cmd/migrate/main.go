package main

import (
	"context"
	"flag"
	"os"

	"backend-service/app/ai/service/internal/data"
	"backend-service/app/ai/service/internal/runtimeconfig"

	"github.com/go-kratos/kratos/v2/log"
)

var flagconf string

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf ./configs")
}

func main() {
	flag.Parse()
	logger := log.NewStdLogger(os.Stdout)

	bc, err := runtimeconfig.Load(flagconf)
	if err != nil {
		panic(err)
	}
	if err := data.RunSchemaMigration(context.Background(), bc.Data, logger); err != nil {
		panic(err)
	}
}
