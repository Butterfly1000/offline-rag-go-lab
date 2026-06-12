// Package chunking 负责把 ingest 请求的正文切成 KnowledgeChunk 列表。
package chunking

import (
	"errors" // 校验 document_id、text 必填
	"fmt"    // 生成 chunk_id：document_id#index
	"strings"

	world "offline-rag-go-lab/internal/gateway/level1_world"
)

const (
	defaultChunkMaxChars     = 120 // 单行超过此字符数（按 rune）会再切分
	defaultChunkOverlapChars = 20  // 长行切分时的重叠 rune 数，避免语义断在边界
)

// BuildChunks 是切块主入口：按行遍历、识别标题、长行滑动窗口切分。
func BuildChunks(req world.IngestRequest) ([]world.KnowledgeChunk, error) {
	if strings.TrimSpace(req.DocumentID) == "" {
		return nil, errors.New("document_id is required")
	}
	if strings.TrimSpace(req.Text) == "" {
		return nil, errors.New("text is required")
	}

	lines := strings.Split(req.Text, "\n") // 按换行拆成行
	chunks := make([]world.KnowledgeChunk, 0, len(lines))
	index := 0                              // 全局 chunk 序号，用于 ChunkID
	currentSectionTitle := ""               // 当前生效的章节标题
	pendingHeading := ""                    // 刚读到但尚未遇到正文的标题（Markdown 标题常单独一行）

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			// 空行：若有 pending 标题，在此刻确认为当前章节（标题下一段正文才用该标题）
			if pendingHeading != "" {
				currentSectionTitle = pendingHeading
				pendingHeading = ""
			}
			continue
		}

		if heading, ok := parseHeadingLine(line); ok {
			pendingHeading = heading // 先挂着，等下一行非空正文或空行确认
			continue
		}
		if pendingHeading != "" {
			currentSectionTitle = pendingHeading
			pendingHeading = ""
		}

		// 一行可能切成多个 part（超长时）
		for _, part := range splitLineWithOverlap(line, defaultChunkMaxChars, defaultChunkOverlapChars) {
			chunks = append(chunks, world.KnowledgeChunk{
				DocumentID: req.DocumentID,
				ChunkID:    fmt.Sprintf("%s#%d", req.DocumentID, index),
				ChunkIndex: index,
				Title:      combineChunkTitle(req.Title, currentSectionTitle),
				SourceRef:  req.SourceRef,
				Text:       part,
				Tags:       append([]string{}, req.Tags...), // 复制 slice，避免共享底层数组被外部修改
			})
			index++
		}
	}

	// 全文无有效行时（如只有空白），仍产出一条 chunk 兜住全文
	if len(chunks) == 0 {
		chunks = append(chunks, world.KnowledgeChunk{
			DocumentID: req.DocumentID,
			ChunkID:    fmt.Sprintf("%s#0", req.DocumentID),
			ChunkIndex: 0,
			Title:      req.Title,
			SourceRef:  req.SourceRef,
			Text:       strings.TrimSpace(req.Text),
			Tags:       append([]string{}, req.Tags...),
		})
	}

	return chunks, nil
}

// parseHeadingLine 判断一行是否为标题：Markdown # 或中文/数字章节行。
func parseHeadingLine(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", false
	}
	if strings.HasPrefix(trimmed, "#") {
		heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#")) // 去掉前导 # 和空白
		return heading, heading != ""
	}
	if looksLikeSectionHeading(trimmed) {
		return strings.TrimRight(trimmed, "：:"), true // 去掉行尾中英文冒号
	}
	return "", false
}

// looksLikeSectionHeading 启发式判断「像小节标题」的行。
func looksLikeSectionHeading(line string) bool {
	if strings.HasSuffix(line, "：") || strings.HasSuffix(line, ":") {
		return true // 以冒号结尾的短行，如「申请条件：」
	}
	if runeCount(line) > 18 {
		return false // 太长不像标题
	}
	return strings.HasPrefix(line, "一、") ||
		strings.HasPrefix(line, "二、") ||
		strings.HasPrefix(line, "三、") ||
		strings.HasPrefix(line, "四、") ||
		strings.HasPrefix(line, "五、") ||
		strings.HasPrefix(line, "1.") ||
		strings.HasPrefix(line, "2.") ||
		strings.HasPrefix(line, "3.")
}

// combineChunkTitle 合并文档标题与章节标题，格式「文档 / 章节」。
func combineChunkTitle(baseTitle, sectionTitle string) string {
	baseTitle = strings.TrimSpace(baseTitle)
	sectionTitle = strings.TrimSpace(sectionTitle)
	if baseTitle == "" {
		return sectionTitle
	}
	if sectionTitle == "" {
		return baseTitle
	}
	return baseTitle + " / " + sectionTitle
}

// splitLineWithOverlap 按 rune 长度切长行，相邻块有 overlapChars 重叠。
func splitLineWithOverlap(line string, maxChars int, overlapChars int) []string {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	if maxChars <= 0 || runeCount(line) <= maxChars {
		return []string{line} // 不需要切
	}

	runes := []rune(line) // 按 Unicode 字符切，避免截断中文
	out := make([]string, 0, len(runes)/maxChars+1)
	start := 0
	for start < len(runes) {
		end := start + maxChars
		if end >= len(runes) {
			out = append(out, strings.TrimSpace(string(runes[start:])))
			break
		}
		out = append(out, strings.TrimSpace(string(runes[start:end])))
		start = end - overlapChars // 下一块起点回退，形成重叠
		if start < 0 {
			start = 0
		}
	}
	return out
}

// runeCount 返回字符串的 Unicode 字符个数（不是字节数）。
func runeCount(value string) int {
	return len([]rune(value))
}
