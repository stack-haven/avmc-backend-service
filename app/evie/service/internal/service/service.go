package service

import "github.com/google/wire"

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(
	NewDictionaryServiceService,
	NewHotwordServiceService,
	NewASRServiceService,
	NewASRStreamService,
	NewProviderServiceService,
	NewCorrectionServiceService,
)
