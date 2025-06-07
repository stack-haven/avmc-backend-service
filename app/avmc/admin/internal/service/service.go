package service

import "github.com/google/wire"

// ProviderSet is service providers.
var ProviderSet = wire.NewSet(
	NewAuthServiceService,
	NewUserServiceService,
	NewDeptServiceService,
	NewMenuServiceService,
	NewRoleServiceService,
	NewPostServiceService,
)
