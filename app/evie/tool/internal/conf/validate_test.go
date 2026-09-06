package conf

import (
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/durationpb"
)

func newValid() *Bootstrap {
	return &Bootstrap{
		Server: &Server{
			Http: &Server_HTTP{Addr: ":8110"},
			Grpc: &Server_GRPC{Addr: ":9110"},
		},
		Data: &Data{
			Redis: &Data_Redis{
				Addr:           "127.0.0.1:6379",
				TokenKeyPrefix: "oauth2_access_token:",
			},
		},
		Qua: &Qua{
			BaseUrl: "http://qua.local",
			Endpoints: &Qua_Endpoints{
				ListUsers: "/users",
				ListDepts: "/depts",
			},
		},
		Asr: &Asr{
			Providers: &Asr_Providers{
				Funasr: &Asr_Provider{Enabled: true},
			},
		},
		Enhancement: &Enhancement{
			Pipeline: []string{"cleaning", "filler"},
		},
		SystemDict:     &SystemDict{Path: "./system.json"},
		TenantRegistry: &TenantRegistry{Path: "./tenants.json"},
	}
}

func TestValidate_Valid(t *testing.T) {
	if err := Validate(newValid()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_MissingServer(t *testing.T) {
	c := newValid()
	c.Server = nil
	err := Validate(c)
	if err == nil || !strings.Contains(err.Error(), "server.http.addr") {
		t.Fatalf("expected http addr error, got %v", err)
	}
}

func TestValidate_MissingRedisTokenPrefix(t *testing.T) {
	c := newValid()
	c.Data.Redis.TokenKeyPrefix = ""
	err := Validate(c)
	if err == nil || !strings.Contains(err.Error(), "token_key_prefix") {
		t.Fatalf("expected token_key_prefix error, got %v", err)
	}
}

func TestValidate_NoASRProvider(t *testing.T) {
	c := newValid()
	c.Asr.Providers = &Asr_Providers{}
	err := Validate(c)
	if err == nil || !strings.Contains(err.Error(), "asr") {
		t.Fatalf("expected asr error, got %v", err)
	}
}

func TestValidate_UnknownPipelineStep(t *testing.T) {
	c := newValid()
	c.Enhancement.Pipeline = []string{"cleaning", "mystery"}
	err := Validate(c)
	if err == nil || !strings.Contains(err.Error(), "mystery") {
		t.Fatalf("expected pipeline step error, got %v", err)
	}
}

// 编译期：导入使用，避免 unused
var _ = durationpb.New
