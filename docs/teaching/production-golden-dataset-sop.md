# 第 34 节：版本化 Production Golden Dataset

本节把 L33 的 12 个教学问题扩成一套可以支撑后续检索决策的固定评估契约。它仍是
仓库内审核过的生产风格数据，不冒充真实线上流量。

真实入口：

```bash
go run ./cmd/retrieval-quality-demo dataset \
  --config config/recent-chat.env \
  --dataset internal/retrievalquality/testdata/golden/v1
```

## 1. 三个版本文件

目录：

```text
internal/retrievalquality/testdata/golden/v1/
  manifest.json
  corpus.json
  cases.json
```

`manifest.json` 固定 dataset/version、24/16 split 数量和两个原始 JSON 文件的
SHA256。任何人改了 corpus 或 case 却没有显式发布新版本，loader 会在评估前拒绝。

`corpus.json` 有 24 个可独立重建的 chunks，分属
`retrieval-quality-course` 和 `operations-course`。每条保存 document/chunk/scope、
结构字段、完整内容和内容 SHA256。

`cases.json` 固定 40 个问题：

| 类型 | 用途 | 数量 |
| --- | --- | ---: |
| exact | 路径、错误码、参数和值 | 8 |
| code | 函数、类型、标识符 | 8 |
| semantic | 不复用原文关键词的释义 | 8 |
| mixed | 概念与精确锚点混合 | 8 |
| negative | 当前 scope 没有答案 | 8 |

每个非负例保存 1-5 个 relevance=1..3 的 judgments；每个 case 都保存至少一个
forbidden chunk。策略只能用 24 个 train cases 调参，16 个 validation cases 只能
出报告。

## 2. 指标怎么算

代码：

- `internal/retrievalquality/metrics.go`
- `internal/retrievalquality/evaluate.go`

```text
Recall@K = top K 命中的不同相关 chunk 数 / 所有标注相关 chunk 数
MRR@10   = 1 / 第一个相关 chunk 的排名
DCG@10   = Σ (2^relevance - 1) / log2(rank + 1)
NDCG@10  = 实际 DCG / 理想顺序 DCG
```

NDCG 不只关心“有没有召回”，还会奖励高 relevance 的结果排在前面。延迟使用一次
评估中每个 case 的真实检索耗时，p50/p95 采用 nearest-rank。

负例只有策略明确 `abstained=true` 才算通过。Dense baseline 总会返回候选，所以当前
negative pass rate=0；这是需要在 L38 用阈值解决的已知基线，不应篡改负例来隐藏。

## 3. 真实 Dense baseline

2026-07-30 使用本地 `bge-m3` 实测：

| Split | Cases | Recall@3 | Recall@10 | MRR@10 | NDCG@10 | Negative pass | p50 | p95 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| train | 24 | 1.0000 | 1.0000 | 0.9750 | 0.9815 | 0.0000 | 95.06 ms | 178.67 ms |
| validation | 16 | 1.0000 | 1.0000 | 1.0000 | 1.0000 | 0.0000 | 97.16 ms | 150.81 ms |

两组都是：

```text
scope_isolation=1
forbidden_hit_count=0
passed=true
```

Dataset identity：

```text
corpus_sha256=759f53ea2abb784af9679784ee62533cf9a4cb35ff7c2d14bf6c91169fe2bba5
cases_sha256=2046c6f694603ffbff5e70816a336933e220190fb829008fb348b88710860cb2
```

## 4. 为什么 Dense 在这里已经很高

Corpus 只有 12 个 chunk/scope，top 10 几乎覆盖整个 scope；所以 Recall@10 很容易
达到 1。真正有区分度的是排名、负例拒答、延迟和分类报告。

这正是后续 L35-L38 必须同时报告 NDCG、negative pass 和成本的原因。不能看到
Recall@10=1 就声称检索问题已经解决。

## 5. 硬门禁

以下不是普通扣分，而是评估失败：

- 文件 checksum 不匹配
- case/split/category 数量不符合 v1
- judgment 引用不存在或跨 scope chunk
- relevant 与 forbidden 重叠
- relevance 不在 1..3
- 策略返回跨 scope、空 ID 或重复 ID
- 一次查询返回超过固定 K

## 6. 面试复述

可以这样回答：

> 我先把语料和 Golden Cases 版本化，用 SHA256 把评估数据固定下来；训练集只负责
> 调参，验证集只负责最终比较。每个问题有分级相关结果和 forbidden hit，因此我会
> 同时报 Recall、MRR、NDCG、负例拒答、scope 隔离和 p95，而不是只挑几个成功问题。
> 这套 v1 是生产评估契约，不代表已经获得真实生产泛化。
