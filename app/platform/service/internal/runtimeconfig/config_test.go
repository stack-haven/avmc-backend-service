package runtimeconfig

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"backend-service/app/platform/service/internal/conf"

	"google.golang.org/protobuf/types/known/durationpb"
)

func TestValidateRejectsUnsafeProductionDefaults(t *testing.T) {
	t.Setenv("platform_ADMIN_ENV", "production")
	bc := bootstrapConfig()

	err := Validate(bc)
	if err == nil {
		t.Fatal("expected production config validation to fail")
	}
	if !strings.Contains(err.Error(), "platform_ADMIN_JWT_KEY") {
		t.Fatalf("expected jwt key validation error, got %v", err)
	}
}

func TestValidateAcceptsSafeProductionConfig(t *testing.T) {
	t.Setenv("platform_ADMIN_ENV", "production")
	bc := bootstrapConfig()
	bc.Server.Http.Middleware.Auth.Key = "0123456789abcdef0123456789abcdef"
	bc.Server.Http.EnableSwagger = false
	bc.Server.Http.Cors.Origins = []string{"https://admin.example.com"}
	bc.Data.Database.Source = "admin:secret@tcp(db:3306)/admin"
	bc.Data.Database.Debug = false
	bc.Data.Database.Migrate = false
	bc.Data.Redis.Password = "redis-secret"

	if err := Validate(bc); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestLoadRejectsDevelopmentConfigInProduction(t *testing.T) {
	t.Setenv("platform_ADMIN_ENV", "production")
	confDir := writeRuntimeConfig(t, developmentRuntimeConfigYAML())

	_, err := Load(confDir)
	if err == nil {
		t.Fatal("expected production load to reject development config")
	}
	if !strings.Contains(err.Error(), "platform_ADMIN_JWT_KEY") {
		t.Fatalf("Load() error = %v, want jwt production validation error", err)
	}
}

func TestLoadAcceptsProductionEnvOverrides(t *testing.T) {
	t.Setenv("platform_ADMIN_ENV", "production")
	t.Setenv("platform_ADMIN_JWT_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("platform_ADMIN_DB_SOURCE", "admin:secret@tcp(db:3306)/platform_system")
	t.Setenv("platform_ADMIN_DB_DEBUG", "false")
	t.Setenv("platform_ADMIN_DB_MIGRATE", "false")
	t.Setenv("platform_ADMIN_REDIS_ADDR", "redis:6379")
	t.Setenv("platform_ADMIN_REDIS_PASSWORD", "redis-secret")
	t.Setenv("platform_ADMIN_CORS_ORIGINS", "https://admin.example.com")
	t.Setenv("platform_ADMIN_ENABLE_SWAGGER", "false")
	confDir := writeRuntimeConfig(t, developmentRuntimeConfigYAML())

	bc, err := Load(confDir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if bc.Server.Http.EnableSwagger {
		t.Fatal("swagger should be disabled by production env override")
	}
	if got := bc.Server.Http.Cors.Origins; len(got) != 1 || got[0] != "https://admin.example.com" {
		t.Fatalf("cors origins = %#v", got)
	}
	if got := bc.Data.Database.Source; got != "admin:secret@tcp(db:3306)/platform_system" {
		t.Fatalf("db source = %q", got)
	}
	if bc.Data.Database.Debug || bc.Data.Database.Migrate {
		t.Fatalf("database debug=%v migrate=%v, want false", bc.Data.Database.Debug, bc.Data.Database.Migrate)
	}
	if got := bc.Data.Redis.Password; got != "redis-secret" {
		t.Fatalf("redis password = %q", got)
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	t.Setenv("platform_ADMIN_HTTP_ADDR", "127.0.0.1:18000")
	t.Setenv("platform_ADMIN_GRPC_ADDR", "127.0.0.1:19000")
	t.Setenv("platform_ADMIN_JWT_KEY", "test-secret")
	t.Setenv("platform_ADMIN_CORS_ORIGINS", "https://admin.example.com, https://ops.example.com")
	t.Setenv("platform_ADMIN_DB_SOURCE", "user:pass@tcp(db:3306)/admin")
	t.Setenv("platform_ADMIN_DB_DEBUG", "false")
	t.Setenv("platform_ADMIN_REDIS_PASSWORD", "")

	bc := bootstrapConfig()
	ApplyEnvOverrides(bc)

	if got := bc.Server.Http.Addr; got != "127.0.0.1:18000" {
		t.Fatalf("http addr = %q", got)
	}
	if got := bc.Server.Grpc.Addr; got != "127.0.0.1:19000" {
		t.Fatalf("grpc addr = %q", got)
	}
	if got := bc.Server.Http.Middleware.Auth.Key; got != "test-secret" {
		t.Fatalf("jwt key = %q", got)
	}
	if got := bc.Server.Http.Cors.Origins; len(got) != 2 || got[0] != "https://admin.example.com" || got[1] != "https://ops.example.com" {
		t.Fatalf("cors origins = %#v", got)
	}
	if got := bc.Data.Database.Source; got != "user:pass@tcp(db:3306)/admin" {
		t.Fatalf("db source = %q", got)
	}
	if bc.Data.Database.Debug {
		t.Fatal("db debug should be false after env override")
	}
	if got := bc.Data.Redis.Password; got != "" {
		t.Fatalf("redis password = %q", got)
	}
}

func writeRuntimeConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(content), 0o600); err != nil {
		t.Fatalf("write runtime config: %v", err)
	}
	return dir
}

func developmentRuntimeConfigYAML() string {
	return `
server:
  http:
    addr: 0.0.0.0:8000
    timeout: 1s
    enable_swagger: true
    enable_pprof: false
    cors:
      methods:
        - GET
      origins:
        - "*"
    middleware:
      limiter:
        name: bbr
      enable_recovery: true
      enable_validate: true
      auth:
        method: HS256
        key: dev-only-change-before-production-32-bytes
  grpc:
    addr: 0.0.0.0:9000
    timeout: 1s
    middleware:
      limiter:
        name: bbr
      enable_recovery: true
      enable_validate: true
data:
  database:
    driver: mysql
    source: root:123456@tcp(127.0.0.1:3306)/platform_system
    migrate: true
    debug: true
  redis:
    addr: 127.0.0.1:6379
    password: "123456"
    db: 0
`
}

func TestValidateRejectsMissingRequiredConfig(t *testing.T) {
	tests := []struct {
		name string
		cfg  *conf.Bootstrap
		want string
	}{
		{name: "nil bootstrap", cfg: nil, want: "bootstrap config is required"},
		{name: "missing server", cfg: &conf.Bootstrap{}, want: "server config is required"},
		{
			name: "missing cors",
			cfg: func() *conf.Bootstrap {
				bc := bootstrapConfig()
				bc.Server.Http.Cors = nil
				return bc
			}(),
			want: "server.http.cors",
		},
		{
			name: "missing database source",
			cfg: func() *conf.Bootstrap {
				bc := bootstrapConfig()
				bc.Data.Database.Source = ""
				return bc
			}(),
			want: "platform_ADMIN_DB_SOURCE",
		},
		{
			name: "missing redis addr",
			cfg: func() *conf.Bootstrap {
				bc := bootstrapConfig()
				bc.Data.Redis.Addr = ""
				return bc
			}(),
			want: "platform_ADMIN_REDIS_ADDR",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.cfg)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("expected %q validation error, got %v", tt.want, err)
			}
		})
	}
}

func TestValidateRejectsInvalidOperationalConfig(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*conf.Bootstrap)
		want   string
	}{
		{
			name: "unsupported jwt method",
			mutate: func(bc *conf.Bootstrap) {
				bc.Server.Http.Middleware.Auth.Method = "RS256"
			},
			want: "HS256, HS384, HS512",
		},
		{
			name: "unsupported http limiter",
			mutate: func(bc *conf.Bootstrap) {
				bc.Server.Http.Middleware.Limiter.Name = "token-bucket"
			},
			want: "server.http.middleware.limiter",
		},
		{
			name: "unsupported grpc limiter",
			mutate: func(bc *conf.Bootstrap) {
				bc.Server.Grpc.Middleware.Limiter.Name = "token-bucket"
			},
			want: "server.grpc.middleware.limiter",
		},
		{
			name: "invalid database pool",
			mutate: func(bc *conf.Bootstrap) {
				bc.Data.Database.MaxIdleConnections = 10
				bc.Data.Database.MaxOpenConnections = 5
			},
			want: "max_idle_connections",
		},
		{
			name: "negative redis db",
			mutate: func(bc *conf.Bootstrap) {
				bc.Data.Redis.Db = -1
			},
			want: "data.redis.db",
		},
		{
			name: "zero http timeout",
			mutate: func(bc *conf.Bootstrap) {
				bc.Server.Http.Timeout = durationpb.New(0)
			},
			want: "server.http.timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bc := bootstrapConfig()
			tt.mutate(bc)
			err := Validate(bc)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("Validate() error = %v, want error containing %q", err, tt.want)
			}
		})
	}
}

func TestValidateRejectsPprofInProduction(t *testing.T) {
	t.Setenv("platform_ADMIN_ENV", "production")
	bc := bootstrapConfig()
	bc.Server.Http.Middleware.Auth.Key = "0123456789abcdef0123456789abcdef"
	bc.Server.Http.EnableSwagger = false
	bc.Server.Http.EnablePprof = true
	bc.Server.Http.Cors.Origins = []string{"https://admin.example.com"}
	bc.Data.Database.Source = "admin:secret@tcp(db:3306)/admin"
	bc.Data.Database.Debug = false
	bc.Data.Database.Migrate = false
	bc.Data.Redis.Password = "redis-secret"

	err := Validate(bc)
	if err == nil || !strings.Contains(err.Error(), "enable_pprof") {
		t.Fatalf("Validate() error = %v, want pprof validation error", err)
	}
}

func bootstrapConfig() *conf.Bootstrap {
	return &conf.Bootstrap{
		Server: &conf.Server{
			Http: &conf.Server_HTTP{
				Addr:          "0.0.0.0:8000",
				Timeout:       durationpb.New(time.Second),
				EnableSwagger: true,
				Cors: &conf.Server_HTTP_CORS{
					Methods: []string{"GET", "POST"},
					Origins: []string{"*"},
				},
				Middleware: &conf.Middleware{
					EnableRecovery: true,
					EnableValidate: true,
					Limiter:        &conf.Middleware_RateLimiter{Name: "bbr"},
					Auth: &conf.Middleware_Auth{
						Method: "HS256",
						Key:    "some_api_key",
					},
				},
			},
			Grpc: &conf.Server_GRPC{
				Addr:    "0.0.0.0:9000",
				Timeout: durationpb.New(time.Second),
				Middleware: &conf.Middleware{
					EnableRecovery: true,
					EnableValidate: true,
					Limiter:        &conf.Middleware_RateLimiter{Name: "bbr"},
				},
			},
		},
		Data: &conf.Data{
			Database: &conf.Data_Database{
				Driver:  "mysql",
				Source:  "root:123456@tcp(127.0.0.1:3306)/platform_system",
				Debug:   true,
				Migrate: true,
			},
			Redis: &conf.Data_Redis{Addr: "127.0.0.1:6379", Password: "123456"},
		},
	}
}
