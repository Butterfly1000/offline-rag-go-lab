# L35：字段感知的稀疏检索

对应源课：[field-aware-sparse-retrieval-sop.md](../field-aware-sparse-retrieval-sop.md)。它让关键词、路径和错误码有自己的检索通道。

## 1. 本课一句话

Sparse retrieval 把 tokenizer 的 token ID 当作可稀疏索引，以标题、层级、路径和正文的不同权重计算 BM25 值；它擅长精确词面匹配，却不会自动理解同义中文语义。

## 2. 先用人话理解

Dense 像按“意思相近”找资料；Sparse 像图书馆索引卡：`E_CONNREFUSED`、函数名、文件路径相同就很有把握。标题出现关键词比正文更醒目，所以不能把所有文本当成一团。

## 3. 系统里有什么

专用 physical collection 是 `offline_rag_retrieval_quality_lab_v1`，稳定 alias 是 `offline_rag_retrieval_quality_lab_active`。每 point 同时有 `dense`（1024 维 Cosine）与 `sparse`（Qdrant IDF）named vector，并有 `knowledge_scope`、`document_id`、`chunk_id` payload index。字段权重为 title=3、heading=2、source_ref=1.5、body/code=1；tokenizer identity 会写入 point。

## 4. 一条完整链路

加载并校验 L34 dataset → 一次性生成全部 Dense embedding，并预校验每一条均为 1024 维且每个值有限 → `BuildFieldStats`/`NewSparseEncoder` → `EnsureCollection` 创建或验证 collection，并创建 `knowledge_scope`、`document_id`、`chunk_id` 三个 payload index、等待成功 → 逐 chunk `encoder.EncodeChunk` 后 upsert → `Inspect` 只验证 green、24 points、Dense/Sparse named-vector schema → 才可 activate alias。查询时两路都附带 scope filter，返回后再次核验身份。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/retrievalquality -count=1
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/retrieval-quality-demo index --config /absolute/path/to/your/recent-chat.env --dataset internal/retrievalquality/testdata/golden/v1
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/retrieval-quality-demo index --config /absolute/path/to/your/recent-chat.env --dataset internal/retrievalquality/testdata/golden/v1 --apply --activate
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/retrieval-quality-demo sparse --config /absolute/path/to/your/recent-chat.env --dataset internal/retrievalquality/testdata/golden/v1
```

- test 可重复，只写 build cache，数秒。
- 第一条 `index` 是干跑：只读本地配置、dataset 和 tokenizer，不访问服务、不写入。
- `--apply --activate` 需要本机 loopback Ollama、Qdrant、tokenizer 和私有配置；它会写隔离实验 collection/alias，通常数十秒到数分钟。所有 embedding 在第一次 collection 写入前完成 1024 维/有限值校验；失败时不应靠删除 collection 猜修复。
- `sparse` 只读本机 Qdrant alias 与本地资产，可重复；先确认 index 已成功且 alias 指向实验 collection。

## 6. 结果怎么看

字段内每个 term：`value=field_weight*tf*(k1+1)/(tf+k1*(1-b+b*field_length/average_field_length))`，其中 `k1=1.2`、`b=0.75`；长字段会被归一化，Qdrant IDF 再降低常见词。在已记录的 v1 validation 中，exact/code/mixed 的 Recall@10、NDCG@10 都为 1，semantic 为 0；这说明它适合精确锚点，不是公式失败。

## 7. 算一遍或走一遍

若某 token 在标题出现一次，标题长度正好是平均长度：`3*1*(1.2+1)/(1+1.2*(1-0.75+0.75*1))=3*2.2/2.2=3`。同一个 token 只在正文出现一次则是 1；因此相同词在标题的证据是正文的三倍（尚未计 IDF）。

## 8. 常见误解

“Sparse 就是词频计数”错误，它还做字段权重、长度归一化和 IDF。“token ID 可换 tokenizer 后继续用”错误，不同词表的整数不能混用。“中文问法一定能搜英文正文”错误，缺少词面重合常得到空结果。“跨 scope 命中可低分保留”错误，它是完整性失败。

## 9. 当前实现与生产边界

当前平均 token 长度是 title=5.0417、heading=6.1250、source_ref=8.1667、body=40.0833；encoder identity 为 `qwen2:b6f5871f48c795dab37040781043d08c4b457c79c1a3f22a394f97cbbfe0a9b8:field-bm25-v1`。这是独立 lab，代码没有 collection 删除 API；不要把它称为已接管真实 `/chat`。生产还需容量、变更发布和持续评估设计。

## 10. 面试怎么说

我用固定 tokenizer 的 token ID 建 sparse vector，并按 title、heading、source_ref、正文做字段加权 BM25，再让 Qdrant 做 IDF。它对错误码、配置键、函数名很强，但同义语义弱，所以与 Dense 分开评估；写入前检查全部 Dense vector 1024 维且有限，查询前后都守住 scope 和 encoder identity。

## 11. 自检题与答案

问：为什么先生成并校验全部向量才写 collection？答：避免半份索引混入维度错误/NaN。问：title 权重是多少？答：3。问：Sparse semantic validation 为 0 是否改 case？答：不改，这正是它的能力边界。问：`--activate` 能单独运行吗？答：不能，必须同时 `--apply`。

## 12. 事实锚点

- 源课：[field-aware-sparse-retrieval-sop.md](../field-aware-sparse-retrieval-sop.md)
- 入口：`cmd/retrieval-quality-demo/{index,sparse}.go`
- 实现：`internal/retrievalquality/{sparse,qdrant}.go`
- 前置：[L34](L34-production-golden-dataset.md)，后续：[L36](L36-hybrid-rrf.md)
