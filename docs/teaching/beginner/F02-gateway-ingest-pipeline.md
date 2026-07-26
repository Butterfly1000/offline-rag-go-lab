# F02：旧版 mock `/ingest` 流水线

对应工程课：[02 Ingest Behavior](../02-ingest-behavior.md)。本课仍是 **18092** 的本地 mock gateway，不是 18093 `recent-chat`，不需要 MySQL、Ollama 或 Qdrant。

## 1. 本课一句话

`/ingest` 将一篇文本变成可检索 chunk，同时把原文保存到本地，形成“使用形态 + 追溯形态”。

## 2. 先用人话理解

像图书管理员把一本书拆成索引卡，并把原书放进档案柜：索引卡用于快速找内容，原书用于日后核对或重新整理。

## 3. 系统里有什么

- HTTP 入口：`cmd/rag-gateway/main.go` 的 `POST /ingest`，端口 18092。
- 主动作：`App.IngestText`；切块器在 `internal/gateway/level5_chunking`。
- 存储：默认 `MemoryKnowledgeStore`；原文写到 `storage/docs/{document_id}.txt`。
- 输入：`document_id`、`title`、`source_ref`、`text`、`tags`；输出含 `chunk_count` 和 `status`。

## 4. 一条完整链路

请求正文 → `BuildChunks` → 按 `chunk_id` 写入内存 store → 原文写入 `storage/docs` → 返回导入摘要。顺序很重要：内存 Upsert **先于**原文写文件。若写文件失败，HTTP 会返回错误，但本次内存 chunk 已可能生效；这不是原子事务。内存 chunk 可立即给同一进程的 `/chat` 检索；重启后不保留，但原文文件仍在。

## 5. 实际怎么做

先按 F01 启动 `go run ./cmd/rag-gateway`，再执行：

```bash
curl -X POST http://127.0.0.1:18092/ingest -H 'Content-Type: application/json' -d '{"document_id":"f02-guide","title":"退款指南","source_ref":"f02-demo","text":"# 退款\n提交订单号后等待审核。","tags":["demo"]}'
```

- 等级：纯本地业务写入。
- 依赖：18092 gateway 正在运行；不依赖外部服务。
- 影响：内存 `f02-guide#…` chunk 与本地 `storage/docs/f02-guide.txt`。
- 重复执行：同 `chunk_id` 会覆盖内存内容，原文文件也会覆盖；不是 no-op。它不是“按 document_id 整体替换”：若旧版本曾有 `f02-guide#2`，新版本只产生 `#0`，旧 `#2` 仍会留在内存 store。覆盖前文件内容及残留旧高序号 chunk 都不会自动恢复/清理，需自行保留源文本并重启服务或采用其他受控清理方式。

## 6. 结果怎么看

`status:"ok"` 表示 chunk 已写入内存且原文已成功落盘；`document_id` 应回显输入 ID；`chunk_count` 是本次产生的 chunk 数，不是向量数。400 且提示 `document_id is required` 或 `text is required` 时，先修正输入。若错误来自原文落盘，不能据此断言“内存没有变”，因为 Upsert 已先执行；没有真实 embedding 调用并不代表失败。

## 7. 算一遍或走一遍

示例有一个 Markdown 标题和一行正文。标题用于给后续正文补充章节语境，正文产出至少一个 chunk，因此 `chunk_count` 预期为 1。标题本身不是一条独立的“知识答案”；它是正文 chunk 的上下文标签。

## 8. 常见误解

“重导同一 `document_id` 就是安全幂等整体覆盖”是错的。默认 store 只按 `chunk_id` 覆盖，缩短后的文档不会自动删除旧高序号 chunk；而且原文写失败后可能留下内存部分生效。默认 store 也会在服务重启后清空，若只删除原文文件并不会清空当前进程的内存 chunk；两种形态职责不同。

## 9. 当前实现与生产边界

已保证：输入校验、确定性的基础 chunk ID、同进程检索和成功落盘后的原文追溯。未保证：内存与文件的原子性、按 document 完整替换、向量 embedding、权限/scope、版本与跨进程持久化。L29–L33 才讲生产型身份、结构化切块、幂等入库与发布。

## 10. 面试怎么说

30 秒版：ingest 不等于“把文件扔给模型”，而是把原文转换为可检索 chunk，并保留原文以便追溯。本 demo 先 Upsert 内存再写文件，因此失败时可能部分生效；它只按 chunk ID 覆盖，不能当作按文档完整替换。展开点：为什么同时保留两种数据、残留高序号 chunk、为什么 chunk 数不等于 embedding 数。

## 11. 自检题与答案

问：重启 18092 后还能检索刚导入的 chunk 吗？答：默认不能，因为 store 是内存。问：重跑同一 `document_id` 会删除旧的高序号 chunk 吗？答：不会；只覆盖这次出现的相同 chunk ID。问：原文写文件失败时内存一定没变吗？答：不一定，Upsert 在前。

## 12. 事实锚点

- 原课：[02-ingest-behavior.md](../02-ingest-behavior.md)
- 入口：`cmd/rag-gateway/main.go`
- Go 组件：`internal/gateway/level2_hq/app.go`、`level5_chunking/chunker.go`、`level3_store`
- 测试：`internal/gateway/boss/gateway_test.go` 的 ingest/retrieval 用例
