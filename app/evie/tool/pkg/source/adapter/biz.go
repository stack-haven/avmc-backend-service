// Package adapter bridges evie/tool/pkg/source.Source with the legacy
// biz.VocabularySource interface so applications that already use
// the biz interface can consume the new evie/tool/pkg/source adapters without
// changing their existing wiring.
//
// # Usage
//
//	hs, _ := httpsource.New(cfg)
//	bizSrc := adapter.Wrap(hs)
//	// bizSrc implements biz.VocabularySource
package adapter

import (
	"context"

	"backend-service/app/evie/tool/pkg/source"
)

// VocabularySource is the legacy interface that mirrors
// biz.VocabularySource. Applications that already depend on
// biz.VocabularySource can keep using it.
type VocabularySource interface {
	Name() string
	Fetch(ctx context.Context) ([]RawEntity, error)
}

// RawEntity mirrors biz.RawEntity. Both types have identical JSON
// layout, so values can be copied field-for-field without
// re-marshalling.
type RawEntity struct {
	SourceID   string
	EntityType string
	Source     string
	Data       map[string]any
}

// Wrap returns a VocabularySource that delegates to s.
func Wrap(s source.Source) VocabularySource {
	return &bridged{s: s}
}

type bridged struct{ s source.Source }

func (b *bridged) Name() string { return b.s.Name() }

func (b *bridged) Fetch(ctx context.Context) ([]RawEntity, error) {
	src, err := b.s.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]RawEntity, 0, len(src))
	for _, e := range src {
		out = append(out, RawEntity{
			SourceID:   e.SourceID,
			EntityType: e.EntityType,
			Source:     e.Source,
			Data:       e.Data,
		})
	}
	return out, nil
}
