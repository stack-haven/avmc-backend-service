package data

import (
	"context"
	"errors"

	"backend-service/app/evie/tool/internal/biz"
	"backend-service/app/evie/tool/internal/conf"
	filesrc "backend-service/app/evie/tool/pkg/source/file"
)

// FileVocabularySource 把 pkg/source/file 适配为 biz.VocabularySource。
//
// 用于 demo / 离线模式：词库从本地 JSON 文件加载，不依赖 qua HTTP。
type FileVocabularySource struct {
	src *filesrc.Source
}

// NewFileVocabularySource 从 conf.FileVocabulary 构造 biz.VocabularySource。
func NewFileVocabularySource(cfg *conf.FileVocabulary) (biz.VocabularySource, error) {
	if cfg == nil || cfg.Path == "" {
		return nil, errors.New("data: file vocabulary requires non-empty path")
	}
	src, err := filesrc.New(filesrc.Config{
		Path:           cfg.Path,
		ReloadOnFetch:  cfg.ReloadOnFetch,
		UserEntityType: "user",
		DeptEntityType: "department",
	})
	if err != nil {
		return nil, err
	}
	return &FileVocabularySource{src: src}, nil
}

// Name 实现 biz.VocabularySource。
func (f *FileVocabularySource) Name() string { return "file" }

// Fetch 把 file source 的 RawEntity 转为 biz.RawEntity（字段一一对应）。
func (f *FileVocabularySource) Fetch(ctx context.Context) ([]biz.RawEntity, error) {
	raw, err := f.src.Fetch(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]biz.RawEntity, 0, len(raw))
	for _, e := range raw {
		out = append(out, biz.RawEntity{
			SourceID:   e.SourceID,
			EntityType: e.EntityType,
			Source:     e.Source,
			Data:       e.Data,
		})
	}
	return out, nil
}
