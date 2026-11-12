//go:build ignore
// +build ignore

package main

import (
	entgeno "backend-service/pkg/entgo/entc"
)

func main() {
	entgeno.MustGenerate(&entgeno.Options{})
}
