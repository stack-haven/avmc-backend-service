package biz

import (
	"context"
	"testing"

	pb "backend-service/api/evie/service/v1"
)

// TestDictionaryUsecase_GetDashboardOverview_DefaultLimit 验证默认 limit=5。
func TestDictionaryUsecase_GetDashboardOverview_DefaultLimit(t *testing.T) {
	var gotLimit int32 = -1
	stub := &dictionaryRepoStub{
		dashboardFn: func(limit int32) (*pb.DashboardOverview, error) {
			gotLimit = limit
			return &pb.DashboardOverview{}, nil
		},
	}
	uc := NewDictionaryUsecase(stub, nil, nil)
	if _, err := uc.GetDashboardOverview(context.Background(), 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLimit != 5 {
		t.Errorf("default limit: got %d, want 5", gotLimit)
	}
}

// TestDictionaryUsecase_GetDashboardOverview_MaxLimit 验证 limit 上限 50。
func TestDictionaryUsecase_GetDashboardOverview_MaxLimit(t *testing.T) {
	var gotLimit int32 = -1
	stub := &dictionaryRepoStub{
		dashboardFn: func(limit int32) (*pb.DashboardOverview, error) {
			gotLimit = limit
			return &pb.DashboardOverview{}, nil
		},
	}
	uc := NewDictionaryUsecase(stub, nil, nil)
	if _, err := uc.GetDashboardOverview(context.Background(), 200); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotLimit != 50 {
		t.Errorf("max limit: got %d, want 50", gotLimit)
	}
}

// TestDictionaryUsecase_GetVocabularyHealth_DefaultDays 验证默认 recent_days=7。
func TestDictionaryUsecase_GetVocabularyHealth_DefaultDays(t *testing.T) {
	var gotDays int32 = -1
	stub := &dictionaryRepoStub{
		healthFn: func(_ string, days int32) ([]*pb.VocabularyHealthDetail, error) {
			gotDays = days
			return nil, nil
		},
	}
	uc := NewDictionaryUsecase(stub, nil, nil)
	if _, err := uc.GetVocabularyHealth(context.Background(), "", 0); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotDays != 7 {
		t.Errorf("default days: got %d, want 7", gotDays)
	}
}

// TestDictionaryUsecase_GetVocabularyHealth_MaxDays 验证 days 上限 90。
func TestDictionaryUsecase_GetVocabularyHealth_MaxDays(t *testing.T) {
	var gotDays int32 = -1
	stub := &dictionaryRepoStub{
		healthFn: func(_ string, days int32) ([]*pb.VocabularyHealthDetail, error) {
			gotDays = days
			return nil, nil
		},
	}
	uc := NewDictionaryUsecase(stub, nil, nil)
	if _, err := uc.GetVocabularyHealth(context.Background(), "ALL", 365); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotDays != 90 {
		t.Errorf("max days: got %d, want 90", gotDays)
	}
}

// TestDictionaryUsecase_GetDashboardOverview_Passthrough 验证 5/10/20 三档 limit 正常透传。
func TestDictionaryUsecase_GetDashboardOverview_Passthrough(t *testing.T) {
	cases := []int32{5, 10, 20}
	for _, want := range cases {
		var gotLimit int32 = -1
		stub := &dictionaryRepoStub{
			dashboardFn: func(limit int32) (*pb.DashboardOverview, error) {
				gotLimit = limit
				return &pb.DashboardOverview{
					Health: &pb.DashboardHealthSummary{TotalDictionaries: 5},
				}, nil
			},
		}
		uc := NewDictionaryUsecase(stub, nil, nil)
		overview, err := uc.GetDashboardOverview(context.Background(), want)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotLimit != want {
			t.Errorf("limit=%d=got %d, want %d", want, gotLimit, want)
		}
		if overview.Health.TotalDictionaries != 5 {
			t.Errorf("response not propagated: %+v", overview)
		}
	}
}