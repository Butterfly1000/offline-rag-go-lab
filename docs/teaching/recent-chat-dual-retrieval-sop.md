# 第 28 节：Dual Retrieval 接入真实 Recent Chat

本节把第 24-27 节接入真实 `/chat`，形成 recent、memory、document、token budget、Ollama 与 MySQL message store 共存的完整链路。

## 1. 完整执行顺序

```text
校验请求
-> 一次 embedding，并行检索 memory/document
-> ownership 重验与故障分类
-> 来源独立排序、quota、去重
-> retrieved_context 精确 token 子预算
-> 把 retrieved_context 合并进 system fixed input
-> 自动计算剩余 recent history 预算
-> 可选 session summary
-> 严格裁剪 recent window
-> Ollama /api/chat
-> 成功后按开关写 MySQL user/assistant turn
```

关键顺序是 retrieval 在自动 fixed-input plan 之前。否则历史预算不知道召回内容已经占用多少 token。

## 2. 请求新增字段

```json
{
  "use_memory": true,
  "use_knowledge": true,
  "knowledge_scope": "offline-rag-course",
  "memory_limit": 3,
  "document_limit": 3,
  "context_token_budget": 512
}
```

规则：

- 任一 retrieval 开启时必须使用 `auto_token_budget`
- memory 开启时 `memory_limit > 0`
- knowledge 开启时 scope 非空且 `document_limit > 0`
- retrieval 开启时 `context_token_budget > 0`
- 两个开关都为 false 时，旧请求无需新增字段，行为保持兼容

`context_token_budget` 只限制安全渲染后的 retrieved context；完整上下文仍由模型
context limit、fixed input、recent history 和 output reserve 共同约束。

## 3. 启动 SOP

确认 MySQL、Ollama、Qdrant 已启动，且本机配置包含：

```text
OLLAMA_BASE_URL=http://127.0.0.1:11434
OLLAMA_EMBED_MODEL=bge-m3
QDRANT_BASE_URL=http://127.0.0.1:6333
QDRANT_MEMORY_COLLECTION=offline_rag_memory_items_v1
QDRANT_DOCUMENT_COLLECTION=offline_rag_document_chunks_v1
RECENT_CHAT_TOKENIZER_PATH=assets/tokenizers/qwen2/tokenizer.json
```

启动：

```bash
go run ./cmd/recent-chat --config config/recent-chat.env --addr :18093
```

启动只构造客户端并监听 HTTP，不创建 collection、不写 point、不执行 schema。文档集合
创建与 fixture 写入仍由第 25 节显式 `--apply` demo 负责。

## 4. 正常 Scope 真实 Curl

本机 `qwen:7b` 在较大的回答上限下可能超过 HTTP client 的 60 秒生成超时。教学验证
使用 `output_token_reserve=128`，retrieval 子预算仍为 512：

```bash
curl -sS --max-time 120 -X POST http://127.0.0.1:18093/chat \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id":"dual-retrieval-chat-stored-003",
    "user_id":"memory-store-demo-user-20260712-a",
    "message":"这个项目使用什么语言，教学要求是什么？",
    "model":"qwen:7b",
    "recent_limit":10,
    "auto_token_budget":true,
    "output_token_reserve":128,
    "use_session_summary":false,
    "use_memory":true,
    "use_knowledge":true,
    "knowledge_scope":"offline-rag-course",
    "memory_limit":3,
    "document_limit":3,
    "context_token_budget":512,
    "store_user_turn":true,
    "store_assistant_turn":true
  }'
```

实际关键结果：

```text
used_memory_items: 1
used_document_chunks: 2
used_context_tokens: 330
retrieval_warnings: null
answer: 非空
```

返回的 memory Hit 只有请求用户 `memory-store-demo-user-20260712-a`；两个 document Hit
都属于 `offline-rag-course`。响应不包含向量。

## 5. 确认 MySQL 两条消息

对同一 session 再发送不写入的短请求：

```bash
curl -sS --max-time 120 -X POST http://127.0.0.1:18093/chat \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id":"dual-retrieval-chat-stored-003",
    "user_id":"memory-store-demo-user-20260712-a",
    "message":"只回复ok",
    "model":"qwen:7b",
    "recent_limit":10,
    "auto_token_budget":true,
    "output_token_reserve":32,
    "store_user_turn":false,
    "store_assistant_turn":false
  }'
```

实际 `used_messages=2`，`recent_window` 包含上一请求的 user ID 30 与 assistant ID 31，
证明成功回答后两条消息都已进入 MySQL。

## 6. 不存在 Scope 的行为

把 `knowledge_scope` 改成 `missing-course`，其他 retrieval 字段不变。实际结果：

```text
answer: Go
used_memory_items: 1
used_document_chunks: 0
used_context_tokens: 76
retrieval_warnings: null
```

“scope 内没有命中”不是服务故障，所以没有 warning；memory 仍能提供项目语言。

## 7. Document 基础设施故障降级

用被 Git 忽略的临时配置把 document collection 指向不存在的
`offline_rag_document_chunks_unavailable_test`，在 `:18094` 启动实例。实际结果：

```text
answer: Go
used_memory_items: 1
used_document_chunks: 0
retrieval_warnings: [document infrastructure failure: ... status 404 ...]
```

Document 404 被分类为基础设施错误，因此保留 memory 并继续 Ollama。测试后本机配置
已恢复为 `offline_rag_document_chunks_v1`，两个教学服务进程均已停止。

跨 user、跨 scope、畸形 payload 不走该降级；它们是 IntegrityFailure，会在 Ollama
和 MySQL 写入前硬失败。

## 8. 代码入口

- `internal/recentchat/types.go`：请求校验与响应观测字段
- `internal/recentchat/service.go`：retrieve -> merge -> budget -> summary -> recent -> chat 编排
- `internal/recentchat/http.go`：把 `r.Context()` 传给 `ChatContext`
- `cmd/recent-chat/main.go`：真实 Ollama/Qdrant 依赖注入
- `internal/recentchat/service_retrieval_test.go`：服务边界、降级、summary 组合与预算测试

`Chat(req)` 仍使用 `context.Background()` 包装 `ChatContext`，保持现有 Go 调用兼容；HTTP
路径则能把客户端取消传给 embedding/Qdrant retrieval。

## 9. 当前实现与生产级边界

当前实现已是真实本地链路，但 API 仍是教学服务：`user_id` 和 `knowledge_scope` 来自请求
JSON。生产中必须从认证身份和服务端授权关系生成，不能信任客户端自报 ownership。

其他生产增强：

- Ollama chat API 也改为 context-aware，客户端断开时取消生成
- 每来源独立 timeout、metrics、trace、熔断与结构化 warning code
- 文档 ingestion worker、版本删除、alias 重建和检索质量评估
- 记录本地 token 计划与 Ollama `prompt_eval_count` 的误差

这些不改变本节主框架。

## 10. 本节重点

1. Retrieval context 必须在自动 fixed-input 规划前加入 system prompt。
2. 服务层仍要二次核对 user/scope，不能只相信下层 searcher。
3. 只有基础设施故障可降级；ownership/数据完整性失败必须阻止生成和写消息。
4. 响应返回 Hit、来源计数、token 和 warning，不返回 embedding vector。
5. 第 24-28 节已实现并验证，但只有用户确认“懂了”后才能记为已学会。
