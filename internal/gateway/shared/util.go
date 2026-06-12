// Package shared 提供各关卡共用的纯函数小工具，无状态、无业务编排。
package shared

import (
	"math"    // Round4 用 math.Round 做四舍五入
	"os"      // MustMkdirAll 用 os.MkdirAll 创建目录
	"strings" // 字符串裁剪、判空

	world "offline-rag-go-lab/internal/gateway/level1_world"
	retrieval "offline-rag-go-lab/internal/gateway/level4_retrieval"
)

// MustMkdirAll 递归创建目录；失败则 panic。
// os.MkdirAll 会连同不存在的父目录一起创建（例如 storage/logs 会顺带创建 storage）。
// 0o755 是八进制权限：所有者 rwx，组和其他用户 rx。
// 「Must」前缀是 Go 惯例：表示出错不可恢复，直接 panic 而非返回 error。
func MustMkdirAll(path string) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		panic(err)
	}
}

// Truncate 将字符串截到最多 max 个「字节」（注意：中文等多字节字符可能被截断一半）。
// max <= 0 或原串已够短时，原样返回。
func Truncate(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}

// Round4 将浮点数四舍五入到 4 位小数，用于 API 返回的 score 字段更整洁。
func Round4(value float64) float64 {
	return math.Round(value*10000) / 10000
}

// ValueOrDefault 若 value 去空白后为空，返回 fallback；否则返回 value。
func ValueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

// ChunkIDs 从命中列表提取所有 chunk_id，用于 JSONL 日志字段 retrieved_chunk_ids。
func ChunkIDs(hits []world.RetrievalHit) []string {
	out := make([]string, 0, len(hits)) // 预分配容量，减少 append 扩容
	for _, hit := range hits {
		out = append(out, hit.ChunkID)
	}
	return out
}

// LimitHits 只保留前 max 条命中；max <= 0 或原列表更短时，原样返回。
func LimitHits(hits []world.RetrievalHit, max int) []world.RetrievalHit {
	if max <= 0 || len(hits) <= max {
		return hits
	}
	return hits[:max] // 切片截取，共享底层数组
}

// DedupeHitsByContent 按「标题+正文」标准化后的内容去重，保留首次出现的命中。
// 用于压缩阶段避免 prompt 里重复段落；标准化逻辑与检索层 NormalizeText 一致。
func DedupeHitsByContent(hits []world.RetrievalHit) []world.RetrievalHit {
	if len(hits) <= 1 {
		return hits
	}

	seen := make(map[string]struct{}, len(hits)) // struct{} 不占额外内存，仅作 set
	out := make([]world.RetrievalHit, 0, len(hits))
	for _, hit := range hits {
		key := retrieval.NormalizeText(hit.Title + " " + hit.Text)
		if _, ok := seen[key]; ok {
			continue // 已见过相同内容，跳过
		}
		seen[key] = struct{}{}
		out = append(out, hit)
	}
	return out
}
