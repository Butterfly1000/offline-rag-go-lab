# 第 36 节：可解释的 Hybrid RRF

本节把 L34 Dense 和 L35 Sparse 两路候选合并。核心原则是：原始分数不能直接相加，
因为 Cosine 相似度和 Sparse IDF 分数不在同一个量纲。

真实入口：

```bash
go run ./cmd/retrieval-quality-demo hybrid \
  --config config/recent-chat.env \
  --dataset internal/retrievalquality/testdata/golden/v1 \
  --dense-weight 1 --sparse-weight 1
```

## 1. 两路怎么执行

一次 query 复制成两份，两份使用完全相同的：

- `knowledge_scope`
- dataset/corpus identity
- top 20 candidate limit

Dense 路调用本地 `bge-m3` 后查询 named vector `dense`；Sparse 路用固定 Qwen
tokenizer/BM25 encoder 查询 named vector `sparse`。两路并发执行，返回 payload 后分别
重校验 scope、point ID、content hash、model 和 encoder identity。

## 2. Weighted RRF 怎么算

固定公式：

```text
rrf_score = dense_weight  / (60 + dense_rank)
          + sparse_weight / (60 + sparse_rank)
```

rank 从 1 开始。某个 chunk 只在一路出现时，另一路 contribution 为 0。

例如 `rq-pava` 在一次真实查询中：

```text
dense_rank=1
dense_score=0.68423474
dense_contribution=1/(60+1)=0.01639344

sparse_rank=1
sparse_score=18.762617
sparse_contribution=1/(60+1)=0.01639344

rrf_score=0.03278689
```

这里记录原始 score 是为了诊断，但融合公式没有使用它。测试专门把原始分数改成
`1000000`、负数等完全不同尺度，并证明只要 rank 不变，融合顺序和 RRF score 就
不变。

## 3. 相同 chunk 怎样合并

使用稳定 `chunk_id` 合并两路结果，并检查 scope/document/content/heading 不冲突。
同一路重复 ID、一个 leg 混合多个 scopes、两路同 ID 却 scope 不同，都会作为
integrity failure 终止。

排序规则固定为：

1. RRF score 降序
2. 最佳单路 rank 升序
3. chunk ID 字典序

所以相同输入重复运行会得到相同排名和 reason。

每个候选保留：

```text
dense_rank / dense_score / dense_contribution
sparse_rank / sparse_score / sparse_contribution
rrf_score
reason=hybrid_rrf|dense_only_rrf|sparse_only_rrf
```

## 4. 真实三路对比

2026-07-30 第二次完整运行：

| Strategy | Split | Recall@10 | MRR@10 | NDCG@10 | p50 | p95 |
| --- | --- | ---: | ---: | ---: | ---: | ---: |
| Dense | train | 1.0000 | 0.9750 | 0.9815 | 101.15 ms | 118.96 ms |
| Sparse | train | 0.7750 | 0.8000 | 0.7894 | 13.09 ms | 21.86 ms |
| Hybrid | train | 1.0000 | 0.9750 | 0.9793 | 100.83 ms | 111.75 ms |
| Dense | validation | 1.0000 | 1.0000 | 1.0000 | 100.90 ms | 115.46 ms |
| Sparse | validation | 0.6667 | 0.6667 | 0.6667 | 13.23 ms | 14.61 ms |
| Hybrid | validation | 1.0000 | 1.0000 | 1.0000 | 102.54 ms | 148.12 ms |

所有策略的 scope isolation 都是 1，forbidden hit 都是 0。

这个小数据集上 Dense validation 已经是 1，所以 Hybrid 没有提升上限；train NDCG
还从 0.9815 轻微降到 0.9793。这说明“加了 Hybrid”不等于“必然更好”，权重必须在
L38 用 train 数据选择，不能凭感觉启用。

两次真实运行的 trace top 5 顺序一致：

```text
rq-pava
rq-reranker-contract
rq-bm25-fields
rq-fuse-function
rq-rrf-formula
```

## 5. 降级边界

Runtime 模式中，只有 Sparse 的基础设施错误可以：

```text
warning -> 保留 Dense 原顺序 -> reason=dense_fallback
```

评估模式任一路不可用都会失败，不能把 fallback 报告冒充 Hybrid。以下在任何模式
都硬失败：

- scope/ownership 违反
- point/payload identity 冲突
- 空或重复 chunk ID
- 非有限 score
- 同一个 leg 混合 scope

## 6. 面试复述

> Dense 和 Sparse 原始分数不可比，所以我用 weighted RRF，只消费每一路的排名。
> 每条结果记录两路 rank、原始 score、RRF contribution 和决策原因，便于复现。
> Runtime 的 Sparse 基础设施故障可以降级 Dense，但 ownership 或 payload 错误必须
> 硬失败；评估时任何 leg 缺失都不能算完整 Hybrid。
