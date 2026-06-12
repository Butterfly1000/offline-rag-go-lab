// Package retrieval 实现教学版检索算法：文本标准化、分词、token 重叠相似度。
// 未来接真实 embedding 时，Similarity 可替换为向量余弦相似度，NormalizeText 仍可复用。
package retrieval

import (
	"math"    // Sqrt 用于归一化相似度分母
	"regexp"  // 匹配英文/数字 token
	"strings" // 大小写、空白处理
)

// asciiTokenPattern 匹配连续的小写字母和数字（NormalizeText 后生效）。
var asciiTokenPattern = regexp.MustCompile(`[a-z0-9]+`)

// NormalizeText 标准化文本：去首尾空白 → 转小写 → 合并连续空白为单个空格。
// 检索和去重共用此逻辑，保证同一句话不同写法得到相同 key。
func NormalizeText(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.TrimSpace(value))), " ")
}

// Similarity 计算 query 与 text 的相似度，范围约 0~1。
// 公式：重叠 token 数 / sqrt(len(qTokens) * len(tTokens))，类似归一化点积。
func Similarity(query, text string) float64 {
	qTokens := tokenize(query)
	tTokens := tokenize(text)
	if len(qTokens) == 0 || len(tTokens) == 0 {
		return 0 // 任一侧无 token 则无法匹配
	}

	overlap := 0
	for token := range qTokens {
		if _, ok := tTokens[token]; ok {
			overlap++ // query 中有多少 token 出现在 text 里
		}
	}
	if overlap == 0 {
		return 0
	}

	return float64(overlap) / math.Sqrt(float64(len(qTokens)*len(tTokens)))
}

// tokenize 把文本切成 token 集合（map 去重）。
// 英文：正则提取 [a-z0-9]+；中文：单字 + 相邻二字 bigram，提升「退款步骤」类匹配。
func tokenize(value string) map[string]struct{} {
	value = NormalizeText(value)
	result := make(map[string]struct{})

	// 英文/数字 token
	for _, match := range asciiTokenPattern.FindAllString(value, -1) {
		result[match] = struct{}{}
	}

	// 中文：Unicode 范围 \u4e00-\u9fff（CJK 统一汉字）
	cjk := make([]rune, 0, len(value))
	for _, r := range value {
		if r >= '\u4e00' && r <= '\u9fff' {
			cjk = append(cjk, r)
			result[string(r)] = struct{}{} // 单字 token
		}
	}
	// 相邻两字 bigram，如「退款」「款步」
	for i := 0; i < len(cjk)-1; i++ {
		result[string(cjk[i:i+2])] = struct{}{}
	}
	return result
}
