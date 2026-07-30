# L36：用 RRF 融合 Dense 与 Sparse

对应源课：[hybrid-retrieval-fusion-sop.md](../hybrid-retrieval-fusion-sop.md)。本课不把两个不可比的原始分数硬相加。

## 1. 本课一句话

Hybrid 同时运行 Dense 与 Sparse，以每一路的排名而非原始分数做 Weighted RRF；候选身份不一致就硬失败，只有运行时 Sparse 基础设施故障可降级为原 Dense 顺序。

## 2. 先用人话理解

Cosine 的 `0.7` 与 Sparse IDF 的 `12.3` 像摄氏度和人民币，直接相加没有意义。RRF 问的是“它在每张榜单排第几”：两边都靠前的资料得到两份贡献，因而更稳健。

## 3. 系统里有什么

同一个 query 被复制为 Dense 和 Sparse 两路，均使用相同 `knowledge_scope`、dataset/corpus identity 与 top-20 候选上限。每个融合候选保留 dense/sparse 的 rank、原始 score、各自 contribution、`rrf_score` 与 reason（`hybrid_rrf`、`dense_only_rrf` 或 `sparse_only_rrf`）。

## 4. 一条完整链路

query → 并发 Dense（本机 bge-m3）与 Sparse（固定 tokenizer/BM25）→ 两路分别复核 scope、point ID、hash、model/encoder → 按稳定 `chunk_id` 合并 → 计算 RRF → 按分数、最佳单路 rank、chunk ID 稳定排序 → 输出可解释 trace。评估模式任一路不可用即失败；运行时仅 Sparse 基础设施错误可警告并保留 Dense 原顺序。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/retrievalquality -count=1
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/retrieval-quality-demo hybrid --config /absolute/path/to/your/recent-chat.env --dataset internal/retrievalquality/testdata/golden/v1 --dense-weight 1 --sparse-weight 1
```

- test 使用本地 fake、可重复，只写 build cache，通常数秒。
- demo 依赖 L35 已发布的实验 alias、本机 loopback Qdrant/Ollama、tokenizer 与私有配置；只读它们、不写 collection，通常数十秒到数分钟。输出失败先检查 alias、模型、encoder identity、scope，不要将评估 fallback 当作 Hybrid 成功。

## 6. 结果怎么看

固定公式为 `rrf_score=dense_weight/(60+dense_rank)+sparse_weight/(60+sparse_rank)`，rank 从 1 开始，未出现的一路贡献为 0。真实例中 `rq-pava` 两路均 rank 1、权重均 1：`1/61+1/61=0.03278689`。原始 score 仅供诊断；改成百万或负数而排名不变，融合顺序仍不变。

## 7. 算一遍或走一遍

chunk A：Dense rank 1、Sparse 未出现，分数 `1/61≈0.016393`；chunk B：两路均 rank 5，分数 `1/65+1/65≈0.030769`。即使 A 的 Dense 原始 score 更高，B 因两路一致而排前。这是融合“证据来源”而不是“分数大小”。

## 8. 常见误解

“Dense/Sparse score 可相加”错误，量纲不同。“只要有一条路成功，评估就算 Hybrid”错误，评估必须两路都成功。“fallback 可掩盖 scope 错误”错误，scope/ownership、冲突 identity、空/重复 ID、非有限 score 永远硬失败。“Hybrid 必然优于 Dense”错误，v1 train NDCG 反而略低。

## 9. 当前实现与生产边界

已记录的 v1 validation：Dense NDCG@10=1、Sparse=0.6667、Hybrid=1；Hybrid p95=148.12 ms（该次运行），不能据此宣布永远更快/更好。运行时 Sparse 基础设施错误可 `dense_fallback`，但评估不能；更复杂的路由权重由 [L38](L38-retrieval-decision-policy.md) 用 train 选择，而不是手工猜。

## 10. 面试怎么说

Dense 和 Sparse 分数不可比，我使用只消费 rank 的 weighted RRF。每条结果记录两路 rank、score、贡献和 reason，稳定排序使重跑可解释。运行时仅 Sparse 基础设施问题可安全退回 Dense；任何 scope 或 payload identity 问题都硬失败，评估时绝不把降级冒充 Hybrid。

## 11. 自检题与答案

问：RRF 的 rank 从几开始？答：1。问：一个 chunk 只在 Sparse 出现，Dense contribution？答：0。问：为什么排序还需 chunk ID？答：处理完全并列，使重复运行确定。问：评估 Dense 成功、Sparse 连接失败怎么办？答：失败，不产出完整 Hybrid 报告。

## 12. 事实锚点

- 源课：[hybrid-retrieval-fusion-sop.md](../hybrid-retrieval-fusion-sop.md)
- 入口：`cmd/retrieval-quality-demo/hybrid.go`
- 融合：`internal/retrievalquality/{fusion,hybrid}.go`
- 前置：[L35](L35-field-aware-sparse-retrieval.md)，后续：[L37](L37-reranker-diversity.md)
