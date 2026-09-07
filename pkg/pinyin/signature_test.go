package pinyin

import (
	"testing"
)

func TestSignature(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"陈欣静", "cxj"},
		{"陈兴静", "cxj"},
		{"陈新进", "cxj"},
		{"佘丽群", "slq"},
		{"周丽群", "zlq"},
		{"伍锡辉", "wxh"},
		{"武西辉", "wxh"},
		{"田清", "tq"},
		{"田青", "tq"},
		{"婷青", "tq"},
		{"陈科沆", "ckh"},
		{"陈科航", "ckh"},
		{"金种籽", "jzz"},
		{"金种子", "jzz"},
		{"", ""},
		// ASCII-only returns empty（拼音库不处理 ASCII）；故意仅中文计算
	}
	for _, c := range cases {
		got := Signature(c.in)
		if got != c.want {
			t.Errorf("Signature(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestFuzzyEqual(t *testing.T) {
	if !FuzzyEqual("cxj", "cxj") {
		t.Error("same should equal")
	}
	if FuzzyEqual("cxj", "slq") {
		t.Error("different should not equal")
	}
	if FuzzyEqual("", "cxj") {
		t.Error("empty vs non-empty should not equal")
	}
	if !FuzzyEqual("", "") {
		t.Error("empty == empty")
	}
}

func TestExactEqual(t *testing.T) {
	if !ExactEqual("abc", "abc") {
		t.Error("identical strings should equal")
	}
	if ExactEqual("abc", "abd") {
		t.Error("differing strings should not equal")
	}
}
