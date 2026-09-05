// Copyright 2024 The Ark Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lexicon_test

import (
	"testing"

	"github.com/stack-haven/lexnorm/lexicon"
)

func TestPinyinIndex_Basic(t *testing.T) {
	idx := lexicon.NewPinyinIndex()
	idx.Add("tian", "e1")
	idx.Add("hua", "e2")
	idx.Add("nian", "e3")
	idx.Build()

	got := idx.Query("tian")
	if len(got) != 1 || got[0] != "e1" {
		t.Errorf("Query(\"tian\") = %v, want [e1]", got)
	}
}

func TestPinyinIndex_MultipleForms(t *testing.T) {
	// A character with 多音字 indexed under multiple pinyins.
	idx := lexicon.NewPinyinIndex()
	idx.Add("chang", "e1") // 长 as chang
	idx.Add("zhang", "e1") // 长 as zhang
	idx.Build()

	// Querying either form returns e1.
	got1 := idx.Query("chang")
	got2 := idx.Query("zhang")
	if len(got1) != 1 || got1[0] != "e1" {
		t.Errorf("Query(\"chang\") = %v, want [e1]", got1)
	}
	if len(got2) != 1 || got2[0] != "e1" {
		t.Errorf("Query(\"zhang\") = %v, want [e1]", got2)
	}
}

func TestPinyinIndex_MultiFormQuery(t *testing.T) {
	idx := lexicon.NewPinyinIndex()
	idx.Add("ni", "e1")
	idx.Add("hao", "e2")
	idx.Build()

	// Query with multiple forms returns union.
	got := idx.Query("ni", "hao")
	if len(got) != 2 {
		t.Errorf("Query(2 forms) = %v, want 2 entries", got)
	}
}

func TestPinyinIndex_NoMatch(t *testing.T) {
	idx := lexicon.NewPinyinIndex()
	idx.Add("tian", "e1")
	idx.Build()

	got := idx.Query("unknown")
	if got != nil {
		t.Errorf("no match should return nil, got %v", got)
	}
}

func TestPinyinIndex_QueryBeforeBuild(t *testing.T) {
	idx := lexicon.NewPinyinIndex()
	idx.Add("tian", "e1")
	// No Build().

	if got := idx.Query("tian"); got != nil {
		t.Errorf("Query before Build must return nil, got %v", got)
	}
}

func TestPassthroughConverter(t *testing.T) {
	c := lexicon.PassthroughConverter{}
	got := c.ToPinyin("hello")
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("PassthroughConverter.ToPinyin = %v, want [hello]", got)
	}

	if got := c.ToPinyin(""); got != nil {
		t.Errorf("PassthroughConverter.ToPinyin(\"\") = %v, want nil", got)
	}
}
