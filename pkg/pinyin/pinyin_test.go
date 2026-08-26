package pinyin

import (
	"strings"
	"testing"
)

func TestConvert_HappyPath(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		includeInitials bool
		wantPinyin      string
		wantInitial     string
		wantNorm        string
	}{
		{
			name:            "纯中文",
			input:           "客服您好",
			includeInitials: true,
			wantPinyin:      "ke fu nin hao",
			wantInitial:     "kfnh",
			wantNorm:        "客服您好",
		},
		{
			name:            "含英文混排",
			input:           "Hello 你好",
			includeInitials: true,
			wantNorm:        "Hello 你好",
		},
		{
			name:            "全角标点被剥离后中文相连",
			input:           "客服，您好！",
			includeInitials: true,
			wantNorm:        "客服您好",
		},
		{
			name:            "全角空格归一为单空格",
			input:           "客服\u3000您好",
			includeInitials: true,
			wantNorm:        "客服 您好",
		},
		{
			name:            "首字母关闭",
			input:           "客服您好",
			includeInitials: false,
			wantPinyin:      "ke fu nin hao",
			wantInitial:     "",
			wantNorm:        "客服您好",
		},
	}
	c := NewConverter()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.Convert(tt.input, tt.includeInitials)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatalf("Convert returned nil")
			}
			if tt.wantPinyin != "" && got.Pinyin != tt.wantPinyin {
				t.Errorf("Pinyin: got %q, want %q", got.Pinyin, tt.wantPinyin)
			}
			if tt.wantInitial != "" && got.PinyinInitial != tt.wantInitial {
				t.Errorf("PinyinInitial: got %q, want %q", got.PinyinInitial, tt.wantInitial)
			}
			if got.NormalizedText != tt.wantNorm {
				t.Errorf("NormalizedText: got %q, want %q", got.NormalizedText, tt.wantNorm)
			}
		})
	}
}

func TestConvert_EmptyAndEdge(t *testing.T) {
	c := NewConverter()
	cases := []struct {
		name  string
		input string
	}{
		{"empty string", ""},
		{"only spaces", "   "},
		{"only ascii punctuation", "!!!,,,..."},
		{"only tabs", "\t\t\n"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.Convert(tt.input, true)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got == nil {
				t.Fatalf("nil result for edge input")
			}
			if got.Pinyin != "" || got.PinyinInitial != "" {
				t.Errorf("expected empty pinyin/initial for %q, got %+v", tt.input, got)
			}
		})
	}
}

func TestConvert_Deterministic(t *testing.T) {
	c := NewConverter()
	a, err := c.Convert("金种子", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	b, err := c.Convert("金种子", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.Pinyin != b.Pinyin || a.PinyinInitial != b.PinyinInitial {
		t.Errorf("non-deterministic result: %+v vs %+v", a, b)
	}
	// 全拼不应包含连续双空格（归一化保证）
	if strings.Contains(a.Pinyin, "  ") {
		t.Errorf("Pinyin should not contain double spaces: %q", a.Pinyin)
	}
}

func TestConvert_TopLevelConvenience(t *testing.T) {
	// 直接调用包级 Convert 也应工作
	got, err := Convert("测试", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil || got.Pinyin == "" {
		t.Errorf("expected non-empty result, got %+v", got)
	}
}