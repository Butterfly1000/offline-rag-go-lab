# 03 Chunking Behavior

主题：系统如何把一篇文档切成可检索的 chunk

这一课开始进入更具体的实现。重点不是只知道“会切块”，而是知道当前 demo 到底按什么规则切，以及为什么会切成那些结果。

---

## Part 1：当前 demo 到底把什么当成“标题”

先看标题识别函数，代码来自 [internal/gateway/level5_chunking/chunker.go](/Users/huangyanyu/offline-rag-go-lab/internal/gateway/level5_chunking/chunker.go:83)。

```go
// parseHeadingLine 判断一行是否为标题：Markdown # 或中文/数字章节行。
func parseHeadingLine(line string) (string, bool) {
    trimmed := strings.TrimSpace(line)
    if trimmed == "" {
        return "", false
    }

    // 规则 1：以 # 开头，按 Markdown 标题处理。
    if strings.HasPrefix(trimmed, "#") {
        // strings.TrimLeft(trimmed, "#")：
        // 去掉左边连续的 #，比如 ### 标题 也会变成 标题
        // 再用 TrimSpace 去掉两边空格。
        heading := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
        return heading, heading != ""
    }

    // 规则 2：如果长得像“小节标题”，也算标题。
    if looksLikeSectionHeading(trimmed) {
        // strings.TrimRight(trimmed, "：:")：
        // 去掉右边结尾的中文冒号或英文冒号。
        return strings.TrimRight(trimmed, "：:"), true
    }

    return "", false
}
```

这段代码说明：当前 demo 不只是认 Markdown 标题，它认两大类：

1. Markdown 风格
   - `# 退款政策`
   - `## 申请流程`

2. 启发式“小节标题”
   - `一、退款条件`
   - `二、处理流程`
   - `1. 提交材料`
   - `申请条件：`

再看 `looksLikeSectionHeading`，代码来自 [internal/gateway/level5_chunking/chunker.go](/Users/huangyanyu/offline-rag-go-lab/internal/gateway/level5_chunking/chunker.go:99)。

```go
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
```

这就把“看起来像标题”具体化了。当前规则其实很死板，只有这几种：

- 结尾是 `：` 或 `:`
- 长度不能太长，超过 `18` 个字符就不像标题
- 前缀是：
  - `一、`
  - `二、`
  - `三、`
  - `四、`
  - `五、`
  - `1.`
  - `2.`
  - `3.`

所以这个 demo 不是“智能识别标题”，而是“靠一组硬编码规则识别标题”。

### 会被识别成标题的例子

```text
# 退款政策
## 申请流程
一、退款条件
2. 审核步骤
申请材料：
```

### 大概率不会被识别成标题的例子

```text
退款政策说明
第一章 退款规则
(一) 申请条件
步骤一 提交订单号
这是一个很长很长很长的标题候选，超过了 18 个字符
```

### Part 1 总概述

当前 chunker 识别标题，不是靠智能理解，而是靠硬编码规则。

这些规则目前主要支持两类：

- Markdown 标题
- 少量中文/数字章节标题

### Part 1 重点

- `strings.HasPrefix(trimmed, "#")`：识别 Markdown 标题。
- `looksLikeSectionHeading(...)`：识别少量启发式小节标题。
- `strings.HasSuffix(line, "：") || strings.HasSuffix(line, ":")`：冒号结尾也可能被当标题。
- `runeCount(line) > 18`：太长的行直接不当标题。
- 这套规则是 demo 级启发式，不是完整文档解析。
