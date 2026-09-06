package biz_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-kratos/kratos/v2/log"

	"backend-service/app/evie/tool/internal/biz"
)

// BenchmarkASRUsecase_AppendList 测量 ASR 记录追加 + 按租户分页查询的延迟。
func BenchmarkASRUsecase_AppendList(b *testing.B) {
	dir := b.TempDir()
	uc := biz.NewASRUsecase(
		&biz.ASRProviders{Batch: &mockBatchProvider{text: "x"}, Stream: &mockStreamProvider{}},
		makeEnhancer(b, dir),
		makeConf(b, "upload/audio"),
		log.DefaultLogger,
	)

	// 预填 200 条，分布到 2 个 tenant。
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		_, _ = uc.Recognize(ctx, fmt.Sprintf("u%d", i), "158", []byte{1, 2, 3},
			biz.AudioFormat{Encoding: "pcm", SampleRate: 16000, BitDepth: 16},
			"", false)
	}
	for i := 0; i < 100; i++ {
		_, _ = uc.Recognize(ctx, fmt.Sprintf("u%d", i), "159", []byte{1, 2, 3},
			biz.AudioFormat{Encoding: "pcm", SampleRate: 16000, BitDepth: 16},
			"", false)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 模拟新记录追加。
		_, _ = uc.Recognize(ctx, fmt.Sprintf("u%d", i%50), "158", []byte{1, 2, 3},
			biz.AudioFormat{Encoding: "pcm", SampleRate: 16000, BitDepth: 16},
			"", false)
		// 模拟分页查询。
		_, _, _ = uc.ListRecords(ctx, "158", 20, "")
	}
}
