package service

import (
	"context"

	pb "backend-service/api/evie/service/v1"
	"backend-service/app/evie/service/internal/biz"
)

// resolveEnhancePolicy 解析文本增强方案：
//   - profileID > 0：按增强场景（Profile）绑定的策略执行
//   - profileID == 0：使用租户默认增强方案（第一个启用的策略）；没有策略时返回 nil（执行全部步骤）
func resolveEnhancePolicy(ctx context.Context, uc *biz.EnhancementPolicyUsecase, profileID uint32) (*pb.EnhancementPolicy, error) {
	if uc == nil {
		return nil, nil
	}
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
	// 默认增强方案：取租户第一个策略
	policies, _, err := uc.ListPolicies(ctx, &pb.ListPoliciesRequest{PageSize: 1})
	if err != nil {
		return nil, err
	}
	if len(policies) > 0 {
		return policies[0], nil
	}
	return nil, nil
}
