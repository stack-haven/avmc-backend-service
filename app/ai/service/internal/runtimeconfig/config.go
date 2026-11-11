package runtimeconfig

import (
	"strings"

	"backend-service/pkg/conf"
	"backend-service/pkg/runtimeconfig"
)

// profile AI 服务的环境变量约定。
//
// 注意两点与平台不同，且都是既有部署约定，不做统一：
//   - 前缀是 platform_AI（不是 platform_ADMIN）；
//   - JWT 密钥走独立的 platform_AI_JWT_KEY，而不是 ARK_JWT_KEY。
//
// ProductionEnvs 同时保留 platform_AI_ENV 与 platform_ADMIN_ENV：
// 历史上 AI 服务曾被当作平台的一部分部署，两种开关都可能存在。
var profile = runtimeconfig.Profile{
	EnvPrefix:      "platform_AI",
	JWTKeyEnv:      "platform_AI_JWT_KEY",
	ProductionEnvs: []string{"platform_AI_ENV", "platform_ADMIN_ENV", "DEPLOY_ENV", "APP_ENV"},
}

// ConfigError 表示配置缺失或不合法（实现见 pkg/runtimeconfig）。
type ConfigError = runtimeconfig.ConfigError

// Load 读取配置文件、应用环境变量覆盖并校验。
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

// ApplyEnvOverrides 应用环境变量覆盖：监听地址/数据源 + 中间件 + SSE。
func ApplyEnvOverrides(bc *conf.Bootstrap) {
	runtimeconfig.ApplyEnvOverrides(bc, profile)
	runtimeconfig.ApplyServerEnvOverrides(bc, profile)
	runtimeconfig.ApplySSEEnvOverrides(bc, profile)
}

// IsProduction 判断当前是否生产环境。
func IsProduction() bool { return runtimeconfig.IsProduction(profile) }

// Validate AI 服务的配置校验策略。
//
// 与平台的区别：额外要求 SSE 配置；不强制 HTTP 中间件的 recovery/validate 开关，
// 也不校验超时与限流器（AI 服务的 HTTP 面较薄，这些约束由平台网关承担）。
func Validate(bc *conf.Bootstrap) error {
	if bc == nil {
		return ConfigError("bootstrap config is required")
	}
	if bc.Server == nil {
		return ConfigError("server config is required")
	}
	if bc.Server.Sse == nil {
		return ConfigError("server.sse config is required")
	}
	if strings.TrimSpace(bc.Server.Sse.Addr) == "" {
		return ConfigError("platform_AI_SSE_ADDR must not be empty")
	}
	if strings.TrimSpace(bc.Server.Sse.Path) == "" {
		return ConfigError("platform_AI_SSE_PATH must not be empty")
	}
	if bc.Server.Http == nil {
		return ConfigError("server.http config is required")
	}
	if strings.TrimSpace(bc.Server.Http.Addr) == "" {
		return ConfigError("platform_AI_HTTP_ADDR must not be empty")
	}
	if bc.Server.Http.Cors == nil {
		return ConfigError("server.http.cors config is required")
	}
	if bc.Server.Grpc == nil {
		return ConfigError("server.grpc config is required")
	}
	if strings.TrimSpace(bc.Server.Grpc.Addr) == "" {
		return ConfigError("platform_AI_GRPC_ADDR must not be empty")
	}
	if bc.Server.Http.Middleware == nil || bc.Server.Http.Middleware.Auth == nil {
		return ConfigError("server.http.middleware.auth config is required")
	}
	key := strings.TrimSpace(bc.Server.Http.Middleware.Auth.Key)
	if key == "" {
		return ConfigError("platform_AI_JWT_KEY must not be empty")
	}
	if IsProduction() && key == "some_api_key" {
		return ConfigError("platform_AI_JWT_KEY must override the development default in production")
	}
	if bc.Data == nil {
		return ConfigError("data config is required")
	}
	if bc.Data.Database == nil {
		return ConfigError("data.database config is required")
	}
	if strings.TrimSpace(bc.Data.Database.Driver) == "" {
		return ConfigError("platform_AI_DB_DRIVER must not be empty")
	}
	if strings.TrimSpace(bc.Data.Database.Source) == "" {
		return ConfigError("platform_AI_DB_SOURCE must not be empty")
	}
	if bc.Data.Redis == nil {
		return ConfigError("data.redis config is required")
	}
	if strings.TrimSpace(bc.Data.Redis.Addr) == "" {
		return ConfigError("platform_AI_REDIS_ADDR must not be empty")
	}
	if IsProduction() {
		if len(key) < 32 || strings.Contains(strings.ToLower(key), "replace-with") {
			return ConfigError("platform_AI_JWT_KEY must be at least 32 bytes and must not be a placeholder in production")
		}
		if runtimeconfig.UnsafeProductionDatabaseSource(bc.Data.Database.Source) {
			return ConfigError("platform_AI_DB_SOURCE must not use root credentials or placeholders in production")
		}
		if strings.TrimSpace(bc.Data.Redis.Password) == "" || strings.Contains(strings.ToLower(bc.Data.Redis.Password), "replace-with") {
			return ConfigError("platform_AI_REDIS_PASSWORD must not be empty or a placeholder in production")
		}
		if len(bc.Server.Http.Cors.Origins) == 0 {
			return ConfigError("platform_AI_CORS_ORIGINS must not be empty in production")
		}
		if bc.Server.Http.EnableSwagger {
			return ConfigError("platform_AI_ENABLE_SWAGGER must be false in production")
		}
		for _, origin := range bc.Server.Http.Cors.Origins {
			if strings.TrimSpace(origin) == "*" {
				return ConfigError("platform_AI_CORS_ORIGINS must not include * in production")
			}
		}
		if bc.Data.Database.Debug {
			return ConfigError("platform_AI_DB_DEBUG must be false in production")
		}
		if bc.Data.Database.Migrate {
			return ConfigError("platform_AI_DB_MIGRATE must be false in production; run migrations out of band")
		}
	}
	return nil
}
