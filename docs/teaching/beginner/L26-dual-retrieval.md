# L26：一次 embedding，并发检索 memory 和 document

对应工程课：[Retrieval 小节 26](../dual-retrieval-sop.md)。本课复用同一个 query vector，同时保留两条来源的 ownership 与故障边界。

## 1. 本课一句话

同一个 query 只 embedding 一次，然后并发查询 memory（user_id）和 document（knowledge_scope）；embedding/基础设施问题可 warning 降级，越权或畸形 payload 必须硬失败。

## 2. 先用人话理解

用户问题只需量一次“语义坐标”，再把同一坐标交给个人记忆目录和课程文档目录同时查。目录临时打不开可以说明“这路没查到”；若目录把别人的卡片递来，则不能假装正常继续回答。

## 3. 系统里有什么

- `DualRequest`：query、user、scope、启用来源及各自 limit。
- `DualRetriever`：一次 `Embed([query])`，再启动 memory/document 两个 goroutine。
- `DualResult`：分开的 `MemoryHits`、`DocumentHits` 与 `Warnings`。
- ownership：memory 只接受请求 user；document 只接受请求 scope，均复用 L24 `ValidateHit`。

## 4. 一条完整链路

校验 request → 对 `[]string{query}` 调用一次 embedding → 若 embedding 调用失败或返回的向量数量/数值不合法，返回 warning 且不发起 Search → 启用的 memory/document 用同一 vector 并发查询 → 收齐结果后固定顺序处理：infrastructure error 加 warning，成功结果逐条 ownership/payload 重验，integrity error 立即硬失败。没有在本课把两路 hit 按 score 混成最终 prompt。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/contextretrieval -run 'Test(DualRetriever|MemoryQdrantSearcher|DocumentQdrant)' -count=1
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/dual-retrieval-demo --config /absolute/path/to/your/recent-chat.env
```

- test 是纯本地 fake embedder/searcher 验证，不访问 Ollama/Qdrant/MySQL，不写业务数据；只写 build cache，可安全重复。
- demo 依赖已完成的 L23 memory fixture、L25 document fixture、Ollama `bge-m3`、Qdrant 和由 `config/recent-chat.env.example` 派生的私有配置。它只调用 embedding 与 Qdrant query，不写 MySQL/Qdrant；可安全重复。standalone demo 要求双路均健康，任何 warning 即退出；真实 chat 可将 warning 带入响应继续工作。

## 6. 结果怎么看

正常 demo 输出 `Query embeddings: 1`，并分别列出 Memory hits 和 Document hits，warning 数为 0，且无 cross-user/cross-scope hit。一次 embedding 不是“每个来源各调用一次恰好相同”，而是代码只执行一笔 query embedding 请求并复用返回 vector。

## 7. 算一遍或走一遍

query 为“项目使用什么语言，聊天历史如何按 token 预算处理？”，两路均启用时 embedder 收到一个文本数组 `[query]`，产生一个 vector；memory 与 document 都拿这一个 vector。memory timeout + document 成功时，结果保留 document hits 和一条 warning；若 memory 返回 `user_id=u-2` 给请求 `u-1`，即使 document 成功，整个 Retrieve 返回错误。

## 8. 常见误解

“embedding 失败必须让整个 chat 失败”错误，当前返回 warning、两路均不查。“Qdrant HTTP 500 与跨 user payload 都可降级”错误，前者是 infrastructure，后者是 integrity。“memory/document raw score 可直接比较后取最高”错误：两 collection 的 score 分布未校准，本课不设 score threshold，也不跨 collection 排序；这由 [L27](L27-context-merge-budget.md) 处理。

## 9. 当前实现与生产边界

已保证：query 只 embed 一次、远程两路并发、embedding/infra failure 以 warning 降级、跨 user/scope/未知 source/畸形 payload 为 hard failure。未保证：每来源独立 timeout、熔断、指标，或 raw score 的跨 collection 可比性；当前也没有 score threshold，L27 才负责确定性 merge、顺序和 token budget。

## 10. 面试怎么说

我把 query embedding 作为共享输入，一次生成后并发检索两个独立 collection。结果始终按 memory user ownership 与 document scope ownership 分开验证；可用性故障转换 warning，数据隔离/完整性故障 fail closed，且不假装跨 collection 的 raw score 可以直接比较。

## 11. 自检题与答案

问：两路都启用时 query embedding 调几次？答：一次。问：embedding 返回 NaN vector 怎么办？答：warning，且不 Search。问：document 返回其他 scope 能降级吗？答：不能，硬失败。问：当前有统一 score threshold 吗？答：没有，留给 L27。

## 12. 事实锚点

- 原课：[dual-retrieval-sop.md](../dual-retrieval-sop.md)
- Go 组件：`internal/contextretrieval/{dual,memory_adapter,document_qdrant}.go`
- 演示：`cmd/dual-retrieval-demo/main.go`
- 前置：[L23](L23-memory-qdrant.md)、[L24](L24-context-hit-boundary.md)、[L25](L25-document-qdrant.md)
- 后续：[L27](L27-context-merge-budget.md)
