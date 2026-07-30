# L34：把检索题库做成可复现的 Golden Dataset

对应源课：[production-golden-dataset-sop.md](../production-golden-dataset-sop.md)。这是一套仓库内审核过的生产风格 fixture，不是线上用户流量。

## 1. 本课一句话

把语料、问题、相关性标注和禁止命中项一起版本化并用 SHA256 锁定；只用 24 条 train 调参，16 条 validation 只作最终报告，才能比较不同检索方案而不“偷看答案”。

## 2. 先用人话理解

Golden Dataset 像一份密封考试卷：题目之外，还明确哪些资料算对、排得多靠前更好、哪些资料绝不能拿到。先封卷，再做实验；若边调参数边看 validation 成绩，就相当于拿期末题练习，最后的高分不可信。

## 3. 系统里有什么

`internal/retrievalquality/testdata/golden/v1/` 有 `manifest.json`、`corpus.json`、`cases.json`：24 个可重建 chunk、40 个 case（24 train、16 validation）。exact、code、semantic、mixed、negative 各 8 条；非负例有 relevance 1–3，所有 case 至少有一个 forbidden chunk。manifest 保存 corpus/cases SHA256，内容被悄悄改动会在评估前拒绝。

## 4. 一条完整链路

加载 manifest 与两份 JSON → 核对 checksum、split/类别覆盖、scope 和 judgment → 对每题检索固定 K 条 → 检查 scope、空/重复 ID、forbidden → 计算指标 → 分别输出 train/validation 报告。当前 fixture 每类各 8 条，但 loader 只强制五类都有覆盖。任何跨 scope、非法引用或超过 K 的结果都是硬失败，不是“扣一点分”。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/retrievalquality -count=1
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/retrieval-quality-demo dataset --config /absolute/path/to/your/recent-chat.env --dataset internal/retrievalquality/testdata/golden/v1
```

- test 使用本地 fake 和仓库 fixture；可重复，只写 Go build cache，通常数秒。
- `dataset` 先读私有配置（不要贴出其内容），需要本机回环地址的 Ollama 与 `OLLAMA_EMBED_MODEL`；它只读 fixture、只调用本地 embedding，不写 Qdrant 或数据集。通常几十秒到数分钟，取决于本机模型。失败时先检查 checksum、模型与 loopback 服务，再重跑；不要修改 case 来“修复”分数。

## 6. 结果怎么看

`Recall@K=命中的不同相关 chunk 数/全部相关 chunk 数`；`MRR@10=1/首个相关结果的排名`；`DCG@10=Σ(2^relevance-1)/log2(rank+1)`，`NDCG@10=DCG/理想 DCG`。negative 只有明确 `abstained=true` 才通过。v1 Dense baseline 的 Recall@10 已很高，不表示全部解决：小语料每 scope 约 12 个 chunk，top 10 几乎覆盖整个 scope；仍要看 NDCG、负例和 p95。

## 7. 算一遍或走一遍

某题相关等级依次是 A=3、B=1，返回 `[B,A]`：`DCG=1/log2(2)+7/log2(3)=1+4.416=5.416`；理想 `[A,B]` 为 `7+1/log2(3)=7.631`，所以 `NDCG≈0.710`。Recall 是 `2/2=1`，但 NDCG 揭示最重要的 A 排晚了；这就是不能只报 Recall 的原因。

## 8. 常见误解

“fixture 是真实生产数据”错误，它只是生产风格、可审核的合成/受控评估契约。“validation 也可挑参数”错误，会污染最终比较。“forbidden 只是低相关”错误，它是明确不该出现的命中。“Dense Recall@10=1 就毕业”错误，负例拒答基线仍为 0。

## 9. 当前实现与生产边界

v1 identity 为 `corpus_sha256=759f53ea2abb784af9679784ee62533cf9a4cb35ff7c2d14bf6c91169fe2bba5`、`cases_sha256=2046c6f694603ffbff5e70816a336933e220190fb829008fb348b88710860cb2`。它覆盖固定教学语料，不代替生产抽样、人工标注、权限测试、数据漂移或线上延迟监控。改 corpus/cases 应发布新版本，而不是改旧 manifest 让历史结果失去含义。

## 10. 面试怎么说

我把检索评估做成版本化数据契约：语料和 case 都有 SHA256，case 同时含分级相关性与 forbidden。train 只用于校准/选参，validation 只用于最后门禁；报告 Recall、MRR、NDCG、negative pass、scope isolation 和 p95，所以不会把“看起来能搜到”误说成生产质量。

## 11. 自检题与答案

问：40 个 case 怎样分割？答：24 train、16 validation。问：negative 怎样算通过？答：策略必须 `abstained=true`。问：相关 A、B 都找到了但 A 排第二，Recall 是否仍可为 1？答：可以，但 NDCG 会下降。问：case JSON 改了一字、未发新版本会怎样？答：SHA256 校验失败并拒绝评估。

## 12. 事实锚点

- 源课：[production-golden-dataset-sop.md](../production-golden-dataset-sop.md)
- 数据：`internal/retrievalquality/testdata/golden/v1/{manifest,corpus,cases}.json`
- 指标/评估：`internal/retrievalquality/{metrics,evaluate,dataset}.go`
- 下一课：[L35](L35-field-aware-sparse-retrieval.md)
