package service

import "github.com/google/wire"

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(
	NewDictionaryServiceService,
	NewASRServiceService,
	NewASRStreamService,
	NewProviderServiceService,
	NewEnhancementServiceService,
)
