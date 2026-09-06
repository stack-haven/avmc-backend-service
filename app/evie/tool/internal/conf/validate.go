package conf

import (
	"errors"
	"fmt"
	"strings"
)

// Validate 校验 Bootstrap 配置的最小可用性。
//
// 目标：启动期快速失败，避免半配置状态运行时出现 401/超时/降级
// 行为不一致的“悄悄能起”问题。
//
// 只做结构性 + 关键字段非空检查；语义层面的合理性（如阈值范围）
// 在业务层使用前自行校验。
func Validate(b *Bootstrap) error {
	if b == nil {
		return errors.New("conf: bootstrap is nil")
	}
	var errs []string

	// Server
	if b.Server == nil || b.Server.Http == nil || b.Server.Http.Addr == "" {
		errs = append(errs, "server.http.addr is required")
	}
	if b.Server == nil || b.Server.Grpc == nil || b.Server.Grpc.Addr == "" {
		errs = append(errs, "server.grpc.addr is required")
	}

	// Redis
	if b.Data == nil || b.Data.Redis == nil || b.Data.Redis.Addr == "" {
		errs = append(errs, "data.redis.addr is required")
	} else if b.Data.Redis.TokenKeyPrefix == "" {
		errs = append(errs, "data.redis.token_key_prefix is required")
	}

	// Qua
	if b.Qua == nil || b.Qua.BaseUrl == "" {
		errs = append(errs, "qua.base_url is required")
	} else {
		if b.Qua.Endpoints == nil || b.Qua.Endpoints.ListUsers == "" {
			errs = append(errs, "qua.endpoints.list_users is required")
		}
		if b.Qua.Endpoints == nil || b.Qua.Endpoints.ListDepts == "" {
			errs = append(errs, "qua.endpoints.list_depts is required")
		}
	}

	// ASR
	if b.Asr == nil {
		errs = append(errs, "asr section is required")
	} else if b.Asr.Providers == nil ||
		!(b.Asr.Providers.Funasr.GetEnabled() || b.Asr.Providers.Xunfei.GetEnabled()) {
		errs = append(errs, "asr: at least one provider must be enabled")
	}

	// Enhancement pipeline
	if b.Enhancement == nil {
		errs = append(errs, "enhancement section is required")
	} else {
		allowed := map[string]bool{
			"cleaning": true, "filler": true,
			"vocab_matching": true, "alias_resolution": true,
			"deterministic_replacement": true, "phrase_standardization": true,
			"pinyin_correction": true, "fuzzy_matching": true,
			"context_correction": true, "llm_reserved": true,
		}
		for _, step := range b.Enhancement.Pipeline {
			if !allowed[step] {
				errs = append(errs, fmt.Sprintf("enhancement.pipeline: unknown step %q", step))
			}
		}
	}

	// System dict / tenant registry
	if b.SystemDict == nil || b.SystemDict.Path == "" {
		errs = append(errs, "system_dict.path is required")
	}
	if b.TenantRegistry == nil || b.TenantRegistry.Path == "" {
		errs = append(errs, "tenant_registry.path is required")
	}

	if len(errs) > 0 {
		return fmt.Errorf("conf: invalid config: %s", strings.Join(errs, "; "))
	}
	return nil
}
