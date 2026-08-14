package biz

import (
	"context"
	"testing"

	"github.com/go-kratos/kratos/v2/log"

	pb "backend-service/api/evie/service/v1"
)

// providerRepoStub is a minimal ProviderRepo stub for usecase tests.
type providerRepoStub struct {
	upserted *pb.TenantProviderConfig
}

func (s *providerRepoStub) ListConfig(context.Context) ([]*pb.TenantProviderConfig, error) {
	return nil, nil
}
func (s *providerRepoStub) UpsertConfig(_ context.Context, c *pb.TenantProviderConfig) (*pb.TenantProviderConfig, error) {
	s.upserted = c
	return c, nil
}

func TestProviderUsecaseListAvailableProviders(t *testing.T) {
	uc := NewProviderUsecase(&providerRepoStub{}, log.NewStdLogger(nil))
	providers, err := uc.ListAvailableProviders(context.Background())
	if err != nil {
		t.Fatalf("ListAvailableProviders error: %v", err)
	}
	if len(providers) != 4 {
		t.Fatalf("expected 4 available providers, got %d", len(providers))
	}
}

func TestProviderUsecaseUpdateTenantConfigDefaults(t *testing.T) {
	stub := &providerRepoStub{}
	uc := NewProviderUsecase(stub, log.NewStdLogger(nil))
	updated, err := uc.UpdateTenantConfig(context.Background(), &pb.TenantProviderConfig{ProviderName: "funasr"})
	if err != nil {
		t.Fatalf("UpdateTenantConfig error: %v", err)
	}
	if updated.GetSampleRate() != 16000 {
		t.Errorf("expected default sample_rate 16000, got %d", updated.GetSampleRate())
	}
	if updated.GetLanguage() != "zh" {
		t.Errorf("expected default language 'zh', got %q", updated.GetLanguage())
	}
}

func TestProviderUsecaseUpdateTenantConfigRequiresName(t *testing.T) {
	uc := NewProviderUsecase(&providerRepoStub{}, log.NewStdLogger(nil))
	if _, err := uc.UpdateTenantConfig(context.Background(), &pb.TenantProviderConfig{}); err == nil {
		t.Fatal("expected error for empty provider name")
	}
}
