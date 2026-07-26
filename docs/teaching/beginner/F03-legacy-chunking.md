# F03：旧版启发式切块

对应工程课：[03 Chunking Behavior](../03-chunking-behavior.md)。本课只观察 **18092** mock gateway 的切分器；不连接 18093 recent-chat、MySQL 或 Ollama。

## 1. 本课一句话

旧版切块器按行处理文本：识别少量标题规则，为正文加章节语境，并把超长单行按字符窗口切开。

## 2. 先用人话理解

这不是“读懂文章”的编辑，而是拿着一张规则卡的剪报员：看到 `#`、少数编号或冒号结尾行就当标题，其他行当正文。

## 3. 系统里有什么

- API：18092 的 `POST /debug/split`，只预览、不入库。
- 组件：`internal/gateway/level5_chunking/chunker.go` 的 `BuildChunks`。
- 当前规则：非空 Markdown `#` 行是标题；**任何**非空且以 `:` 或 `：` 结尾的行也是标题，不受长度限制；其余行若超过 18 个 Unicode 字符，就不会再走 `一、` 到 `五、`、`1.` 到 `3.` 的编号标题分支。
- 固定实现值：单行最多 120 个 Unicode 字符，相邻长行块重叠 20 个 Unicode 字符；这是当前代码值，不是通用 RAG 标准。

## 4. 一条完整链路

`document_id + text` → 按换行读取 → 暂存/确认标题 → 每个正文行生成 `document_id#index` → 长行滑动切分 → 返回 chunk 预览。`/debug/split` 的响应只有 `document_id`、`chunks[].chunk_id`、`chunk_index` 和 `text`：它能证明切出了哪些正文，**不能直接显示或证明标题语境**。此接口不写内存 store、不写原文文件。

## 5. 实际怎么做

启动 18092 gateway 后执行：

```bash
curl -X POST http://127.0.0.1:18092/debug/split -H 'Content-Type: application/json' -d '{"document_id":"f03-guide","title":"退款指南","text":"# 退款\n提交订单号。\n申请材料：\n保留付款凭证。"}'
```

- 等级：纯本地、只读式预览；不持久化业务数据。
- 依赖：Go gateway 正在运行。
- 重复执行：安全，输入相同则返回相同切分；没有需要恢复的副作用。

## 6. 结果怎么看

先查看 `chunks`：`chunk_id` 从 `f03-guide#0` 起递增，`text` 是实际正文。标题行通常不作为独立正文 chunk 返回；但这里不能从响应看见标题语境，不能据此断言标题是否已合成。若只返回错误，先检查 `document_id` 与 `text` 是否为空。

要验证标题语境，先在**同一个仍在运行的 18092 进程**准备一份独立测试文档：

```bash
curl -X POST http://127.0.0.1:18092/ingest -H 'Content-Type: application/json' -d '{"document_id":"f03-title-verify","title":"退款指南","source_ref":"f03-title-demo","text":"申请材料：\n保留付款凭证。","tags":["f03-demo"]}'
```

- 等级：纯本地业务写入；写入该进程内存中的 `f03-title-verify#…` chunk，并写入本地 `storage/docs/f03-title-verify.txt`。
- 依赖：18092 gateway 正在运行；不依赖 MySQL、Ollama 或 18093 recent-chat。
- 重复执行：可以重复，但同 ID 的内存 chunk 与原文文件会被覆盖，不是 no-op；覆盖前内容不能自动恢复，需自行保存源文本。

随后执行下面的只读观察命令：

```bash
curl 'http://127.0.0.1:18092/debug/retrieval?question=%E4%BB%98%E6%AC%BE%E5%87%AD%E8%AF%81'
```

- 等级：纯本地、只读；只读取该进程的内存 store，不写原文、日志或知识数据。
- 依赖：上一步导入已成功且 gateway 尚未重启；重启后内存 store 会清空，需要重新导入。
- 重复执行：安全；输入和内存内容不变时，命中结构应保持一致。

这个真实响应的可观察字段是 `hits[].title`（不是 `/debug/split` 的字段）。若命中刚导入的“保留付款凭证。”正文，`title` 应把文档标题与识别到的章节组合为类似 `退款指南 / 申请材料`；若没有命中，先确认导入响应和查询词，再把“标题语境已写入 chunk”仅视为代码实现说明。

## 7. 算一遍或走一遍

对 `/debug/split` 示例来说，`# 退款` 和 `申请材料：` 都不是正文块；两行正文分别产生 `f03-guide#0`、`f03-guide#1`，所以预期两块而非四块。这是接口可以直接验证的结论。

标题语境的走法则是实现说明：`# 退款` 在下一段正文前成为当前章节，`申请材料：` 随后更新当前章节；本节独立导入后的 `/debug/retrieval` 才能通过 `hits[].title` 观察组合结果。这样把“切分数量”与“标题是否进入存储 chunk”分成两条证据，不用 preview 接口去证明它没有返回的字段。

## 8. 常见误解

“超过 18 个字符就绝不会被当标题”是错的：很长但以 `：` 或 `:` 结尾的非空行仍会先被识别为标题。18 字符限制只约束后续的 `一、` 到 `五、`、`1.` 到 `3.` 分支；例如“第一章 退款规则”仍未必命中。删掉标题语境会让相同正文在检索结果中失去所属章节信息。

## 9. 当前实现与生产边界

已保证：简单、可重复、避免按字节截断中文的基础行为。未保证：Markdown AST、代码语法、语义边界、token 上限和稳定跨版本身份。生产型 Markdown/Go 结构化切块在 L30，不能把本课的 120/20 字符参数当作生产策略。

## 10. 面试怎么说

30 秒版：chunking 的目标是把全文变成可检索单元；这个 demo 用规则识别标题、按行切分和重叠窗口处理长行，优点是可解释，缺点是不能真正理解文档结构。展开点：标题为何影响检索语境、字符和 token 的区别、为什么需预览接口。

## 11. 自检题与答案

问：`/debug/split` 会把文本导入知识库吗？答：不会，它只返回预览。问：120 是 token 上限吗？答：不是，是当前实现的 Unicode 字符窗口。

## 12. 事实锚点

- 原课：[03-chunking-behavior.md](../03-chunking-behavior.md)
- 入口：`cmd/rag-gateway/main.go` 的 `/debug/split`
- Go 组件：`internal/gateway/level5_chunking/chunker.go`
- 测试：`internal/gateway/level5_chunking/chunker_test.go`、`internal/gateway/boss/gateway_test.go`
