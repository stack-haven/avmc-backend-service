package runtimeconfig

import (
	"strings"

	"backend-service/pkg/conf"
	"backend-service/pkg/runtimeconfig"
)

// profile version 服务的环境变量约定。
//
// version 服务已冻结，只覆盖最少的运行时字段（监听地址 + 数据源），
// 不读取 Swagger / CORS / JWT 相关变量，因此 Load 中不调用
// runtimeconfig.ApplyServerEnvOverrides。这是刻意的：不要因为"顺便统一"
// 而让一个冻结服务开始响应它从未读取过的环境变量。
var profile = runtimeconfig.Profile{
	EnvPrefix:      "platform_VERSION",
	ProductionEnvs: []string{"platform_VERSION_ENV", "DEPLOY_ENV", "APP_ENV"},
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

// ApplyEnvOverrides 应用环境变量覆盖（仅监听地址与数据源）。
func ApplyEnvOverrides(bc *conf.Bootstrap) {
	runtimeconfig.ApplyEnvOverrides(bc, profile)
}

// IsProduction 判断当前是否生产环境。
func IsProduction() bool { return runtimeconfig.IsProduction(profile) }

// Validate version 服务的配置校验策略：只保证能启动，不做中间件加固校验。
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

// unsafeProductionDatabaseSource 是 version 服务特有的判定：
// 它把 "replace-me" 视为占位符，而 pkg/runtimeconfig 的通用实现只认 "replace-with"。
//
// 这里刻意保留本地实现以保持既有行为不变（version 已冻结）。
// 两种占位符写法本应统一，但那是行为收紧，需要单独确认后再改。
func unsafeProductionDatabaseSource(source string) bool {
	source = strings.ToLower(strings.TrimSpace(source))
	return strings.HasPrefix(source, "root:") ||
		strings.HasPrefix(source, "root@") ||
		strings.Contains(source, "replace-me")
}
