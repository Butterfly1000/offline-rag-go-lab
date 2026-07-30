# 第 38 节：校准后的检索决策策略

本节不训练新模型。它把 L34 的固定 train 标注、L35 Sparse、L36 Hybrid 和 L37
Reranker 门禁组合成一份可版本化的检索策略。核心边界是：

```text
train：拟合校准器、选择参数和路由
validation：只做最终报告和回归门禁
```

真实入口：

```bash
go run ./cmd/retrieval-quality-demo policy \
  --config config/recent-chat.env \
  --dataset internal/retrievalquality/testdata/golden/v1 \
  --policy-output .cache/retrieval-quality/policy-v1.json
```

命令只读本地 Qdrant/Ollama，policy artifact 写入 Git 忽略的项目 `.cache`。

## 1. 为什么要校准 score

Dense Cosine、Sparse IDF 和 Hybrid RRF 分数不是同一量纲。不能把 `0.7`、`12.3` 和
`0.032` 当作相同置信度。

本节用 deterministic isotonic regression（PAVA）分别拟合：

```text
raw score -> [0,1] relevance probability
```

样本来自 train 候选：被该 query 标注为相关的 chunk 是 1，其余候选是 0。算法先按
score 排序并聚合相同 score，再把违反单调性的相邻块合并。prediction 在两端 clamp，
校准器保存完整 breakpoints/values。

测试覆盖：

- 下降相邻块合并；
- 重复 score 与输入顺序无关；
- prediction 单调且在 `[0,1]`；
- NaN、Inf、空样本和非法 label 失败；
- Brier score 和 ECE 的固定算例。

## 2. 参数网格怎样选

代码中固定完整网格：

```text
dense_weight:     0.5, 1.0, 2.0
sparse_weight:    0.5, 1.0, 2.0
candidate_quota:  5, 10, 20
rerank_enabled:   false, true
minimum_relevance: 0, 0.25, 0.5
```

共 162 组。L37 已证明本地 `qwen:7b` Reranker 的 validation 更差、延迟约 30 秒，
因此本次 viable grid 排除 `rerank_enabled=true`，实际评估 81 组；选项仍保留在
schema 和测试中，未来专用 reranker 通过 L37 门禁后可以重新进入。

真实候选只请求一次并按 case 缓存；不同 quota、threshold 和权重在 Go 中确定性重放，
避免为 81 组参数重复调用模型。最终产生 396 条按 query kind 聚合的 train outcome。

每种 query kind 独立排序：

1. train NDCG@10 更高；
2. train Recall@10 更高；
3. 操作级 latency tier 更低；
4. canonical params JSON；
5. strategy 名称。

validation outcome 即使写成一个会改变 winner 的“陷阱值”，测试也证明 selection
完全忽略它。

## 3. 为什么延迟不是直接比较纳秒

第一次真实实现直接用 p95 纳秒做 tie-break。连续运行时质量和候选完全相同，但模型
warm/cold 与调度抖动让 semantic 权重在 `0.5/2`、`1/2` 之间变化，policy checksum
不稳定。

第一轮修正使用 25ms/250ms 等细等级，真实运行仍会跨越 25ms 边界。最终规则改为操作
级延迟：

```text
<= 5s   本地检索级
<= 60s  慢服务级
> 60s   超慢级
```

它能区分本课约百毫秒的检索和 L37 约 30 秒的通用模型 reranker，同时不会把本地热缓存
状态写进版本化 policy。精确性能退化仍由 validation 的 `p95 <= Dense baseline * 3`
回归门禁检查。

最终连续两次运行得到完全相同的：

```text
policy_checksum=737d487569d553249941f979450f9751fae34715fa393cc1543b7ba116d30310
```

测量延迟可以变化，策略身份不能变化。

## 4. 最终路由

train-only 选择结果：

| Query kind | Strategy | Dense/Sparse weight | Quota | Min relevance | Rerank |
| --- | --- | --- | ---: | ---: | --- |
| code | Dense | 1 / 1 | 10 | 0.25 | false |
| exact | Hybrid | 0.5 / 0.5 | 10 | 0.25 | false |
| mixed | Hybrid | 0.5 / 0.5 | 10 | 0.25 | false |
| semantic | Hybrid | 0.5 / 0.5 | 10 | 0 | false |
| unknown | Hybrid fallback | 1 / 1 | 10 | 0 | false |

`0.5/0.5` 与 `1/1` 的 RRF 相对权重相同；前者由 canonical tie-break 选中，不代表质量
神奇提升。

运行时输出 calibrated relevance 和 decision reason。选中策略发生基础设施错误时：

```text
Sparse -> Hybrid -> Dense
Hybrid -> Dense
```

每次 fallback 都带 warning。scope、ownership、identity 或非有限分数错误仍硬失败。

## 5. 校准和 validation 结果

train 拟合，validation 只评估：

| Strategy | Train samples | Train Brier | Train ECE | Validation samples | Validation Brier | Validation ECE |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Dense | 288 | 0.009742 | ~0 | 192 | 0.012097 | 0.017650 |
| Sparse | 148 | 0.015653 | ~0 | 103 | 0.010801 | 0.016990 |
| Hybrid | 288 | 0.018503 | ~0 | 192 | 0.031773 | 0.014947 |

最后一次重复运行：

| Report | Recall@10 | MRR@10 | NDCG@10 | Negative pass | p95 |
| --- | ---: | ---: | ---: | ---: | ---: |
| Dense validation | 1 | 1 | 1 | 0 | 113.665 ms |
| Policy validation | 1 | 1 | 1 | 0 | 111.473 ms |

Policy 同时满足：

```text
scope_isolation=1
forbidden_hits=0
negative_pass >= Dense baseline
Recall@10 >= Dense baseline
NDCG@10 >= Dense baseline
p95 <= Dense baseline * 3
```

负例通过率仍为 0，说明本阶段没有解决“无依据时拒答”。不能为了让数字好看修改
validation；该问题交给 L39-L43 的 evidence/refusal 主线。

## 6. Artifact 身份

Policy 保存：

- dataset ID/version；
- corpus/cases SHA256；
- embedding model 和 sparse encoder ID；
- 每类 route、参数、校准器和原因；
- deterministic policy checksum。

加载时 dataset、encoder 或 checksum 任一不一致都会拒绝运行。生成时间不进入 artifact，
避免时间戳破坏 deterministic checksum。

## 7. 面试复述

> 我用 train split 的候选和二元相关性标注，通过 PAVA 分别校准 Dense、Sparse、
> Hybrid score；再按 query kind 在固定网格里用 train NDCG、Recall、操作级延迟和
> canonical 参数选路。validation 不参与选参，只检查隔离、forbidden、负例、质量和
> 3 倍 p95 门禁。Policy 绑定 dataset 与 encoder checksum，重复运行 checksum 相同；
> 基础设施故障可回到 Hybrid/Dense，identity 错误必须硬失败。
