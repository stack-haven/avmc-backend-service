package runtimeconfig

import (
	"strings"
	"testing"

	"backend-service/app/version/service/internal/conf"
)

func TestValidateProductionRejectsDevelopmentDatabaseCredentials(t *testing.T) {
	t.Setenv("platform_VERSION_ENV", "production")
	bc := validBootstrap()

	err := Validate(bc)
	if err == nil || !strings.Contains(err.Error(), "root credentials") {
		t.Fatalf("Validate() error = %v, want root credentials error", err)
	}
}

func TestValidateProductionAcceptsSafeConfig(t *testing.T) {
	t.Setenv("platform_VERSION_ENV", "production")
	bc := validBootstrap()
	bc.Data.Database.Source = "service:secret@tcp(db:3306)/version?parseTime=True"
	bc.Data.Redis.Password = "redis-secret"

	if err := Validate(bc); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	t.Setenv("platform_VERSION_HTTP_ADDR", ":18000")
	t.Setenv("platform_VERSION_GRPC_ADDR", ":19000")
	t.Setenv("platform_VERSION_DB_SOURCE", "service:secret@tcp(db:3306)/version")
	t.Setenv("platform_VERSION_DB_DEBUG", "true")
	t.Setenv("platform_VERSION_REDIS_ADDR", "redis:6379")
	t.Setenv("platform_VERSION_REDIS_PASSWORD", "secret")
	bc := validBootstrap()

	ApplyEnvOverrides(bc)

	if bc.Server.Http.Addr != ":18000" || bc.Server.Grpc.Addr != ":19000" {
		t.Fatalf("server overrides not applied: %#v", bc.Server)
	}
	if bc.Data.Database.Source != "service:secret@tcp(db:3306)/version" || !bc.Data.Database.Debug {
		t.Fatalf("database overrides not applied: %#v", bc.Data.Database)
	}
	if bc.Data.Redis.Addr != "redis:6379" || bc.Data.Redis.Password != "secret" {
		t.Fatalf("redis overrides not applied: %#v", bc.Data.Redis)
	}
}

func TestValidateRequiredSections(t *testing.T) {
	tests := []struct {
		name string
		bc   *conf.Bootstrap
	}{
		{name: "nil", bc: nil},
		{name: "missing server", bc: &conf.Bootstrap{Data: validBootstrap().Data}},
		{name: "missing data", bc: &conf.Bootstrap{Server: validBootstrap().Server}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Validate(tt.bc); err == nil {
				t.Fatal("Validate() error = nil")
			}
		})
	}
}

func validBootstrap() *conf.Bootstrap {
	return &conf.Bootstrap{
		Server: &conf.Server{
			Http: &conf.Server_HTTP{Addr: ":8000"},
			Grpc: &conf.Server_GRPC{Addr: ":9000"},
		},
		Data: &conf.Data{
			Database: &conf.Data_Database{
				Driver: "mysql",
				Source: "root:root@tcp(127.0.0.1:3306)/test",
			},
			Redis: &conf.Data_Redis{Addr: "127.0.0.1:6379"},
		},
	}
}
