# L38：用 train 校准并选择检索策略

对应源课：[retrieval-decision-policy-sop.md](../retrieval-decision-policy-sop.md)。它只选择 Dense/Sparse/Hybrid 的检索路由，Reranker 是否可启用仍由 L37 单独裁定。

## 1. 本课一句话

用 train 候选的 PAVA 把不同分数校准到 0–1，再在 81 组纯检索参数中按 train NDCG、Recall、同批实测 p95 与确定性 tie-break 选每类 query 的路由；validation 只做回归门禁，合格才原子发布供运行时读取的本地 JSON 策略文件（policy artifact）。

## 2. 先用人话理解

Dense 的 0.7、Sparse 的 12.3、RRF 的 0.032 都不是同一把尺。校准器像把各自的“分数语言”翻译为大致相关概率；而 train 是练习卷，validation 是封存考试卷。把期末卷成绩写回选参，得到的策略会虚高且不可相信。

## 3. 系统里有什么

PAVA（单调 isotonic regression）把原始分数从低到高看；如果预测相关率出现倒退，就合并相邻区间，直到相关率不再下降。它保存 `raw score -> [0,1] relevance` 的 breakpoints/values：breakpoint 是分段规则的分界分数；clamp 是分数超出两端时使用最靠近端点的预测值。quota 是最多保留的候选数；threshold 只看最高位候选的校准相关率，低于它就让整次 query abstain 并清空全部候选。网格仅为 dense weight `{0.5,1,2}` × sparse weight `{0.5,1,2}` × quota `{5,10,20}` × threshold `{0,0.25,0.5}`，即 `3×3×3×3=81` 组纯检索组合；没有 `rerank_enabled`。

## 4. 一条完整链路

对每 case 各测一次 Dense 与 Sparse 并缓存 → 从 train 相关性拟合校准器 → 权重复用同批候选做 RRF → quota/threshold 在内存确定性重放 → 按 kind 汇总 396 条 train outcome → 每类选赢家 → validation 比基线做门禁 → 仅通过时将 artifact 写入允许目录的临时文件并原子替换。

396 的构成是：81 个 Hybrid 参数组合 × 4 个 query kind = 324；等权（1/1）时有 `3 个 quota × 3 个 threshold=9` 组，Dense 与 Sparse 各评估一次，所以共有 `9×2=18` 组，再乘 4 个 kind 得 72；总计 `324+72=396`。每个 kind 的排序顺序固定为 NDCG@10、Recall@10、同一批测量的 p95，三者完全平手才比较 canonical params JSON（固定字段顺序序列化），仍平手才比较 strategy 名称；这只为稳定打破平手，不把随机顺序当质量。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/retrievalquality ./cmd/retrieval-quality-demo -count=1
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/retrieval-quality-demo policy --config /absolute/path/to/your/recent-chat.env --dataset internal/retrievalquality/testdata/golden/v1 --policy-output .cache/retrieval-quality/policy-v1.json
```

- test 可重复，只写 build cache，通常数秒。
- `policy` 需要 L35 alias、本机回环 Qdrant/Ollama、tokenizer 与私有配置；读取索引/模型，可能在门禁成功后本地写入唯一允许的 `.cache/retrieval-quality/*.json`，通常数十秒到数分钟。路径必须是该目录下的相对 `.json`；只有全部校验完成的同目录临时文件才会原子替换目标 artifact。回归门禁失败不写新文件，也不会替换已有成功 artifact；先看 JSON 的 regression、identity 与服务，再修复后重跑，不能复制/手写产物绕过门禁。

## 6. 结果怎么看

Hybrid 单 case latency 固定为 `max(dense_duration,sparse_duration)`，不是两路相加，也不为每个权重重新请求；因此权重只影响排名，赢家直接比较同批 p95。连续四次得到同一 checksum：`8ce7c0df4fe5bbab63cbf847cf8e486da17b4f8f156aa49574ec6ecb77dd2e03`。validation 门禁为 scope isolation=1、forbidden=0、negative pass/Recall/NDCG 不低于 Dense baseline、且 `p95 ≤ Dense baseline×3`。

## 7. 算一遍或走一遍

最小 PAVA 手算：按 raw score 从低到高，相邻标签/相关率是 `1,0`，第二个预测比第一个低，违反“不下降”，所以合并成 `0.5,0.5`；若后面还有 `1`，结果为 `0.5,0.5,1`，已经单调。再看延迟：一条 query 的 Dense=80 ms、Sparse=12 ms，则任意 Hybrid 权重的这次 latency 是 `max(80,12)=80 ms`。权重 `0.5/0.5` 与 `1/1` 的相对比例都为 1:1，RRF 顺序相同；质量与 p95 也平手时，才由 canonical 参数排序选 `0.5/0.5`。

## 8. 常见误解

“81 组包含 rerank 开关”错误，L38 是纯检索 train-only grid。“validation 分数好也能参与 winner”错误，代码忽略 validation outcome。“每个权重重测才公平”错误，会把 warm/cold 抖动当参数差异。“回归失败仍可留下 artifact 供试用”错误，根本不会发布。“负例通过率 0 是成功拒答”错误，表示本阶段仍没解决无依据拒答。

## 9. 当前实现与生产边界

最终 train-only routes：

| Query kind | Strategy | Dense/Sparse | Quota | Threshold |
| --- | --- | ---: | ---: | ---: |
| code | Dense | 1 / 1 | 10 | .25 |
| exact | Sparse | 1 / 1 | 10 | .25 |
| mixed | Sparse | 1 / 1 | 10 | .25 |
| semantic | Hybrid | .5 / .5 | 10 | 0 |
| unknown | 等权 Hybrid fallback | 1 / 1 | 10 | 0 |

artifact 绑定 dataset/version、corpus/cases SHA256、embedding model、sparse encoder、calibrators 与 checksum；缺失 artifact 时明确 warning 用等权 RRF，身份/checksum 不匹配硬失败。基础设施降级为 Sparse→Hybrid→Dense、Hybrid→Dense；scope/ownership/identity/非有限分数永远硬失败。

## 10. 面试怎么说

我只在 train 上用 PAVA 分别校准 Dense、Sparse、Hybrid，并在固定 81 组纯检索网格中，以 train NDCG、Recall、同批 p95 和 canonical 参数选各 query kind 路由。validation 不参与选参，只检查质量、隔离、forbidden、负例和 3 倍 p95。artifact 绑定数据与 encoder identity，发布原子且回归失败不写；Reranker 启用资格由独立 L37 validation 决定。

## 11. 自检题与答案

问：网格有多少组，为什么？答：81，`3×3×3×3`。问：Hybrid duration 怎样算？答：`max(Dense,Sparse)`。问：policy 输出允许写到 `/tmp/a.json` 吗？答：不允许，只能 `.cache/retrieval-quality/*.json`。问：semantic 最终 route？答：Hybrid，.5/.5、quota 10、threshold 0。问：checksum 连续几次相同？答：四次。

## 12. 事实锚点

- 源课：[retrieval-decision-policy-sop.md](../retrieval-decision-policy-sop.md)
- 入口：`cmd/retrieval-quality-demo/policy.go`
- 实现：`internal/retrievalquality/{calibration,policy,policy_strategy,regression}.go`
- 前置：[L34](L34-production-golden-dataset.md) → [L35](L35-field-aware-sparse-retrieval.md) → [L36](L36-hybrid-rrf.md)；Reranker 门禁见 [L37](L37-reranker-diversity.md)
