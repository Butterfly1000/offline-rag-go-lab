# F01：旧版 mock `/chat` 流水线

对应工程课：[01 Chat Behavior](../01-chat-behavior.md)。这是早期教学 gateway，监听 **18092**；不是后续 MySQL + Ollama 的 18093 `recent-chat` 服务。

## 1. 本课一句话

它把一次提问按“校验 → 可选检索 → 压缩 → mock 回答 → JSONL 日志”串成可观察的本地流水线。

## 2. 先用人话理解

把它看作接线员：先确认来电信息完整，再按需翻本地资料，缩短资料，交给演示用答复器，并把本次处理记入档案。

## 3. 系统里有什么

- 启动入口：`cmd/rag-gateway/main.go`，固定端口 `:18092`。
- 编排器：`internal/gateway/level2_hq/app.go` 的 `App.Chat`。
- 默认能力：内存知识库、简单检索/压缩、`mock-chat` 生成器、JSONL 日志器；不需要 MySQL、Ollama 或 Qdrant。
- 输入：`session_id`、`user_id`、`question`、可选 `model` 与 `use_knowledge`。
- 输出：`answer`、`used_knowledge`、`retrieved_chunks`、`latency_ms`。

## 4. 一条完整链路

`POST /chat` → 检查三个必填 ID/问题 → `use_knowledge=true` 时查内存 chunk → 限制数量和长度 → mock 生成 answer → 写入 `storage/logs/` 的 JSONL → 返回 JSON。

数据只落在本机日志目录；内存知识在进程退出后消失。

## 5. 实际怎么做

先启动：

```bash
go run ./cmd/rag-gateway
```

- 等级：纯本地；不连接外部服务。
- 依赖：Go 与仓库；启动会创建 `storage/logs`、`storage/docs`。
- 重复执行：可以；已有目录不会造成业务重复。

另开终端发送：

```bash
curl -X POST http://127.0.0.1:18092/chat -H 'Content-Type: application/json' -d '{"session_id":"f01-s1","user_id":"f01-u1","question":"这个 gateway 做什么？","use_knowledge":false}'
```

- 等级：纯本地，但会向 `storage/logs` 追加一行 JSONL。
- 重复执行：会新增日志行，不是 no-op；若要清理，先人工备份再处理该本地日志文件。

## 6. 结果怎么看

HTTP 200 且 `answer` 非空说明流水线完成。`used_knowledge=false` 是本例的正常结果；`retrieved_chunks=[]` 表示没有把资料送入 mock 生成器。400 常见原因是三个必填字段之一为空；先检查请求 JSON，而不是怀疑模型。

## 7. 算一遍或走一遍

本例 `use_knowledge=false`，所以检索步骤被跳过：命中数 `0`，进入回答器的 chunk 数 `0`，因此 `used_knowledge=false`。即使 answer 有文本，也不代表调用了真实大模型：默认是 mock。

## 8. 常见误解

“看到 `model` 字段就已经调用 Ollama”是错的。这里默认模型名主要用于记录；删除 mock 生成器或换成真实客户端才会产生真实模型调用。把它误当生产 chat，会错误估计外部依赖和数据持久化。

## 9. 当前实现与生产边界

已保证：请求校验、可选内存检索、日志与响应结构。未保证：真实模型、跨进程知识持久化、会话记忆、token 预算。生产通常还需真实检索、权限隔离、重试和可观测性；后续 18093 服务是另一条演进链，不是本服务的同端口升级。

## 10. 面试怎么说

30 秒版：这是一个分层 RAG gateway 的最小演示，`App.Chat` 只编排校验、检索、压缩、生成和日志；默认实现均可替换，因此能先用 mock 验证链路。展开点：为什么检索可选、为什么压缩在生成前、为什么日志失败会让本次请求失败。

## 11. 自检题与答案

问：`answer` 非空能证明使用了知识库吗？答：不能；要同时看 `used_knowledge` 和 `retrieved_chunks`。问：数据会写入 MySQL 吗？答：不会；这一课只写本地 JSONL 日志。

## 12. 事实锚点

- 原课：[01-chat-behavior.md](../01-chat-behavior.md)
- 入口：`cmd/rag-gateway/main.go`
- Go 组件：`internal/gateway/level2_hq/app.go`、`level3_store`、`level4_retrieval`、`level6_compression`
- 测试：`internal/gateway/boss/gateway_test.go` 的 chat、日志与检索用例
