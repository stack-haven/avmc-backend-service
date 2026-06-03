package runtimeconfig

import (
	"os"
	"strconv"
	"strings"

	"backend-service/app/avmc/ai/internal/conf"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
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
	if bc.Server.Sse == nil {
		return ConfigError("server.sse config is required")
	}
	if strings.TrimSpace(bc.Server.Sse.Addr) == "" {
		return ConfigError("AVMC_AI_SSE_ADDR must not be empty")
	}
	if strings.TrimSpace(bc.Server.Sse.Path) == "" {
		return ConfigError("AVMC_AI_SSE_PATH must not be empty")
	}
	if bc.Server.Http == nil {
		return ConfigError("server.http config is required")
	}
	if strings.TrimSpace(bc.Server.Http.Addr) == "" {
		return ConfigError("AVMC_AI_HTTP_ADDR must not be empty")
	}
	if bc.Server.Grpc == nil {
		return ConfigError("server.grpc config is required")
	}
	if strings.TrimSpace(bc.Server.Grpc.Addr) == "" {
		return ConfigError("AVMC_AI_GRPC_ADDR must not be empty")
	}
	if bc.Server.Http.Middleware == nil || bc.Server.Http.Middleware.Auth == nil {
		return ConfigError("server.http.middleware.auth config is required")
	}
	key := strings.TrimSpace(bc.Server.Http.Middleware.Auth.Key)
	if key == "" {
		return ConfigError("AVMC_AI_JWT_KEY must not be empty")
	}
	if IsProduction() && key == "some_api_key" {
		return ConfigError("AVMC_AI_JWT_KEY must override the development default in production")
	}
	if bc.Data == nil {
		return ConfigError("data config is required")
	}
	if bc.Data.Database == nil {
		return ConfigError("data.database config is required")
	}
	if strings.TrimSpace(bc.Data.Database.Driver) == "" {
		return ConfigError("AVMC_AI_DB_DRIVER must not be empty")
	}
	if strings.TrimSpace(bc.Data.Database.Source) == "" {
		return ConfigError("AVMC_AI_DB_SOURCE must not be empty")
	}
	if bc.Data.Redis == nil {
		return ConfigError("data.redis config is required")
	}
	if strings.TrimSpace(bc.Data.Redis.Addr) == "" {
		return ConfigError("AVMC_AI_REDIS_ADDR must not be empty")
	}
	if IsProduction() {
		if bc.Server.Http.EnableSwagger {
			return ConfigError("AVMC_AI_ENABLE_SWAGGER must be false in production")
		}
		if bc.Server.Http.Cors != nil {
			for _, origin := range bc.Server.Http.Cors.Origins {
				if strings.TrimSpace(origin) == "*" {
					return ConfigError("AVMC_AI_CORS_ORIGINS must not include * in production")
				}
			}
		}
		if bc.Data.Database.Debug {
			return ConfigError("AVMC_AI_DB_DEBUG must be false in production")
		}
		if bc.Data.Database.Migrate {
			return ConfigError("AVMC_AI_DB_MIGRATE must be false in production; run migrations out of band")
		}
	}
	return nil
}

func IsProduction() bool {
	for _, key := range []string{"AVMC_AI_ENV", "AVMC_ADMIN_ENV", "DEPLOY_ENV", "APP_ENV"} {
		switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
		case "prod", "production":
			return true
		}
	}
	return false
}

func applyServerEnv(server *conf.Server) {
	if server.Sse != nil {
		if v := strings.TrimSpace(os.Getenv("AVMC_AI_SSE_ADDR")); v != "" {
			server.Sse.Addr = v
		}
		if v := strings.TrimSpace(os.Getenv("AVMC_AI_SSE_PATH")); v != "" {
			server.Sse.Path = v
		}
	}
	if server.Http != nil {
		if v := strings.TrimSpace(os.Getenv("AVMC_AI_HTTP_ADDR")); v != "" {
			server.Http.Addr = v
		}
		if v, ok := envBool("AVMC_AI_ENABLE_SWAGGER"); ok {
			server.Http.EnableSwagger = v
		}
		if v := splitCSV(os.Getenv("AVMC_AI_CORS_ORIGINS")); len(v) > 0 {
			if server.Http.Cors == nil {
				server.Http.Cors = &conf.Server_HTTP_CORS{}
			}
			server.Http.Cors.Origins = v
		}
		if server.Http.Middleware != nil && server.Http.Middleware.Auth != nil {
			if v := strings.TrimSpace(os.Getenv("AVMC_AI_JWT_KEY")); v != "" {
				server.Http.Middleware.Auth.Key = v
			}
		}
	}
	if server.Grpc != nil {
		if v := strings.TrimSpace(os.Getenv("AVMC_AI_GRPC_ADDR")); v != "" {
			server.Grpc.Addr = v
		}
	}
}

func applyDataEnv(data *conf.Data) {
	if data.Database != nil {
		if v := strings.TrimSpace(os.Getenv("AVMC_AI_DB_DRIVER")); v != "" {
			data.Database.Driver = v
		}
		if v := strings.TrimSpace(os.Getenv("AVMC_AI_DB_SOURCE")); v != "" {
			data.Database.Source = v
		}
		if v, ok := envBool("AVMC_AI_DB_MIGRATE"); ok {
			data.Database.Migrate = v
		}
		if v, ok := envBool("AVMC_AI_DB_DEBUG"); ok {
			data.Database.Debug = v
		}
	}
	if data.Redis != nil {
		if v := strings.TrimSpace(os.Getenv("AVMC_AI_REDIS_ADDR")); v != "" {
			data.Redis.Addr = v
		}
		if v, ok := os.LookupEnv("AVMC_AI_REDIS_PASSWORD"); ok {
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
