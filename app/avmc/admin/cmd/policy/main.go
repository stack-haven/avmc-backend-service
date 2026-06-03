package main

import (
	"context"
	"flag"
	"os"
	"strings"

	"backend-service/app/avmc/admin/internal/authzpolicy"
	"backend-service/app/avmc/admin/internal/runtimeconfig"
	"backend-service/pkg/auth/authz"
	"backend-service/pkg/auth/authz/casbin"

	"github.com/go-kratos/kratos/v2/log"
)

var (
	flagconf string
	domain   string
	role     string
	users    string
)

func init() {
	flag.StringVar(&flagconf, "conf", "../../configs", "config path, eg: -conf ./configs")
	flag.StringVar(&domain, "domain", "1", "authorization domain/tenant id")
	flag.StringVar(&role, "role", "super_admin", "role subject to seed")
	flag.StringVar(&users, "users", "1", "comma-separated user ids to bind to role")
}

func main() {
	flag.Parse()
	logger := log.NewHelper(log.NewStdLogger(os.Stdout))

	bc, err := runtimeconfig.Load(flagconf)
	if err != nil {
		panic(err)
	}

	provider := casbin.NewProvider()
	authorizer, err := provider.NewAuthorizer(
		context.Background(),
		authz.WithAdapterType(authz.AdapterMySQL),
		authz.WithAdapterDSN(bc.Data.Database.Source),
	)
	if err != nil {
		panic(err)
	}
	defer authorizer.Close()

	subjects := parseSubjects(users)
	if err := authzpolicy.SyncSuperAdmin(context.Background(), authorizer, authz.Subject(role), authz.Domain(domain), subjects); err != nil {
		panic(err)
	}
	logger.Infof("synced admin policies: role=%s domain=%s users=%v", role, domain, subjects)
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
