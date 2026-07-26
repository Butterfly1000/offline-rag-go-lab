# L23：把 active memory 建成用户隔离的向量索引

对应工程课：[Memory 小节 23](../memory-item-qdrant-sop.md)。本课用 embedding 为 MySQL 中仍 active 的长期记忆建立可重建的 Qdrant 索引。

## 1. 本课一句话

qwen 负责生成/提取文本，embedding 模型负责向量；本课本机 `bge-m3` 示例预期 1024 维 Cosine，查询强制 `user_id` filter 并在返回后再次核验。

## 2. 先用人话理解

qwen 像会写字的助手，bge-m3 像把句子变成坐标的尺子；不能把两者混作同一模型。MySQL 是档案柜，Qdrant 是按“意思相近”查索引卡的目录，目录丢了可由档案柜重建。

## 3. 系统里有什么

- qwen：chat、摘要、candidate 提取等生成工作。
- `bge-m3`：本机示例的 Ollama `/api/embed` 模型，实测/预期可返回 1024 维向量。
- collection：`offline_rag_memory_items_v1`；本机 bge-m3 示例使用 `size=1024`、`distance=Cosine`。
- point payload：`user_id`、MySQL item ID、kind/key/value/version、embedding model。

## 4. 一条完整链路

MySQL 读取 active item → 稳定文本如 `project_fact/implementation_language: Go` → 配置的 embedding 模型生成向量 → 校验数量、非空、同维度和每个值有限 → 将本次实际响应维度传给 `EnsureCollection` → 用 MySQL item ID upsert point。已有 collection 只核对其 size 是否等于本次维度且 distance 为 Cosine。forgotten item 不写入，反而按 item ID 删除已有 point；查询时先带 `user_id` filter，再精确重验返回字段。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/memoryitem -run 'Test(Qdrant|HTTPOllamaEmbedder)' -count=1
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/memory-qdrant-demo --config /absolute/path/to/your/recent-chat.env --ensure-collection
```

- test 是纯本地 HTTP fake 验证，不访问真实 MySQL/Ollama/Qdrant，不写业务数据；只写 build cache，可安全重复。
- demo 需要 L22 的 primary fixture、MySQL、配置的 Ollama embedding 模型、Qdrant 和由 `config/recent-chat.env.example` 派生的私有配置。`--ensure-collection` 可能创建 collection 与 payload indexes，并按实际 embedding 响应维度 ensure/upsert/delete Qdrant points；若 secondary fixture 缺失，还会写一个专用 MySQL source/item。稳定 point ID、upsert 与删除可重复；恢复时以专用 IDs 检查 MySQL/Qdrant，再从 MySQL active items 重建索引。

## 6. 结果怎么看

本机 bge-m3 示例输出会显示 `Vector dimension: 1024` 与 `offline_rag_memory_items_v1 (Cosine)`；实际 demo 显示的是当次 embedding 响应维度，非 bge-m3 模型名或 1024 的硬编码断言。primary user 查询没有其他用户 point。检索返回不带 vector，只返回 score 与 payload；score 是相似度排序信息，不是事实正确率。

## 7. 算一遍或走一遍

两条输入文本返回 vectors 时，必须得到两条向量；若第一条长度为 d、第二条为 d-1，整个批次失败，不能挑一条写入。query vector 也必须是有限数。对 user `u-1` 的请求，Search 会精确重验 payload `user_id`、Qdrant point ID 与 `memory_item_id`、kind、normalized key、非空 value、正 version 与有限 score；任一不符都会报错，绝不交给 prompt。它不验证 payload 的 `embedding_model`。

## 8. 常见误解

“qwen 能回答所以可直接 embed”错误，本机示例的 embedding 模型是 bge-m3。“Qdrant filter 已足够”错误，外部 payload 仍需二次核验。“forgotten 只是 status，不需删 point”错误，遗留 point 仍可能被召回。`EnsureCollection` 也不是纯检查：集合缺失时它会写入创建和索引。

## 9. 当前实现与生产边界

已保证：collection 已存在时严格核对“本次实际 embedding 维度/Cosine”，不匹配则失败而不自动删库；每次 Search 强制 user filter，并重验 user、point/item ID、kind/key/value/version 与 score。未保证：payload `embedding_model` 的返回重验，或 MySQL item commit 与 Qdrant upsert/delete 原子一致；没有 outbox 时需要重建/对账处理失败后的漂移。1024/Cosine 是本课本机 bge-m3 示例，不是 demo 对所有模型的硬编码要求。

## 10. 面试怎么说

我将 MySQL active memory 作为事实源，把稳定 kind/key/value 文本交给配置的 embedding 模型生成向量，并以当次实际维度 ensure Cosine collection；1024 是本机 bge-m3 示例。Qdrant 只做派生索引：查询既带 user_id filter，又重验 user、point/item ID、kind/key/value/version 和 score（不验返回 embedding_model）；forgotten item 删除 point，跨库一致性靠 outbox/重建而非假设事务存在。

## 11. 自检题与答案

问：qwen 与 bge-m3 的职责相同吗？答：不同，前者生成，后者是本机示例 embedding 模型。问：demo 强制 bge-m3 或 1024 吗？答：不强制，使用实际 embedding 响应维度。问：向量维度不一致能部分写入吗？答：不能，整批失败。问：Search 会重验 payload embedding_model 吗？答：不会。问：EnsureCollection 一定无副作用吗？答：不一定，可能创建 collection/index。

## 12. 事实锚点

- 原课：[memory-item-qdrant-sop.md](../memory-item-qdrant-sop.md)
- Go 组件：`internal/memoryitem/{embed,qdrant}.go`
- 演示：`cmd/memory-qdrant-demo/main.go`
- 前置：[L22](L22-memory-store.md)
