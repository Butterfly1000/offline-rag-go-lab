# L25：用固定文档 fixture 验证 scope 检索

对应工程课：[Retrieval 小节 25](../document-qdrant-sop.md)。本课只用固定切块走真实 Ollama + Qdrant 路径，验证 document 的 `knowledge_scope` 隔离。

## 1. 本课一句话

固定 fixture 以稳定 point ID 写入默认 `offline_rag_document_chunks_v1`，检索必须带 scope filter 并返回后重验；它验证 scope 检索，不负责真实文件解析、版本、幂等入库或发布。

## 2. 先用人话理解

这是给向量检索铺一条可重复的测试跑道：三块预先写好的路标足以检验“不同知识范围不会串台”，却不能替代把真实 PDF/网页解析、切块、版本化和上线的整座机场。

## 3. 系统里有什么

- 固定 `DocumentChunk` fixture：两个 `offline-rag-course`、一个 `another-course`。
- embedding：当前 `bge-m3` 返回的 1024 维向量；collection 使用 Cosine。
- 默认/唯一 demo collection：`offline_rag_document_chunks_v1`。
- point ID：`SHA256(knowledge_scope + NUL + chunk_id)` 导出的稳定 UUID。

## 4. 一条完整链路

固定 chunk 文本 → bge-m3 embedding → `EnsureCollection` 设/验 1024 Cosine 和 scope/document 索引 → stable ID upsert payload（scope、document/chunk identity、正文 hash、model）→ query embedding → Qdrant `knowledge_scope` filter → 返回后检查 scope、point ID、content hash、model、score 与统一 Hit。L25 与后续 [L29](L29-document-identity.md)–L33 处在同一“fixture → document identity/version → chunk → ingestion collection → alias → retrieval”关系链：L25/L26/recent-chat 当前默认读 `offline_rag_document_chunks_v1`；L29–L33 则构建/发布 `offline_rag_document_ingestion_lab_v1`、`offline_rag_document_ingestion_lab_v2` 与 alias `offline_rag_document_ingestion_lab_active`。除非显式修改 recent-chat 的 `QDRANT_DOCUMENT_COLLECTION` 配置，后续发布结果不会自动进入真实 `/chat`。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/contextretrieval -run 'TestDocumentQdrant|TestDeterministicDocumentPointID' -count=1
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/document-qdrant-demo --config /absolute/path/to/your/recent-chat.env --apply
```

- test 使用本地 fake Qdrant，纯本地、不需真实 Ollama/Qdrant/配置，不写业务数据；只写 build cache，可安全重复。
- demo 依赖 Ollama `bge-m3`、Qdrant 和由 `config/recent-chat.env.example` 派生的私有配置。`--apply` 是写入确认：可能创建 collection/index，并 upsert 三个固定 Qdrant points；稳定 ID 使相同 fixture 重跑覆盖同一 points。恢复时可按该固定 collection/scope/point ID 检查或重建，不应把它误当生产文档清理工具。

## 6. 结果怎么看

输出应有 `Vector dimension: 1024`、`Collection: offline_rag_document_chunks_v1 (Cosine)`、`Cross-scope point present: false` 与 `Idempotent point IDs: true`。对 `offline-rag-course` 查询只能得到该 scope fixture；`another-course` 的固定点只能在其 scope 查询中出现。

## 7. 算一遍或走一遍

同为 `chunk-001` 的两个 chunk，若 scope 分别为 `course-a` 与 `course-b`，NUL 分隔的 hash 输入不同，因此 stable UUID 不同。查询 `course-a` 时即使 Qdrant 返回了 payload `knowledge_scope=course-b`，返回后重验硬失败；它不是“过滤后再丢掉一条普通结果”。

## 8. 常见误解

“固定 fixture 跑通就是生产 ingestion 完成”错误。本课不解析真实文件、不决定 chunk 策略、不管理 document version、幂等 ingestion 或 alias 发布；这些由 [L29](L29-document-identity.md) 起的同一关系链后续课程承担。“稳定 ID 等于全部幂等性”也错误，它只保证本 fixture 对同一 point 的覆盖。

## 9. 当前实现与生产边界

已保证：scope filter 进入 Qdrant 请求，返回 payload 再验证 scope/identity/hash，跨 scope 或畸形 payload 是 integrity hard failure；L25/L26/recent-chat 默认读取 `offline_rag_document_chunks_v1`。未保证：真实文件解析、版本快照、删除旧 chunk、生产 schema 与发布切换；L29–L33 发布到 `offline_rag_document_ingestion_lab_active` 不会自动改变真实 `/chat` 的读取集合，必须显式修改 `QDRANT_DOCUMENT_COLLECTION` 配置。不要用本 demo 的三条 fixture 推断真实文档生命周期已实现。

## 10. 面试怎么说

我先用固定 fixture 验证真实 document vector path 的关键安全契约：scope 参与 stable point identity，查询必须带 scope filter，结果再验证 payload/ID/hash。它与 L29–L33 是 fixture → ingestion → alias 的同一条前向关系链，后续 L29 会反向链接本课的 fixture 边界；但默认 real/chat 仍读 `offline_rag_document_chunks_v1`，未改配置就不会自动切到 alias。

## 11. 自检题与答案

问：L25 会解析 PDF 并自动切块吗？答：不会。问：L25/L26/recent-chat 默认 collection 名是什么？答：`offline_rag_document_chunks_v1`。问：L29–L33 发布 alias 后会自动进真实 `/chat` 吗？答：不会，除非显式改 `QDRANT_DOCUMENT_COLLECTION`。问：跨 scope payload 可否当作空结果忽略？答：不可以，必须硬失败。问：下一步身份/版本在哪讲？答：双向关系链的 [L29](L29-document-identity.md)。

## 12. 事实锚点

- 原课：[document-qdrant-sop.md](../document-qdrant-sop.md)
- Go 组件：`internal/contextretrieval/{document,document_qdrant}.go`
- 演示：`cmd/document-qdrant-demo/main.go`
- 双向前向关系链：[L24](L24-context-hit-boundary.md) → L25 → [L29](L29-document-identity.md)–L33 → `offline_rag_document_ingestion_lab_active`；默认 real/chat 仍读 `offline_rag_document_chunks_v1`，L29 会反向链接 L25。
