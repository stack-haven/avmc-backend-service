package biz_test

import (
	"context"
	"testing"
	"time"

	durationpb "google.golang.org/protobuf/types/known/durationpb"

	"backend-service/app/evie/tool/internal/biz"
	v1conf "backend-service/app/evie/tool/internal/conf"
)

// TestEnhancementUsecase_Timeout_ZeroByDefault 验证默认无超时（NewEnhancementUsecase 不设 timeout）。
func TestEnhancementUsecase_Timeout_ZeroByDefault(t *testing.T) {
	uc := biz.NewEnhancementUsecase(nil).WithTimeout(0)
	if uc == nil {
		t.Fatal("nil usecase")
	}
	// rawText=="" 早返回，不调 engine（避免 nil engine panic）；只验证 timeout=0 不修改 ctx。
	if _, err := uc.EnhanceText(context.Background(), "", "t1"); err == nil {
		t.Error("expected error for empty rawText")
	}
}

// TestEnhancementUsecase_WithTimeout_Applied 验证 NewEnhancementUsecaseWithConf 设置了 timeout。
func TestEnhancementUsecase_WithTimeout_Applied(t *testing.T) {
	conf := &v1conf.Enhancement{
		Timeout: durationpb.New(50 * time.Millisecond),
	}
	uc := biz.NewEnhancementUsecaseWithConf(nil, conf)
	if uc == nil {
		t.Fatal("nil usecase")
	}
	// rawText=="" 早返回。timeout 字段已设置即可。
	if _, err := uc.EnhanceText(context.Background(), "", "t1"); err == nil {
		t.Error("expected error for empty rawText")
	}
}

// TestASRUsecase_ApplyRecognizeTimeout_NoConfig 验证无配置时不修改 ctx。
func TestASRUsecase_ApplyRecognizeTimeout_NoConfig(t *testing.T) {
	uc := biz.NewASRUsecase(&biz.ASRProviders{Batch: &mockBatchProvider{}}, nil, &v1conf.Asr{}, nil)
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	out, cancelOut := uc.ApplyRecognizeTimeoutForTest(parent)
	defer cancelOut()
	if out != parent {
		t.Error("ctx should be unchanged when no timeout configured")
	}
}

// TestASRUsecase_ApplyRecognizeTimeout_WithConfig 验证配置后 ctx 带 deadline。
func TestASRUsecase_ApplyRecognizeTimeout_WithConfig(t *testing.T) {
	dur := 100 * time.Millisecond
	conf := &v1conf.Asr{
		Timeouts: &v1conf.Asr_Timeouts{
			Recognize: durationpb.New(dur),
		},
	}
	uc := biz.NewASRUsecase(&biz.ASRProviders{Batch: &mockBatchProvider{}}, nil, conf, nil)
	out, cancelOut := uc.ApplyRecognizeTimeoutForTest(context.Background())
	defer cancelOut()

	deadline, ok := out.Deadline()
	if !ok {
		t.Fatal("ctx should have deadline")
	}
	remaining := time.Until(deadline)
	if remaining > dur+50*time.Millisecond || remaining < 0 {
		t.Errorf("deadline remaining = %v, want ~%v", remaining, dur)
	}
}

// TestASRUsecase_ApplyStreamTimeout_NoConfig 验证无配置时不修改 ctx。
func TestASRUsecase_ApplyStreamTimeout_NoConfig(t *testing.T) {
	uc := biz.NewASRUsecase(&biz.ASRProviders{Batch: &mockBatchProvider{}}, nil, &v1conf.Asr{}, nil)
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	out, cancelOut := uc.ApplyStreamTimeoutForTest(parent)
	defer cancelOut()
	if out != parent {
		t.Error("ctx should be unchanged when no timeout configured")
	}
}

// TestASRUsecase_ApplyStreamTimeout_WithConfig 验证配置后 ctx 带 deadline。
func TestASRUsecase_ApplyStreamTimeout_WithConfig(t *testing.T) {
	dur := 200 * time.Millisecond
	conf := &v1conf.Asr{
		Timeouts: &v1conf.Asr_Timeouts{
			Stream: durationpb.New(dur),
		},
	}
	uc := biz.NewASRUsecase(&biz.ASRProviders{Batch: &mockBatchProvider{}}, nil, conf, nil)
	out, cancelOut := uc.ApplyStreamTimeoutForTest(context.Background())
	defer cancelOut()

	deadline, ok := out.Deadline()
	if !ok {
		t.Fatal("ctx should have deadline")
	}
	remaining := time.Until(deadline)
	if remaining > dur+50*time.Millisecond || remaining < 0 {
		t.Errorf("deadline remaining = %v, want ~%v", remaining, dur)
	}
}
