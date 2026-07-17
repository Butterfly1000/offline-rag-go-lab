# 第 30 节：Markdown 与 Go 结构化分块

本节实现生产 document ingestion 的第二个环节：**不是按固定字符数盲切，而是先识别文档结构，再用真实 tokenizer 强制每个 chunk 不超过 token 上限。**

本节只读取本地文件和 tokenizer，不连接 MySQL、Qdrant 或 Ollama。

## 1. 先实践效果

### 1.1 Markdown

在仓库根目录执行：

```bash
go run ./cmd/document-chunk-demo \
  --config config/recent-chat.env \
  --format markdown \
  --source internal/documentingest/testdata/course.md \
  --max-tokens 160
```

关键结果：

```text
Format: markdown
Policy: max_tokens=160 overlap_lines=2
Chunks: 5

[0] kind=paragraph tokens=33
path=Document Ingestion

[2] kind=paragraph tokens=41
path=Document Ingestion / Version State

[3] kind=code tokens=36
path=Document Ingestion / Version State
```

观察重点：

- 标题本身没有变成空 chunk，而是成为正文的结构路径。
- `Version State` 下的段落和 Go fenced code 分成不同类型。
- 每个 chunk 都打印真实 token 数和稳定 `chunk_id`。

### 1.2 Go 源码

执行：

```bash
go run ./cmd/document-chunk-demo \
  --config config/recent-chat.env \
  --format go \
  --source internal/documentingest/testdata/service.go.txt \
  --max-tokens 160
```

关键结果：

```text
Format: go
Chunks: 5

[0] kind=go_preamble tokens=11
path=package course

[2] kind=go_declaration tokens=28
path=package course / type Version

[3] kind=go_declaration tokens=88
path=package course / func Version.Activate
```

观察重点：

- package comment 和 package 声明形成 preamble。
- import、type、func 使用 Go 语法树识别，不靠字符串猜测。
- `Version.Activate` 的 doc comment 与函数声明保留在同一结构中。
- 不属于任何声明的独立注释会归入后续声明；文件末尾注释归入最后一个声明，避免静默丢失可检索内容。

### 1.3 强制小预算观察真实拆分

执行：

```bash
go run ./cmd/document-chunk-demo \
  --config config/recent-chat.env \
  --format markdown \
  --source internal/documentingest/testdata/course.md \
  --max-tokens 30 \
  --overlap-lines 1
```

此时会得到 11 个 chunks，而且每个 `tokens <= 30`。超长 fenced code 被拆成两个仍带有 `````go`` 和 ````` `` 的独立代码块。

## 2. Part 1：整体代码流程

入口：

- [main.go](/offline-rag-go-lab/cmd/document-chunk-demo/main.go:1)

核心编排：

- [chunker.go](/offline-rag-go-lab/internal/documentingest/chunker.go:1)

主流程：

```text
读取文件
-> NormalizeDocument
-> Markdown 或 Go parser
-> structural units
-> 超限 unit 拆分
-> 同标题相邻段落按预算合并
-> 再次真实 token 计数
-> StableChunkID
-> 输出 chunks
```

`ChunkDocument` 接收 `TokenCounter` 接口，而不是依赖某个全局 tokenizer：

```go
type TokenCounter interface {
	Count(text string) (int, error)
}
```

这样单元测试可以精确构造边界，真实 demo 则使用本地 Qwen tokenizer。

## 3. Part 2：Markdown 如何识别标题和代码块

代码：

- [markdown.go](/offline-rag-go-lab/internal/documentingest/markdown.go:1)

当前只把合法 ATX heading 识别为标题：

```markdown
# 一级标题
## 二级标题
### 三级标题
```

必须满足：

1. 行首最多三个空格。
2. 连续 `#` 数量为 1 到 6。
3. `#` 后面必须是空格或 tab。

所以正文里的下面内容不会误判：

```text
C# is a language.
The value #1 remains text.
```

parser 维护六级 heading stack。例如依次遇到：

```text
# Document Ingestion
## Version State
```

正文结构路径就是：

```text
Document Ingestion / Version State
```

fenced code 支持反引号和波浪线。打开 fence 后，直到匹配的关闭 fence 才结束；文件结束仍未关闭时直接返回错误，不把残缺代码静默当成段落。

## 4. Part 3：Go 为什么使用 `go/parser`

代码：

- [golang.go](/offline-rag-go-lab/internal/documentingest/golang.go:1)

核心调用：

```go
fileSet := token.NewFileSet()
file, err := parser.ParseFile(fileSet, sourceRef, content, parser.ParseComments|parser.AllErrors)
```

标准库行为：

- `token.NewFileSet`：建立源码位置和 byte offset 映射。
- `parser.ParseFile`：把 Go 源码解析为 AST。
- `parser.ParseComments`：保留 doc comment，否则函数说明无法和声明一起切块。
- `parser.AllErrors`：尽量收集语法错误，但只要解析失败，ingestion 仍会返回错误。

`ast.FuncDecl`、`ast.GenDecl`、`ast.TypeSpec` 等节点告诉我们这是函数、import、type 还是 var。函数 receiver 会进入结构路径：

```text
package course / func Version.Activate
```

为什么不按 `strings.HasPrefix(line, "func ")`：

- 泛型、receiver、多行声明和注释会让行规则快速失效。
- 注释或字符串里也可能出现 `func`。
- Go 已经提供正式 parser，没有必要重新发明不完整语法。

## 5. Part 4：真实 token 上限如何执行

adapter：

- [token_counter.go](/offline-rag-go-lab/internal/documentingest/token_counter.go:1)

它启动时加载一次 `tokenizer.json`，每次调用 `Count` 执行真实 encode。这里没有字符数换算。

处理顺序：

1. 整个结构 unit 先计数。
2. 未超过 `max_tokens` 就保持原子性。
3. 超长段落先按句号、问号、感叹号和换行找自然边界。
4. 单句仍超限时，枚举 rune 前缀并对每个完整候选调用 tokenizer。
5. 超长代码/Go 声明优先按完整源码行组合。
6. 最终生成 `Chunk` 前再计数一次，超过上限就是硬错误。

为什么不能二分前缀：BPE merge 可能让较长文本使用更少 token，计数不保证对字符长度单调。当前实现选择最长合规前缀时会逐个实测，性能优化必须建立 benchmark，不能用错误的单调假设换速度。

## 6. Part 5：Overlap 具体怎么工作

Overlap 只用于“一个结构 unit 本身太大”的情况。

例如函数按行拆分：

```text
chunk A: function signature + if line
chunk B: if line + return line + closing brace
```

`if line` 帮助第二块保留局部上下文。

两个重要约束：

1. 普通相邻章节不机械 overlap，避免大量重复 embedding。
2. overlap 行如果无法和至少一条新行共同进入预算，就不产生只有重复内容的 chunk。

fenced code 拆分后，每个结果都会重新加 opening/closing fence，并对**带 wrapper 的完整文本**计数。不能用 `max - wrapperTokens`，因为 BPE token 数不满足简单加法。

## 7. Part 6：Chunk ID 在什么时候计算

先完成结构识别、拆分和合并，再计算稳定 ID：

```go
chunkID, err := StableChunkID(ChunkIdentityInput{
	KnowledgeScope: document.KnowledgeScope,
	DocumentID:     document.DocumentID,
	StructureKind:  unit.Kind,
	HeadingPath:    unit.HeadingPath,
	Content:        text,
})
```

如果先按原始段落计算 ID，后面又因 token 上限拆成三块，就无法让 ID 与真正写入 Qdrant 的内容一一对应。

相同内容在同一个结构路径重复出现时，按出现顺序增加 `duplicate_ordinal`；全局 `Ordinal` 只负责展示顺序，不参与身份，所以前面插入其他章节不会重命名后续未变化 chunk。

## 8. 本节真实发现的 Tokenizer 正确性问题

最初 demo 出现：非空中文段落 `tokens=0`。这不是“中文可能没有 token”，而是本地 Go tokenizer fork 的三个 bug 叠加：

1. `regexp2` 返回 rune offset，代码却当 UTF-8 byte offset 使用。
2. `PreTokenizedString.Normalize` 丢弃了已经附带 added token 的 split。
3. added-token 匹配先按位置排序又全局按 ID 重排，不同位置的 token 相互淘汰。

修复后回归结果：

```text
我 -> 1 token, id=56023
未 -> 1 token, id=73306
我叫小黄，这个项目是 Go 写的。 -> 15 tokens
版本从 pending ... 允许重试。 -> 41 tokens
```

旧 demo 曾显示第一段只有 8 tokens。该数值来自错误 tokenizer 行为，不能继续作为正确教学结论。

重要边界：当前资产来自 `~/Dolphin/hf_model/tokenizer.json`，尚未证明与 Ollama `qwen:7b` 同源。本次修复证明 Go 端不再丢 token，但“与模型官方实现完全一致”仍需要官方资产和黄金 token IDs 对照。

## 9. 测试与错误实践

执行：

```bash
go test ./internal/tokenizerdemo ./internal/documentingest
go test -race ./internal/tokenizerdemo ./internal/documentingest
```

测试覆盖：

- Markdown heading stack、段落和 fenced code
- 普通 `#` 正文不误判标题
- 未关闭 fence 硬失败
- Go doc comment + type/function 声明
- Go 独立注释和文件末尾注释不丢失
- malformed Go 硬失败
- 段落、代码和声明的 token 上限
- 真实 tokenizer 中文/added-token 回归
- 非单调 BPE、wrapper 非可加、overlap-only 边界
- 确定性 ordinal、content hash 和 chunk ID

错误实践：

```bash
go run ./cmd/document-chunk-demo \
  --config config/recent-chat.env \
  --format go \
  --source internal/documentingest/testdata/course.md
```

Markdown 被声明为 Go 时，`go/parser` 会报语法错误。系统不会在生产中猜错格式后悄悄退回纯文本。

## 10. 当前边界和生产边界

当前已支持：

- Markdown ATX heading、段落、反引号/波浪线 fenced code
- Go package/import/type/var/func 顶层结构
- qwen tokenizer 精确上限
- oversized unit 行 overlap
- 稳定 chunk identity

当前明确不支持：

- Setext Markdown 标题、表格语义和复杂嵌套列表
- HTML、PDF、Office
- Java/Python 等其他代码语言
- tree-sitter 多语言语法树
- LLM semantic chunking

这些内容不阻塞当前 ingestion 框架，记录到优化 backlog，必须由真实文档和 retrieval evaluation 决定是否追加。

## 11. 总结与重点

1. 结构化分块先识别结构，再执行 token 上限，不是反过来按字符盲切。
2. Markdown 标题规则必须明确，不能把“看起来像标题”当实现。
3. Go 使用标准 AST，doc comment 与声明一起保留。
4. token 上限对最终完整候选实测，BPE 不保证单调或可加。
5. overlap 只服务超大结构，而且不能制造只有重复内容的 chunk。
6. tokenizer 返回 0 不是估算补救问题，必须沿规则执行链修复根因。
7. 下一节才把这些 chunks、embedding、MySQL manifest 和 Qdrant 连接起来。
