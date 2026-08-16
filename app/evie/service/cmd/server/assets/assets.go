package assets

import (
	"embed"
)

//go:embed swagger-ui/*
//go:embed openapi.yaml
var OpenAPIData embed.FS
