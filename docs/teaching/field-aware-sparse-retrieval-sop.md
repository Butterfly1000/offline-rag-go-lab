# 第 35 节：Field-aware Sparse Retrieval

本节把“关键词搜索”实现成可版本化的 Sparse Vector，并与 Dense Vector 一起写入专用
Qdrant collection。它擅长错误码、路径和标识符，不负责理解没有词面重合的语义。

## 1. 先建立 named-vector snapshot

```bash
go run ./cmd/retrieval-quality-demo index \
  --config config/recent-chat.env \
  --dataset internal/retrievalquality/testdata/golden/v1 \
  --apply --activate
```

唯一允许写入的资源：

```text
physical=offline_rag_retrieval_quality_lab_v1
alias=offline_rag_retrieval_quality_lab_active
```

代码没有 collection 删除 API。Alias 只会在 collection 读回验证为 green、
point_count=24 且 named-vector schema 正确后激活。

真实结果：

```text
vector_size=1024
points=24
dense=Cosine
sparse.modifier=idf
payload indexes=knowledge_scope,document_id,chunk_id
status=green
alias_activated=true
```

## 2. Sparse Vector 怎么算

Tokenizer：

```text
assets/tokenizers/qwen2/tokenizer.json
```

Go 加载这个 JSON，然后对 title、heading_path、source_ref、content 分别取得 token
IDs。Token ID 直接作为 Sparse Vector 的 index，同一个 ID 在多个字段出现时合并。

字段权重：

```text
title=3.0
heading_path=2.0
source_ref=1.5
body_or_code=1.0
```

每个字段中的 term value：

```text
value = field_weight * tf * (k1 + 1)
        / (tf + k1 * (1 - b + b * field_length / average_field_length))

k1=1.2
b=0.75
```

`tf` 增长会逐渐饱和；字段比平均长度更长时，单个词的贡献会降低。最终 indices
按数字递增，零值移除，非有限值和负 token ID 直接失败。Qdrant 再用
`modifier=idf` 按整个 collection 的文档频率降低常见词权重。

本次真实平均字段 token 长度：

```text
title=5.0417
heading=6.1250
source_ref=8.1667
body=40.0833
```

Encoder identity：

```text
qwen2:b6f5871f48c795dab37040781043d08c4b457c79c1a3f22a394f97cbbfe0a9b8:field-bm25-v1
```

它随 tokenizer 文件 checksum 和 Sparse policy version 写入每个 point。查询时发现
point 的 encoder/model 不一致会硬失败，不能把两个词表的 token ID 当成同一维度。

## 3. 运行 Sparse baseline

```bash
go run ./cmd/retrieval-quality-demo sparse \
  --config config/recent-chat.env \
  --dataset internal/retrievalquality/testdata/golden/v1
```

2026-07-30 真实结果：

| Split | Recall@3/10 | MRR@10 | NDCG@10 | p50 | p95 |
| --- | ---: | ---: | ---: | ---: | ---: |
| train | 0.7750 | 0.8000 | 0.7894 | 12.77 ms | 17.14 ms |
| validation | 0.6667 | 0.6667 | 0.6667 | 12.41 ms | 15.02 ms |

Validation 分类：

| 类型 | Recall@10 | NDCG@10 |
| --- | ---: | ---: |
| exact | 1.0000 | 1.0000 |
| code | 1.0000 | 1.0000 |
| mixed | 1.0000 | 1.0000 |
| semantic | 0.0000 | 0.0000 |

两组始终：

```text
scope_isolation=1
forbidden_hit_count=0
```

## 4. 为什么中文语义问题是 0

语料正文主要是英文，而 semantic cases 使用中文释义。Sparse 只知道 token 是否相同，
不会自动知道“切回旧索引”和“rollback verified collection”语义接近。因此查询没有
词面重合时可能返回空列表。

这是正确观察，不是要修改 Golden Dataset 的理由。L36 会保留 Sparse 对精确词的
优势，同时用 Dense 补回语义候选。

## 5. Scope 为什么仍是硬门禁

Dense 和 Sparse 请求都必须在 Qdrant Query API 中发送：

```json
{
  "filter": {
    "must": [
      {"key": "knowledge_scope", "match": {"value": "requested-scope"}}
    ]
  }
}
```

返回后 Go 还会检查 scope、稳定 point ID、content hash、embedding model 和 encoder
identity。跨 scope 不是低分结果，而是 integrity failure。

## 6. 面试复述

> 我用与生成模型一致的 tokenizer IDs 建 Sparse Vector，按 title、heading、路径和
> 正文分别加权，再用 BM25 TF/长度归一化和 Qdrant IDF。它在错误码、函数名和配置键
> 上很强，但无法替代 Dense 语义检索，所以我先单独评估两路，再用 RRF 融合。每路都
> 强制 scope filter，并把 tokenizer checksum 写进 point 防止词表错配。
