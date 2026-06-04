package runtimeconfig

import (
	"strings"
	"testing"

	"backend-service/app/avmc/admin/internal/conf"
)

func TestValidateRejectsUnsafeProductionDefaults(t *testing.T) {
	t.Setenv("AVMC_ADMIN_ENV", "production")
	bc := bootstrapConfig()

	err := Validate(bc)
	if err == nil {
		t.Fatal("expected production config validation to fail")
	}
	if !strings.Contains(err.Error(), "AVMC_ADMIN_JWT_KEY") {
		t.Fatalf("expected jwt key validation error, got %v", err)
	}
}

func TestValidateAcceptsSafeProductionConfig(t *testing.T) {
	t.Setenv("AVMC_ADMIN_ENV", "production")
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

func TestApplyEnvOverrides(t *testing.T) {
	t.Setenv("AVMC_ADMIN_HTTP_ADDR", "127.0.0.1:18000")
	t.Setenv("AVMC_ADMIN_GRPC_ADDR", "127.0.0.1:19000")
	t.Setenv("AVMC_ADMIN_JWT_KEY", "test-secret")
	t.Setenv("AVMC_ADMIN_CORS_ORIGINS", "https://admin.example.com, https://ops.example.com")
	t.Setenv("AVMC_ADMIN_DB_SOURCE", "user:pass@tcp(db:3306)/admin")
	t.Setenv("AVMC_ADMIN_DB_DEBUG", "false")
	t.Setenv("AVMC_ADMIN_REDIS_PASSWORD", "")

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
			want: "AVMC_ADMIN_DB_SOURCE",
		},
		{
			name: "missing redis addr",
			cfg: func() *conf.Bootstrap {
				bc := bootstrapConfig()
				bc.Data.Redis.Addr = ""
				return bc
			}(),
			want: "AVMC_ADMIN_REDIS_ADDR",
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

func bootstrapConfig() *conf.Bootstrap {
	return &conf.Bootstrap{
		Server: &conf.Server{
			Http: &conf.Server_HTTP{
				Addr:          "0.0.0.0:8000",
				EnableSwagger: true,
				Cors: &conf.Server_HTTP_CORS{
					Origins: []string{"*"},
				},
				Middleware: &conf.Middleware{
					Auth: &conf.Middleware_Auth{
						Key: "some_api_key",
					},
				},
			},
			Grpc: &conf.Server_GRPC{Addr: "0.0.0.0:9000"},
		},
		Data: &conf.Data{
			Database: &conf.Data_Database{
				Driver:  "mysql",
				Source:  "root:123456@tcp(127.0.0.1:3306)/avmc_system",
				Debug:   true,
				Migrate: true,
			},
			Redis: &conf.Data_Redis{Addr: "127.0.0.1:6379", Password: "123456"},
		},
	}
}
