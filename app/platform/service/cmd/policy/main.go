package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"backend-service/app/platform/service/internal/authzpolicy"
	"backend-service/app/platform/service/internal/runtimeconfig"
	"backend-service/pkg/auth/authz"
	"backend-service/pkg/auth/authz/casbin"

	"github.com/go-kratos/kratos/v2/log"
)

var (
	flagconf string
	tenant   string
	role     string
	users    string
	platform bool
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf ./configs")
	flag.StringVar(&tenant, "tenant", "1", "authorization tenant/tenant id")
	flag.StringVar(&role, "role", "1", "role subject to seed")
	flag.StringVar(&users, "users", "1", "comma-separated user ids to bind to role")
	flag.BoolVar(&platform, "platform", true, "seed platform control-plane policies")
}

func main() {
	flag.Parse()
	logger := log.NewHelper(log.NewStdLogger(os.Stdout))
	if err := run(context.Background(), logger); err != nil {
		fmt.Fprintf(os.Stderr, "admin policy sync failed: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *log.Helper) error {
	bc, err := runtimeconfig.Load(flagconf)
	if err != nil {
		return err
	}

	provider := casbin.NewProvider()
	authorizer, err := provider.NewAuthorizer(
		ctx,
		authz.WithAdapterType(authz.AdapterMySQL),
		authz.WithAdapterDSN(bc.Data.Database.Source),
	)
	if err != nil {
		return err
	}
	defer authorizer.Close()

	subjects := parseSubjects(users)
	sync := authzpolicy.SyncSuperAdmin
	if platform {
		sync = authzpolicy.SyncPlatformAdmin
	}
	if err := sync(ctx, authorizer, authz.Subject(role), authz.Tenant(tenant), subjects); err != nil {
		return err
	}
	logger.Infof("synced admin policies: role=%s tenant=%s platform=%t users=%v", role, tenant, platform, subjects)
	logger.Info("restart the admin service or reload Casbin policies before testing newly synced permissions")
	return nil
}

func parseSubjects(raw string) []authz.Subject {
	parts := strings.Split(raw, ",")
	subjects := make([]authz.Subject, 0, len(parts))
	for _, part := range parts {
		if v := strings.TrimSpace(part); v != "" {
			subjects = append(subjects, authz.Subject(v))
		}
	}
	return subjects
}
