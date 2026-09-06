package biz_test

import (
	"testing"
)

// BenchmarkEnhancementUsecase_EnhanceText 测量 8 层增强端到端延迟。
//
// 复用 makeEnhancer（其内部构造 lexnorm engine + VocabularyBuilder + system dict）。
func BenchmarkEnhancementUsecase_EnhanceText(b *testing.B) {
	uc := makeEnhancer(b, b.TempDir())
	cases := []string{
		"金种籽是新产品",
		"今天开会讨论了佘丽群负责的金种子与黑种籽方案",
		"我们需要测试长文本的模糊匹配能力，包括人名纠错、词库匹配、短语标准化与拼音同音纠错。",
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := uc.EnhanceText(b.Context(), cases[i%len(cases)], "158")
		if err != nil {
			b.Fatalf("enhance: %v", err)
		}
	}
}
