package biz

import "context"

// EditDistanceCorrector 编辑距离纠错器：将 ASR 文本中与字典标准词（或别名）
// 编辑距离较近的错字纠正为标准词。优先级 70，低于规则(100)/实体(90)/拼音(80)。
type EditDistanceCorrector struct {
	dict    DictionaryRepo
	maxDist int // 最大编辑距离，默认 2
}

// NewEditDistanceCorrector 创建编辑距离纠错器。
func NewEditDistanceCorrector(dict DictionaryRepo) *EditDistanceCorrector {
	return &EditDistanceCorrector{dict: dict, maxDist: 2}
}

func (e *EditDistanceCorrector) Name() string  { return "edit_distance" }
func (e *EditDistanceCorrector) Priority() int { return 70 }

// Correct 用滑动窗口将文本中的子串与字典词做编辑距离匹配，生成纠错候选。
//
// 仅匹配「与字典词同长度」的子串；置信度 = 1 - dist/len(word)，
// 阈值 0.6（即 3 字词错 1 字可纠，2 字词错 1 字不纠，避免误伤短词）。
func (e *EditDistanceCorrector) Correct(ctx context.Context, text string) ([]CorrectionCandidate, error) {
	words, err := e.dict.ListActiveWords(ctx)
	if err != nil {
		return nil, err
	}
	if len(words) == 0 {
		return nil, nil
	}

	runes := []rune(text)
	var candidates []CorrectionCandidate
	for _, word := range words {
		n := len([]rune(word))
		if n < 2 || n > 8 {
			continue // 过短/过长的词不做编辑距离纠错
		}
		for i := 0; i+n <= len(runes); i++ {
			sub := string(runes[i : i+n])
			dist := levenshteinDistance(sub, word)
			if dist == 0 || dist > e.maxDist {
				continue
			}
			conf := 1.0 - float64(dist)/float64(n)
			if conf < 0.6 {
				continue
			}
			candidates = append(candidates, CorrectionCandidate{
				From:       sub,
				To:         word,
				Type:       "edit_distance",
				Confidence: conf,
				Source:     "edit_distance",
			})
		}
	}
	return candidates, nil
}

// levenshteinDistance 计算两个字符串的编辑距离（Levenshtein，基于 rune）。
func levenshteinDistance(a, b string) int {
	ar, br := []rune(a), []rune(b)
	la, lb := len(ar), len(br)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		for j := 1; j <= lb; j++ {
			cost := 0
			if ar[i-1] != br[j-1] {
				cost = 1
			}
			curr[j] = minInt(prev[j]+1, minInt(curr[j-1]+1, prev[j-1]+cost))
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
