# 第 38 节：校准后的检索决策策略

本节不训练新模型。它把 L34 的固定 train 标注、L35 Sparse 和 L36 Hybrid 组合成
一份可版本化的检索策略。L37 独立决定 Reranker 是否具备启用资格，不把 validation
结论反向写进 L38 的候选网格。核心边界是：

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

命令只接受本机回环地址，只读本地 Qdrant/Ollama。policy artifact 只能写入 Git
忽略的 `.cache/retrieval-quality/*.json`，使用同目录临时文件原子替换；validation
回归门禁不通过时不发布产物。

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
minimum_relevance: 0, 0.25, 0.5
```

共 81 组，全部只依赖 train 数据。Reranker 不在 policy schema 和本节网格中：
当前 `qwen:7b` 的质量与延迟结论属于 L37 validation 门禁；未来专用 Reranker
是否启用也应先重跑 L37，而不是让 L38 根据 validation 结果删减训练候选。

Dense 与 Sparse 的真实候选各请求一次并按 case 缓存；不同权重直接融合同一批候选，
每个 Hybrid case 的测量延迟取两路实测值的较大者。quota、threshold 和权重随后在 Go
中确定性重放，避免参数组合重复调用模型。最终产生 396 条按 query kind 聚合的 train
outcome。

每种 query kind 独立排序：

1. train NDCG@10 更高；
2. train Recall@10 更高；
3. 同一批测量中的 p95 更低；
4. canonical params JSON；
5. strategy 名称。

validation outcome 即使写成一个会改变 winner 的“陷阱值”，测试也证明 selection
完全忽略它。

## 3. 怎样直接比较 p95 又保持可重复

首版为每个 Hybrid 权重重新请求模型，warm/cold 与调度差异会把“权重差异”和“本次
请求快慢”混在一起。粗粒度 latency tier 虽能隐藏抖动，却违反“质量相同时选择真实
较低 p95”的规则。

修正后，每个 case 只测一次 Dense 与一次 Sparse。所有权重共享这两路候选和耗时，
Hybrid 延迟固定为 `max(dense_duration, sparse_duration)`；因此权重只改变 RRF 排名，
不再凭重复请求的偶然快慢获胜。选择器直接比较同批 train p95，validation 继续用
`p95 <= Dense baseline * 3` 检查实际退化。

审查修复后的真实运行得到：

```text
policy_checksum=8ce7c0df4fe5bbab63cbf847cf8e486da17b4f8f156aa49574ec6ecb77dd2e03
```

连续四次真实运行得到相同路由和 checksum；测量延迟可以变化，策略身份保持不变。

## 4. 最终路由

train-only 选择结果：

| Query kind | Strategy | Dense/Sparse weight | Quota | Min relevance |
| --- | --- | --- | ---: | ---: |
| code | Dense | 1 / 1 | 10 | 0.25 |
| exact | Sparse | 1 / 1 | 10 | 0.25 |
| mixed | Sparse | 1 / 1 | 10 | 0.25 |
| semantic | Hybrid | 0.5 / 0.5 | 10 | 0 |
| unknown | Hybrid fallback | 1 / 1 | 10 | 0 |

`0.5/0.5` 与 `1/1` 的 RRF 相对权重相同；前者由 canonical tie-break 选中，不代表质量
神奇提升。

Policy 运行时根据 artifact 自己创建对应权重的 Hybrid，不依赖命令外预先拼好的路由。
artifact 缺失时明确 warning 并使用等权 RRF；artifact 存在但 checksum、dataset 或
encoder identity 不匹配时硬失败。运行时输出 calibrated relevance 和 decision
reason。选中策略发生基础设施错误时：

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
| Dense validation | 1 | 1 | 1 | 0 | 114.850 ms |
| Policy validation | 1 | 1 | 1 | 0 | 114.850 ms |

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
