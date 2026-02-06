package assets

import (
	"embed"
)

//go:embed swagger-ui/*
//go:embed openapi.yaml
var OpenApiData embed.FS
