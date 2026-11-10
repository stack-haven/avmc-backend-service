package runtimeconfig

import (
	"errors"
	"os"
	"strconv"
	"strings"

	"backend-service/app/edai/service/internal/conf"

	"github.com/go-kratos/kratos/v2/config"
	"github.com/go-kratos/kratos/v2/config/file"
)

// ConfigError 表示配置缺失或不合法。
func ConfigError(msg string) error { return errors.New("config: " + msg) }

// Load 读取配置文件、应用环境变量覆盖并校验。
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
	if err := ApplyEnvOverrides(&bc); err != nil {
		return nil, err
	}
	if err := Validate(&bc); err != nil {
		return nil, err
	}
	return &bc, nil
}

// 环境变量前缀：EDAI_*
const envPrefix = "EDAI_"

// ApplyEnvOverrides 用环境变量覆盖关键配置。
//
// 覆盖范围刻意最小：只覆盖监听地址、数据库、JWT 密钥与数据目录，
// 其余走配置文件，避免配置来源过度分散。
func ApplyEnvOverrides(bc *conf.Bootstrap) error {
	if bc == nil {
		return nil
	}
	if v := strings.TrimSpace(os.Getenv(envPrefix + "HTTP_ADDR")); v != "" {
		if bc.Server == nil {
			bc.Server = &conf.Server{}
		}
		if bc.Server.Http == nil {
			bc.Server.Http = &conf.Server_HTTP{}
		}
		bc.Server.Http.Addr = v
	}
	if v := strings.TrimSpace(os.Getenv(envPrefix + "DB_DRIVER")); v != "" {
		if bc.Data == nil {
			bc.Data = &conf.Data{}
		}
		if bc.Data.Database == nil {
			bc.Data.Database = &conf.Data_Database{}
		}
		bc.Data.Database.Driver = v
	}
	if v := strings.TrimSpace(os.Getenv(envPrefix + "DB_SOURCE")); v != "" {
		if bc.Data == nil {
			bc.Data = &conf.Data{}
		}
		if bc.Data.Database == nil {
			bc.Data.Database = &conf.Data_Database{}
		}
		bc.Data.Database.Source = v
	}
	return nil
}

// Validate 校验启动必需项。
func Validate(bc *conf.Bootstrap) error {
	if bc == nil {
		return ConfigError("bootstrap is required")
	}
	if bc.Server == nil || bc.Server.Http == nil {
		return ConfigError("server.http is required")
	}
	if strings.TrimSpace(bc.Server.Http.Addr) == "" {
		return ConfigError("server.http.addr must not be empty")
	}
	if bc.Data == nil || bc.Data.Database == nil {
		return ConfigError("data.database is required")
	}
	if strings.TrimSpace(bc.Data.Database.Driver) == "" {
		return ConfigError("data.database.driver must not be empty")
	}
	if strings.TrimSpace(bc.Data.Database.Source) == "" {
		return ConfigError("data.database.source must not be empty")
	}
	return nil
}

// AuthConfig 从 server.http.middleware.auth 提取认证配置。
func AuthConfig(bc *conf.Bootstrap) (key, method string, expiresSeconds int64) {
	if bc == nil || bc.Server == nil || bc.Server.Http == nil || bc.Server.Http.Middleware == nil {
		return "", "", 0
	}
	auth := bc.Server.Http.Middleware.Auth
	if auth == nil {
		return "", "", 0
	}
	expires := int64(0)
	if auth.ExpiresTime != nil {
		expires = int64(auth.ExpiresTime.AsDuration().Seconds())
	}
	return auth.Key, auth.Method, expires
}

// DataDir 返回 EDAI 数据目录（预览、派生文件、临时文件）。
func DataDir() string {
	if v := strings.TrimSpace(os.Getenv(envPrefix + "DATA_DIR")); v != "" {
		return v
	}
	return "./data"
}

// RunMode 返回运行模式：local_dashboard（默认）或 backend_service。
func RunMode() string {
	if v := strings.TrimSpace(os.Getenv(envPrefix + "RUN_MODE")); v != "" {
		return v
	}
	return "local_dashboard"
}

// Bool 读取布尔型环境变量。
func Bool(name string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}
