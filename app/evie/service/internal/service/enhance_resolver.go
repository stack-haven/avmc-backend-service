package service

import (
	"context"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
)

// resolveEnhancePolicy 解析文本增强方案：
//   - profileID > 0：按增强场景（Profile）绑定的策略执行
//
// 设计原则：增强策略只能通过场景 Profile 关联，不接受 policy_id 直传。
// 这与系统「场景关联策略」的设计一致——避免策略在多个场景间游离。
func resolveEnhancePolicy(ctx context.Context, uc *biz.EnhancementPolicyUsecase, profileID uint32) (*pb.EnhancementPolicy, error) {
	if uc == nil {
		return nil, nil
	}
	// 1) profileID：按场景绑定的策略取
	if profileID != 0 {
		profile, err := uc.GetProfile(ctx, profileID)
		if err != nil {
			return nil, err
		}
		if profile == nil || profile.GetPolicyId() == 0 {
			return nil, nil
		}
		return uc.GetPolicy(ctx, profile.GetPolicyId())
	}
	// 默认增强方案：优先选择「高性能模式且启用口水词+别名+确定性替换」的策略；
	// 其次选启用核心步骤（口水词+别名+确定性替换）的策略；没有则 nil（执行全部步骤）。
	policies, _, err := uc.ListPolicies(ctx, &pb.ListPoliciesRequest{PageSize: 100})
	if err != nil {
		return nil, err
	}
	for _, p := range policies {
		if p.GetMode() == "HIGH_PERFORMANCE" && p.GetFillerRemoval() && p.GetAliasResolution() && p.GetDeterministicReplacement() {
			return p, nil
		}
	}
	for _, p := range policies {
		if p.GetFillerRemoval() && p.GetAliasResolution() && p.GetDeterministicReplacement() {
			return p, nil
		}
	}
	return nil, nil
}
