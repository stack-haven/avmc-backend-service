//go:build mock

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"backend-service/app/platform/service/internal/data"
	entviewer "backend-service/app/platform/service/internal/data/ent/viewer"
	"backend-service/app/platform/service/internal/runtimeconfig"
	authzEngine "backend-service/pkg/auth/authz"
	authzCasbin "backend-service/pkg/auth/authz/casbin"

	"github.com/go-kratos/kratos/v2/log"
	_ "github.com/go-sql-driver/mysql"
)

const mockPassword = "Admin@123456"

var flagconf = "../../configs"

func main() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path")
	flag.Parse()
	if err := run(context.Background()); err != nil {
		fmt.Fprintf(os.Stderr, "admin mock failed: %v\n", err)
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

	if err := seed(ctx, client, authorizer); err != nil {
		return err
	}
	if err := verify(ctx, client); err != nil {
		return err
	}
	fmt.Printf("admin mock data ready: tenants=[技术中台管理(1),客户企业(2)] users=[admin,vben,jack,operator,tenant2] password=%s\n", mockPassword)
	return nil
}
