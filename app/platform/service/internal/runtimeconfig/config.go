package runtimeconfig

import (
	"strings"

	"backend-service/pkg/conf"
	"backend-service/pkg/runtimeconfig"
)

// profile 平台服务的环境变量约定。
//
// JWT 密钥使用全仓默认的 ARK_JWT_KEY（不带服务前缀），这是既有的部署约定。
var profile = runtimeconfig.Profile{
	EnvPrefix:      "platform_ADMIN",
	ProductionEnvs: []string{"platform_ADMIN_ENV", "DEPLOY_ENV", "APP_ENV"},
}

// ConfigError 表示配置缺失或不合法（实现见 pkg/runtimeconfig）。
type ConfigError = runtimeconfig.ConfigError

// Load 读取配置文件、应用环境变量覆盖并校验。
//
// 装载机制在 pkg/runtimeconfig；本函数只负责把「本服务的校验策略」接上。
func Load(path string) (*conf.Bootstrap, error) {
	bc, err := runtimeconfig.Load(path)
	if err != nil {
		return nil, err
	}
	ApplyEnvOverrides(bc)
	if err := Validate(bc); err != nil {
		return nil, err
	}
	return bc, nil
}

// ApplyEnvOverrides 应用环境变量覆盖。
func ApplyEnvOverrides(bc *conf.Bootstrap) {
	runtimeconfig.ApplyEnvOverrides(bc, profile)
	runtimeconfig.ApplyServerEnvOverrides(bc, profile)
}

// IsProduction 判断当前是否生产环境。
func IsProduction() bool { return runtimeconfig.IsProduction(profile) }

// Validate 平台服务的配置校验策略。
//
// 平台是底座服务：对中间件、限流器、超时、CORS 的加固要求最严，
// 因此这里的策略比 ai/version 更严格。策略属于本服务，不下沉到 pkg。
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
	if err := runtimeconfig.PositiveDuration("server.http.timeout", bc.Server.Http.Timeout); err != nil {
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
	if err := runtimeconfig.PositiveDuration("server.grpc.timeout", bc.Server.Grpc.Timeout); err != nil {
		return err
	}
	if bc.Server.Http.Middleware == nil || bc.Server.Http.Middleware.Auth == nil {
		return ConfigError("server.http.middleware.auth config is required")
	}
	if err := runtimeconfig.Limiter("server.http.middleware.limiter", bc.Server.Http.Middleware.Limiter); err != nil {
		return err
	}
	if !bc.Server.Http.Middleware.EnableRecovery || !bc.Server.Http.Middleware.EnableValidate {
		return ConfigError("server.http middleware recovery and validation must be enabled")
	}
	if bc.Server.Grpc.Middleware != nil {
		if err := runtimeconfig.Limiter("server.grpc.middleware.limiter", bc.Server.Grpc.Middleware.Limiter); err != nil {
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
		return ConfigError("ARK_JWT_KEY must not be empty")
	}
	if IsProduction() && key == "some_api_key" {
		return ConfigError("ARK_JWT_KEY must override the development default in production")
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
			return ConfigError("ARK_JWT_KEY must be at least 32 bytes and must not be a placeholder in production")
		}
		if runtimeconfig.UnsafeProductionDatabaseSource(bc.Data.Database.Source) {
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
