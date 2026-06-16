package runtimeconfig

import (
	"os"
	"strconv"
	"strings"

	"backend-service/app/version/service/internal/conf"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
)

func Load(path string) (*conf.Bootstrap, error) {
	c := config.New(config.WithSource(file.NewSource(path)))
	defer c.Close()

	if err := c.Load(); err != nil {
		return nil, err
	}
	var bc conf.Bootstrap
	if err := c.Scan(&bc); err != nil {
		return nil, err
	}
	ApplyEnvOverrides(&bc)
	if err := Validate(&bc); err != nil {
		return nil, err
	}
	return &bc, nil
}

func ApplyEnvOverrides(bc *conf.Bootstrap) {
	if bc == nil {
		return
	}
	if bc.Server != nil {
		if bc.Server.Http != nil {
			if v := strings.TrimSpace(os.Getenv("platform_VERSION_HTTP_ADDR")); v != "" {
				bc.Server.Http.Addr = v
			}
		}
		if bc.Server.Grpc != nil {
			if v := strings.TrimSpace(os.Getenv("platform_VERSION_GRPC_ADDR")); v != "" {
				bc.Server.Grpc.Addr = v
			}
		}
	}
	if bc.Data != nil {
		if bc.Data.Database != nil {
			if v := strings.TrimSpace(os.Getenv("platform_VERSION_DB_DRIVER")); v != "" {
				bc.Data.Database.Driver = v
			}
			if v := strings.TrimSpace(os.Getenv("platform_VERSION_DB_SOURCE")); v != "" {
				bc.Data.Database.Source = v
			}
			if v, ok := envBool("platform_VERSION_DB_DEBUG"); ok {
				bc.Data.Database.Debug = v
			}
			if v, ok := envBool("platform_VERSION_DB_MIGRATE"); ok {
				bc.Data.Database.Migrate = v
			}
		}
		if bc.Data.Redis != nil {
			if v := strings.TrimSpace(os.Getenv("platform_VERSION_REDIS_ADDR")); v != "" {
				bc.Data.Redis.Addr = v
			}
			if v, ok := os.LookupEnv("platform_VERSION_REDIS_PASSWORD"); ok {
				bc.Data.Redis.Password = v
			}
		}
	}
}

func Validate(bc *conf.Bootstrap) error {
	if bc == nil {
		return ConfigError("bootstrap config is required")
	}
	if bc.Server == nil {
		return ConfigError("server config is required")
	}
	if bc.Server.Http == nil || strings.TrimSpace(bc.Server.Http.Addr) == "" {
		return ConfigError("platform_VERSION_HTTP_ADDR must not be empty")
	}
	if bc.Server.Grpc == nil || strings.TrimSpace(bc.Server.Grpc.Addr) == "" {
		return ConfigError("platform_VERSION_GRPC_ADDR must not be empty")
	}
	if bc.Data == nil {
		return ConfigError("data config is required")
	}
	if bc.Data.Database == nil {
		return ConfigError("data.database config is required")
	}
	if strings.TrimSpace(bc.Data.Database.Driver) == "" {
		return ConfigError("platform_VERSION_DB_DRIVER must not be empty")
	}
	source := strings.TrimSpace(bc.Data.Database.Source)
	if source == "" {
		return ConfigError("platform_VERSION_DB_SOURCE must not be empty")
	}
	if bc.Data.Redis == nil || strings.TrimSpace(bc.Data.Redis.Addr) == "" {
		return ConfigError("platform_VERSION_REDIS_ADDR must not be empty")
	}
	if IsProduction() {
		if bc.Data.Database.Debug {
			return ConfigError("platform_VERSION_DB_DEBUG must be false in production")
		}
		if bc.Data.Database.Migrate {
			return ConfigError("platform_VERSION_DB_MIGRATE must be false in production; run migrations out of band")
		}
		if unsafeProductionDatabaseSource(source) {
			return ConfigError("platform_VERSION_DB_SOURCE must not use root credentials or placeholders in production")
		}
		if strings.TrimSpace(bc.Data.Redis.Password) == "" || strings.Contains(strings.ToLower(bc.Data.Redis.Password), "replace-me") {
			return ConfigError("platform_VERSION_REDIS_PASSWORD must not be empty or a placeholder in production")
		}
	}
	return nil
}

func unsafeProductionDatabaseSource(source string) bool {
	source = strings.ToLower(strings.TrimSpace(source))
	return strings.HasPrefix(source, "root:") ||
		strings.HasPrefix(source, "root@") ||
		strings.Contains(source, "replace-me")
}

func IsProduction() bool {
	for _, key := range []string{"platform_VERSION_ENV", "DEPLOY_ENV", "APP_ENV"} {
		switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
		case "prod", "production":
			return true
		}
	}
	return false
}

func envBool(key string) (bool, bool) {
	raw, ok := os.LookupEnv(key)
	if !ok {
		return false, false
	}
	v, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false, false
	}
	return v, true
}

type ConfigError string

func (e ConfigError) Error() string {
	return string(e)
}
