# L30：按 Markdown/Go 结构切块并强制真实 token 上限

对应工程课：[Ingestion 小节 30](../structured-document-chunking-sop.md)。本课先识别可解释结构，再用本地 tokenizer 确保每个 chunk 真不超限。

## 1. 本课一句话

Markdown 只识别 ATX heading 与 fenced code，Go 必须能被 parser 解析；所有结构单元都经真实 tokenizer 强制 `max_tokens`，超大单元才按规则拆分并可带行 overlap。

## 2. 先用人话理解

切块不是每 500 个字符剪一刀，而是先看“这是章节、段落还是代码声明”。结构足够大时再细分，但每份都必须真的量过 token，不能凭字符数猜。

## 3. 系统里有什么

- Markdown：最多三级前导空格的 ATX `#`–`######` heading、反引号/波浪线 fenced code、普通段落；不支持所有 Markdown 方言。
- Go：`go/parser.ParseFile` 成功后按 preamble/声明结构切，语法不可解析即失败。
- policy：`MaxTokens`、`OverlapLines`；每个输出有结构路径、ordinal、content hash、stable chunk ID、实际 token count。
- counter：仓库本地 Qwen tokenizer fork 通过 `NewQwenTokenCounter` 加载资产。

## 4. 一条完整链路

规范化 document → 按 format 解析结构 → 每个 structural unit 先真实计 token → 小于上限保留，超大 paragraph 按句/精确文本分割，超大 code/Go 单元按行分割并可重叠完整行 → 再 pack 兼容段落 → 对最终 chunk 再计 token 并拒绝超限 → 用 L29 identity 生成 stable ID。fence 未闭合或 Go parser 失败不会降级为随便按字符切。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/documentingest -run 'Test(Markdown|Go|ChunkDocument)' -count=1
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/document-chunk-demo --config /absolute/path/to/your/recent-chat.env --format markdown --source internal/documentingest/testdata/course.md --max-tokens 160 --overlap-lines 2
```

- test 是纯本地 fake counter，不需 MySQL/Ollama/Qdrant；只写 build cache，可重复。
- demo 读取本地 source 与由 `config/recent-chat.env.example` 派生配置指向的 tokenizer 资产，不写业务数据，可重复。tokenizer 资产不存在/加载失败会停止；本地 tokenizer fork 的计数与上游 runtime 的长期一致性仍需黄金集对照，不能把 fork 当自动等价证明。

## 6. 结果怎么看

每个输出显示 `kind`、heading path、chunk ID 和 tokens，且每个 token count 不超过 max。fenced code 作为 code unit 保留 fence；过大 fence 的分片仍保留语言 marker 和闭合 fence。普通 `C#` 或 `#1` 不因含 `#` 自动成为 heading。

## 7. 算一遍或走一遍

max=30、overlap=1 的超大 code fence 可分为两份，第二份重带前一份末尾的一整行和相同 ` ```go`/闭合 fence；若 overlap 本身没有留下新行，算法丢弃重复-only 片段以继续前进。若一枚 rune 连包装后都超过 max，则失败，不产生违规 chunk。

## 8. 常见误解

“所有 Markdown heading 都支持”错误，当前只 ATX。“Go 有文本就能切”错误，必须可解析。“overlap 是任意重复字符”错误，当前是行级规则。“tokenizer fork 只要加载成功就等于官方结果”错误，本地实现/资产升级应以固定黄金样例与上游对照。

## 9. 当前实现与生产边界

已保证：真实 counter 强制最终 token ceiling、稳定 ID、非负 overlap、未闭合 fence/不可解析 Go 失败。未保证：CommonMark 全覆盖、其他语言 AST、语义 chunking，或本地 tokenizer fork 与任一外部 runtime 的永久一致；资产/实现变化需重新验证 token 基线。

## 10. 面试怎么说

我先按可信结构切，再用真实 tokenizer 把每个最终 chunk 卡在硬上限内。Markdown 只承诺 ATX/fence，Go 必须 AST 可解析；overlap 是完整行而非盲复制，chunk identity 来自结构和内容而非全局位置。tokenizer fork 是可验证依赖，不是无需比对的真理。

## 11. 自检题与答案

问：未闭合 code fence 怎么办？答：失败。问：最终 token 上限只在初始段落计吗？答：不是，每个最终 chunk 都重计。问：chunk ID 含全局行号吗？答：不含。问：本地 fork 需要何时对照上游？答：资产或实现升级时。

## 12. 事实锚点

- 原课：[structured-document-chunking-sop.md](../structured-document-chunking-sop.md)
- Go 组件：`internal/documentingest/{chunker,markdown,golang,token_counter}.go`
- 演示：`cmd/document-chunk-demo/main.go`
- 前置：[L29](L29-document-identity.md)，后续：[L31](L31-idempotent-ingestion.md)
