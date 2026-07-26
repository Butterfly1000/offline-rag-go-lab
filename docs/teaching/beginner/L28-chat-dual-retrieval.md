# L28：把双路召回放进真实 `/chat`

对应工程课：[Retrieval 小节 28](../recent-chat-dual-retrieval-sop.md)。本课把 memory、document、retrieved-context 子预算与 recent-chat 的总预算编排在一起。

## 1. 本课一句话

开启 retrieval 的 `/chat` 必须使用自动 token 预算：先检索/合并/计 retrieved context，再把它加入 fixed input 计算 recent 历史额度，成功回答后才按开关写 MySQL turn。

## 2. 先用人话理解

先把查到的资料放进系统提示，再称整份固定输入；若先算历史空间、后塞资料，就会把已经占用的上下文当成空位。

## 3. 系统里有什么

- 请求：`use_memory`、`use_knowledge`、`knowledge_scope`、`memory_limit`、`document_limit`、`context_token_budget`。
- 前提：任一路开启都要求 `auto_token_budget=true`；memory 要正 limit，knowledge 要 scope 和正 document limit，retrieval 子预算也必须为正。
- 响应：retrieved hits、`used_memory_items`、`used_document_chunks`、`used_context_tokens`、`retrieval_warnings`。
- 当前 document 默认 collection：`offline_rag_document_chunks_v1`。

## 4. 一条完整链路

校验请求 → L26 一次 embedding/并发检索/ownership 重验 → L27 merge、escape 与 context 子预算 → 将 rendered context 合入 system fixed input → automatic plan → 可选 summary/recent strict window → Ollama `/api/chat` → 成功后按 `store_user_turn` 与 `store_assistant_turn` 分别写 MySQL。完整性失败在 Ollama/MySQL 写入前终止，基础设施 warning 可携带到响应。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/recentchat ./internal/contextretrieval -run 'Test(ChatRequestRetrievalValidationAndOldRequestCompatibility|ServiceRetrieval|DualRetriever|Merge)' -count=1
curl -sS --max-time 120 -X POST http://127.0.0.1:18093/chat -H 'Content-Type: application/json' -d '{"session_id":"dual-retrieval-demo-001","user_id":"memory-store-demo-user-20260712-a","message":"这个项目使用什么语言，教学要求是什么？","model":"qwen:7b","recent_limit":10,"auto_token_budget":true,"output_token_reserve":128,"use_memory":true,"use_knowledge":true,"knowledge_scope":"offline-rag-course","memory_limit":3,"document_limit":3,"context_token_budget":512,"store_user_turn":true,"store_assistant_turn":true}'
```

- test 覆盖 retrieval 请求校验/旧请求兼容、recent-chat service 编排，以及 dual retriever/merge；它使用本地 fake，安全重复，仅写 build cache。
- curl 依赖运行中的 recent-chat、MySQL、Ollama、tokenizer、memory/document Qdrant fixture 与私有配置（从 `config/recent-chat.env.example` 派生）；它调用模型并在成功后写一轮 MySQL user/assistant。每次专用 session 可重复但新增记录；恢复前按精确 session/user 检查。

## 6. 结果怎么看

成功响应会分别计 memory/document hit 和 context token，并不返回 embedding vector。scope 内没有 document 命中是正常空结果；document 超时/404 等基础设施故障可给 warning 后继续，跨 user/scope 或畸形 payload 则没有 answer、也不写本轮 turn。

## 7. 算一遍或走一遍

假设 L27 rendered context 为 120 token，system 原本 30、本轮问题 20、输出 reserve 100、模型 context 1000，则 automatic fixed 输入包含 context 后为 170，recent 额度为 `1000-170-100=730`。若先按 30+20 算出 850，再塞 120 context，就会错误多留 120 历史空间。

## 8. 常见误解

“`context_token_budget` 就是模型总预算”错误，它仅限制 retrieved_context 子块。“关掉 store flag 就没有任何写入”不总是成立：这里只关闭 turn 才不写消息，其他启用能力可能有自身写入。“L29–L33 发布 alias 后 chat 自动读新集合”错误；当前默认仍是 `offline_rag_document_chunks_v1`，除非显式改 `QDRANT_DOCUMENT_COLLECTION`。

## 9. 当前实现与生产边界

已保证 retrieval 在自动 fixed-input plan 前、服务层再次核对 ownership、warnings 可观测、MySQL 写发生在主回答成功后。未保证 user/scope 已经来自认证授权；当前 API 仍从请求 JSON 取值。L29–L33 的 ingestion alias 是另一条关系链，默认不会自动接入此 chat collection。

## 10. 面试怎么说

我先把双路召回合并成安全渲染的 retrieved context，再把它算进 fixed input，之后才算 recent history 预算；这样每一块都被同一上下文总账覆盖。数据完整性失败 fail closed，基础设施失败可以 warning 降级，成功回答后才落 MySQL turn。

## 11. 自检题与答案

问：开启 use_knowledge 可用 manual budget 吗？答：不能，必须 automatic。问：retrieved context 何时加入 system？答：自动 plan 前。问：当前 chat 默认读 ingestion alias 吗？答：不读，默认 `offline_rag_document_chunks_v1`。

## 12. 事实锚点

- 原课：[recent-chat-dual-retrieval-sop.md](../recent-chat-dual-retrieval-sop.md)
- Go 组件：`internal/recentchat/{types,service,http}.go`
- 前置：[L24](L24-context-hit-boundary.md)–[L27](L27-context-merge-budget.md)
