//go:build ignore
// +build ignore

package main

import (
	entgeno "backend-service/pkg/entgo/entc"

	"entgo.io/ent/entc/gen"
)

func main() {
	// version 服务沿用 Ent 默认布局（生成物直接在 ent/ 下）且不启用任何 Feature，
	// 因此显式传空特性集合（非 nil 空切片 = 不要特性）。
	entgeno.MustGenerate(&entgeno.Options{
		Target:   ".",
		Features: []gen.Feature{},
	})
}
