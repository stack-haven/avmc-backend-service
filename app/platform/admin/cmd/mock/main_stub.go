//go:build !mock

// Package main provides a mock data generator for development.
// The full mock generator is in main.go (build tag: mock).
// This fallback provides a no-op binary for the default build.
package main

import "fmt"

func main() {
	fmt.Println("mock: use -tags mock to build the full mock data generator") //nolint:forbidigo // mock CLI tool
}
