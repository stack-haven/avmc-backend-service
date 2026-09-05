// Package service 聚合 evie/tool 的 transport ↔ biz 桥接 Provider。
//
// 包含：EnhancementService（M6c）、ASRService（M7）。
package service

import "github.com/google/wire"

// ProviderSet service providers。
var ProviderSet = wire.NewSet(
	NewEnhancementService,
	NewASRService,
)
