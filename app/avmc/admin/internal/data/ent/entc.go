//go:build ignore
// +build ignore

package main

import (
	"log"

	"entgo.io/ent/entc"
	"entgo.io/ent/entc/gen"
)

func main() {
	if err := entc.Generate(
		"./schema",
		&gen.Config{
			Target:  "./gen",
			Package: "backend-service/app/avmc/admin/internal/data/ent/gen",
			Features: []gen.Feature{
				gen.FeatureUpsert,
				gen.FeatureModifier,
				gen.FeatureExecQuery,
				gen.FeatureIntercept,
				gen.FeatureLock,
			},
		},
		entc.Extensions(
			&Page{},
		),
	); err != nil {
		log.Fatal("running ent codegen:", err)
	}
}

type Page struct {
	entc.DefaultExtension
}

func (*Page) Templates() []*gen.Template {
	return []*gen.Template{
		gen.MustParse(gen.NewTemplate("page").
			ParseFiles("./templates/page.tmpl")),
	}
}
