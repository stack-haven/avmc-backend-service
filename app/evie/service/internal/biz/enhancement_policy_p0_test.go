package biz

import (
	"context"
	"errors"
	"strings"
	"testing"

	pb "backend-service/api/evie/service/v1"
	kerrors "github.com/go-kratos/kratos/v2/errors"
)

// enhancementPolicyRepoStub is a minimal EnhancementPolicyRepo stub for GeneratePinyin tests.
type enhancementPolicyRepoStub struct {
	generatePinyinFn func(text string, includeInitials bool) (*pb.GeneratePinyinResponse, error)
}

func (s *enhancementPolicyRepoStub) ListPolicies(context.Context, *pb.ListPoliciesRequest) ([]*pb.EnhancementPolicy, int32, error) {
	return nil, 0, nil
}
func (s *enhancementPolicyRepoStub) GetPolicy(context.Context, uint32) (*pb.EnhancementPolicy, error) {
	return nil, nil
}
func (s *enhancementPolicyRepoStub) CreatePolicy(context.Context, *pb.EnhancementPolicy) (*pb.EnhancementPolicy, error) {
	return nil, nil
}
func (s *enhancementPolicyRepoStub) UpdatePolicy(context.Context, *pb.EnhancementPolicy) (*pb.EnhancementPolicy, error) {
	return nil, nil
}
func (s *enhancementPolicyRepoStub) DeletePolicy(context.Context, uint32) error { return nil }
func (s *enhancementPolicyRepoStub) ListProfiles(context.Context, *pb.ListProfilesRequest) ([]*pb.EnhancementProfile, int32, error) {
	return nil, 0, nil
}
func (s *enhancementPolicyRepoStub) GetProfile(context.Context, uint32) (*pb.EnhancementProfile, error) {
	return nil, nil
}
func (s *enhancementPolicyRepoStub) CreateProfile(context.Context, *pb.EnhancementProfile) (*pb.EnhancementProfile, error) {
	return nil, nil
}
func (s *enhancementPolicyRepoStub) UpdateProfile(context.Context, *pb.EnhancementProfile) (*pb.EnhancementProfile, error) {
	return nil, nil
}
func (s *enhancementPolicyRepoStub) DeleteProfile(context.Context, uint32) error { return nil }

func (s *enhancementPolicyRepoStub) GeneratePinyin(_ context.Context, text string, includeInitials bool) (*pb.GeneratePinyinResponse, error) {
	if s.generatePinyinFn != nil {
		return s.generatePinyinFn(text, includeInitials)
	}
	return &pb.GeneratePinyinResponse{Pinyin: "", PinyinInitial: "", NormalizedText: text}, nil
}

// TestEnhancementPolicyUsecase_GeneratePinyin_Empty 验证空文本返回 TEXT_REQUIRED 错误。
func TestEnhancementPolicyUsecase_GeneratePinyin_Empty(t *testing.T) {
	uc := NewEnhancementPolicyUsecase(&enhancementPolicyRepoStub{}, nil)
	resp, err := uc.GeneratePinyin(context.Background(), "", true)
	if err == nil {
		t.Fatalf("expected error for empty text, got resp=%+v", resp)
	}
	ke := new(kerrors.Error)
	if !errors.As(err, &ke) || ke.Reason != "TEXT_REQUIRED" {
		t.Fatalf("expected TEXT_REQUIRED error, got: %v", err)
	}
}

// TestEnhancementPolicyUsecase_GeneratePinyin_TooLong 验证超过 256 字符返回 TEXT_TOO_LONG 错误。
func TestEnhancementPolicyUsecase_GeneratePinyin_TooLong(t *testing.T) {
	uc := NewEnhancementPolicyUsecase(&enhancementPolicyRepoStub{}, nil)
	long := strings.Repeat("中", 257)
	resp, err := uc.GeneratePinyin(context.Background(), long, true)
	if err == nil {
		t.Fatalf("expected error for too-long text, got resp=%+v", resp)
	}
	ke := new(kerrors.Error)
	if !errors.As(err, &ke) || ke.Reason != "TEXT_TOO_LONG" {
		t.Fatalf("expected TEXT_TOO_LONG error, got: %v", err)
	}
}

// TestEnhancementPolicyUsecase_GeneratePinyin_Passthrough 验证参数透传到 repo。
func TestEnhancementPolicyUsecase_GeneratePinyin_Passthrough(t *testing.T) {
	called := false
	stub := &enhancementPolicyRepoStub{
		generatePinyinFn: func(text string, includeInitials bool) (*pb.GeneratePinyinResponse, error) {
			called = true
			if text != "客服您好" {
				t.Errorf("unexpected text: got %q, want %q", text, "客服您好")
			}
			if !includeInitials {
				t.Errorf("expected includeInitials=true")
			}
			return &pb.GeneratePinyinResponse{
				Pinyin:         "ke fu nin hao",
				PinyinInitial:  "kfnh",
				NormalizedText: "客服您好",
			}, nil
		},
	}
	uc := NewEnhancementPolicyUsecase(stub, nil)
	resp, err := uc.GeneratePinyin(context.Background(), "客服您好", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatalf("repo.GeneratePinyin not invoked")
	}
	if resp.Pinyin != "ke fu nin hao" || resp.PinyinInitial != "kfnh" {
		t.Errorf("response not propagated: %+v", resp)
	}
}

// TestEnhancementPolicyUsecase_GeneratePinyin_MaxLengthBoundary 验证 256 字符边界通过。
func TestEnhancementPolicyUsecase_GeneratePinyin_MaxLengthBoundary(t *testing.T) {
	called := false
	stub := &enhancementPolicyRepoStub{
		generatePinyinFn: func(text string, includeInitials bool) (*pb.GeneratePinyinResponse, error) {
			called = true
			return &pb.GeneratePinyinResponse{NormalizedText: text}, nil
		},
	}
	uc := NewEnhancementPolicyUsecase(stub, nil)
	exact := strings.Repeat("中", 256) // 256 chars == upper bound (len > 256 not allowed)
	resp, err := uc.GeneratePinyin(context.Background(), exact, false)
	if err != nil {
		t.Fatalf("256 chars should be allowed, got: %v", err)
	}
	if !called {
		t.Fatalf("repo.GeneratePinyin not invoked at boundary")
	}
	if resp == nil {
		t.Errorf("unexpected nil response")
	}
}