package runtimeconfig

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"backend-service/app/evie/service/internal/conf"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
	"google.golang.org/protobuf/types/known/durationpb"
)

func Load(path string) (*conf.Bootstrap, error) {
	c := config.New(
		config.WithSource(
			file.NewSource(path),
		),
	)
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
		applyServerEnv(bc.Server)
	}
	if bc.Data != nil {
		applyDataEnv(bc.Data)
	}
}

func Validate(bc *conf.Bootstrap) error {
	if bc == nil {
		return ConfigError("bootstrap config is required")
	}
	if bc.Server == nil {
		return ConfigError("server config is required")
	}
	if bc.Server.Http == nil {
		return ConfigError("server.http config is required")
	}
	if strings.TrimSpace(bc.Server.Http.Addr) == "" {
		return ConfigError("platform_ADMIN_HTTP_ADDR must not be empty")
	}
	if err := validatePositiveDuration("server.http.timeout", bc.Server.Http.Timeout); err != nil {
		return err
	}
	if bc.Server.Http.Cors == nil {
		return ConfigError("server.http.cors config is required")
	}
	if len(bc.Server.Http.Cors.Methods) == 0 {
		return ConfigError("server.http.cors.methods must not be empty")
	}
	if len(bc.Server.Http.Cors.Origins) == 0 {
		return ConfigError("server.http.cors.origins must not be empty")
	}
	if bc.Server.Grpc == nil {
		return ConfigError("server.grpc config is required")
	}
	if strings.TrimSpace(bc.Server.Grpc.Addr) == "" {
		return ConfigError("platform_ADMIN_GRPC_ADDR must not be empty")
	}
	if err := validatePositiveDuration("server.grpc.timeout", bc.Server.Grpc.Timeout); err != nil {
		return err
	}
	if bc.Server.Http.Middleware == nil || bc.Server.Http.Middleware.Auth == nil {
		return ConfigError("server.http.middleware.auth config is required")
	}
	if err := validateLimiter("server.http.middleware.limiter", bc.Server.Http.Middleware.Limiter); err != nil {
		return err
	}
	if !bc.Server.Http.Middleware.EnableRecovery || !bc.Server.Http.Middleware.EnableValidate {
		return ConfigError("server.http middleware recovery and validation must be enabled")
	}
	if bc.Server.Grpc.Middleware != nil {
		if err := validateLimiter("server.grpc.middleware.limiter", bc.Server.Grpc.Middleware.Limiter); err != nil {
			return err
		}
		if !bc.Server.Grpc.Middleware.EnableRecovery || !bc.Server.Grpc.Middleware.EnableValidate {
			return ConfigError("server.grpc middleware recovery and validation must be enabled")
		}
	} else {
		return ConfigError("server.grpc.middleware config is required")
	}
	method := strings.ToUpper(strings.TrimSpace(bc.Server.Http.Middleware.Auth.Method))
	switch method {
	case "HS256", "HS384", "HS512":
	default:
		return ConfigError("server.http.middleware.auth.method must be one of HS256, HS384, HS512")
	}
	key := strings.TrimSpace(bc.Server.Http.Middleware.Auth.Key)
	if key == "" {
		return ConfigError("platform_ADMIN_JWT_KEY must not be empty")
	}
	if IsProduction() && key == "some_api_key" {
		return ConfigError("platform_ADMIN_JWT_KEY must override the development default in production")
	}
	if bc.Data == nil {
		return ConfigError("data config is required")
	}
	if bc.Data.Database == nil {
		return ConfigError("data.database config is required")
	}
	if strings.TrimSpace(bc.Data.Database.Driver) == "" {
		return ConfigError("platform_ADMIN_DB_DRIVER must not be empty")
	}
	if strings.TrimSpace(bc.Data.Database.Source) == "" {
		return ConfigError("platform_ADMIN_DB_SOURCE must not be empty")
	}
	if bc.Data.Database.MaxIdleConnections < 0 || bc.Data.Database.MaxOpenConnections < 0 {
		return ConfigError("database connection pool sizes must not be negative")
	}
	if bc.Data.Database.MaxOpenConnections > 0 && bc.Data.Database.MaxIdleConnections > bc.Data.Database.MaxOpenConnections {
		return ConfigError("data.database.max_idle_connections must not exceed max_open_connections")
	}
	if bc.Data.Redis == nil {
		return ConfigError("data.redis config is required")
	}
	if strings.TrimSpace(bc.Data.Redis.Addr) == "" {
		return ConfigError("platform_ADMIN_REDIS_ADDR must not be empty")
	}
	if bc.Data.Redis.Db < 0 {
		return ConfigError("data.redis.db must not be negative")
	}
	if IsProduction() {
		lowerKey := strings.ToLower(key)
		if len(key) < 32 || strings.Contains(lowerKey, "replace-with") || strings.Contains(lowerKey, "dev-only") {
			return ConfigError("platform_ADMIN_JWT_KEY must be at least 32 bytes and must not be a placeholder in production")
		}
		if unsafeProductionDatabaseSource(bc.Data.Database.Source) {
			return ConfigError("platform_ADMIN_DB_SOURCE must not use root credentials or placeholders in production")
		}
		if strings.TrimSpace(bc.Data.Redis.Password) == "" || strings.Contains(strings.ToLower(bc.Data.Redis.Password), "replace-with") {
			return ConfigError("platform_ADMIN_REDIS_PASSWORD must not be empty or a placeholder in production")
		}
		if len(bc.Server.Http.Cors.Origins) == 0 {
			return ConfigError("platform_ADMIN_CORS_ORIGINS must not be empty in production")
		}
		if bc.Server.Http.EnableSwagger {
			return ConfigError("platform_ADMIN_ENABLE_SWAGGER must be false in production")
		}
		if bc.Server.Http.EnablePprof {
			return ConfigError("server.http.enable_pprof must be false in production")
		}
		for _, origin := range bc.Server.Http.Cors.Origins {
			if strings.TrimSpace(origin) == "*" {
				return ConfigError("platform_ADMIN_CORS_ORIGINS must not include * in production")
			}
		}
		if bc.Data.Database.Debug {
			return ConfigError("platform_ADMIN_DB_DEBUG must be false in production")
		}
		if bc.Data.Database.Migrate {
			return ConfigError("platform_ADMIN_DB_MIGRATE must be false in production; run migrations out of band")
		}
	}
	return nil
}

func validatePositiveDuration(name string, value *durationpb.Duration) error {
	if value == nil || value.AsDuration() <= 0 {
		return ConfigError(fmt.Sprintf("%s must be greater than zero", name))
	}
	return nil
}

func validateLimiter(name string, limiter *conf.Middleware_RateLimiter) error {
	if limiter == nil {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(limiter.Name)) {
	case "", "off", "none", "disabled", "bbr":
		return nil
	default:
		return ConfigError(fmt.Sprintf("%s supports only bbr or disabled", name))
	}
}

func unsafeProductionDatabaseSource(source string) bool {
	source = strings.ToLower(strings.TrimSpace(source))
	return strings.HasPrefix(source, "root:") ||
		strings.HasPrefix(source, "root@") ||
		strings.Contains(source, "replace-with")
}

func IsProduction() bool {
	for _, key := range []string{"platform_ADMIN_ENV", "DEPLOY_ENV", "APP_ENV"} {
		switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
		case "prod", "production":
			return true
		}
	}
	return false
}

func applyServerEnv(server *conf.Server) {
	if server.Http != nil {
		if v := strings.TrimSpace(os.Getenv("platform_ADMIN_HTTP_ADDR")); v != "" {
			server.Http.Addr = v
		}
		if v, ok := envBool("platform_ADMIN_ENABLE_SWAGGER"); ok {
			server.Http.EnableSwagger = v
		}
		if v := splitCSV(os.Getenv("platform_ADMIN_CORS_ORIGINS")); len(v) > 0 {
			if server.Http.Cors == nil {
				server.Http.Cors = &conf.Server_HTTP_CORS{}
			}
			server.Http.Cors.Origins = v
		}
		if server.Http.Middleware != nil && server.Http.Middleware.Auth != nil {
			if v := strings.TrimSpace(os.Getenv("platform_ADMIN_JWT_KEY")); v != "" {
				server.Http.Middleware.Auth.Key = v
			}
		}
	}
	if server.Grpc != nil {
		if v := strings.TrimSpace(os.Getenv("platform_ADMIN_GRPC_ADDR")); v != "" {
			server.Grpc.Addr = v
		}
	}
}

func applyDataEnv(data *conf.Data) {
	if data.Database != nil {
		if v := strings.TrimSpace(os.Getenv("platform_ADMIN_DB_DRIVER")); v != "" {
			data.Database.Driver = v
		}
		if v := strings.TrimSpace(os.Getenv("platform_ADMIN_DB_SOURCE")); v != "" {
			data.Database.Source = v
		}
		if v, ok := envBool("platform_ADMIN_DB_MIGRATE"); ok {
			data.Database.Migrate = v
		}
		if v, ok := envBool("platform_ADMIN_DB_DEBUG"); ok {
			data.Database.Debug = v
		}
	}
	if data.Redis != nil {
		if v := strings.TrimSpace(os.Getenv("platform_ADMIN_REDIS_ADDR")); v != "" {
			data.Redis.Addr = v
		}
		if v, ok := os.LookupEnv("platform_ADMIN_REDIS_PASSWORD"); ok {
			data.Redis.Password = v
		}
	}
}

func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if v := strings.TrimSpace(part); v != "" {
			values = append(values, v)
		}
	}
	return values
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
