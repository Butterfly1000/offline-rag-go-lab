# L37：Reranker、多样性与安全回退

对应源课：[reranker-diversity-sop.md](../reranker-diversity-sop.md)。它是独立 validation 门禁，不是 L38 选参网格的一项。

## 1. 本课一句话

Reranker 只给 Hybrid 的既有候选重新打相关性分，不能创造 ID；服务或协议不合法时保留原 RRF 顺序并告警，合法结果再做稳定的 document/heading 多样性过滤，当前 `qwen:7b` 因更慢且更差而默认关闭。

## 2. 先用人话理解

召回像先从书库拿 20 本候选书，rerank 像重新排序；它没有权力塞进第 21 本书或给书换条码。模型输出不可靠时，最安全的选择不是乱排，而是退回已经验证过的 RRF 顺序。

## 3. 系统里有什么

输入仅为 query 与 `candidate_id + content`，输出为每候选一个有限 `relevance`。JSON Schema 限制字段、候选数和 ID enum，Go 仍独立检查每个输入 ID 恰好一次。多样性上限：每 document 最多 3 条、每 document+heading path 最多 2 条；底层 `ApplyDiversity` 会返回被跳过的 ID 与 `document_limit`/`heading_limit` 原因，供测试核对；完整 report 只汇总 `diversity_skipped` 数量。

## 4. 一条完整链路

Hybrid top 20 → 模型前检查 scope、ID 与原候选 Score 是否有限 → 每题把全部候选作为一次请求送本机 Ollama（正文标为不可信数据）→ 返回后检查 relevance 有限、ID exact coverage，并稳定排序（relevance 降序、原 RRF rank、chunk ID）→ `ApplyDiversity` 检查 document/heading ownership 并稳定扫描 → 输出 trace、warning、耗时和统计。跨 scope、空/重复 ID、ownership 缺失是硬失败，不能 fallback。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/retrievalquality -count=1
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/retrieval-quality-demo rerank --config /absolute/path/to/your/recent-chat.env --dataset internal/retrievalquality/testdata/golden/v1 --model qwen:7b
```

- test 可重复，只写 build cache，通常数秒。
- demo 依赖 L35 alias、回环 Qdrant/Ollama（embedding 与 `qwen:7b`）、tokenizer 和私有配置；只读索引，不写 collection。它还以 `http://127.0.0.1:1` 证明强制 fallback，完全是本机 loopback。完整 rerank 每题一次请求，携带该题全部候选；16 个 case 对应 calls=16，约数分钟。连接/JSON/coverage 失败会保留 RRF 顺序。不要把私有 config 内容输出到报告。

## 6. 结果怎么看

已记录 validation：Hybrid RRF NDCG@10=1、p95=2.339 s；Rerank+diversity NDCG@10=0.8348、p95=34.156 s。16 次调用中 used=11、fallbacks=5，输入 192 候选，diversity_skipped=0；强制 fallback 的 10 个 ID 与原 RRF 顺序逐项相同。因此 `default_enabled=false`，不是“多一层 AI 就默认开”。

## 7. 算一遍或走一遍

候选原顺序为 `[A(doc1,h1), B(doc1,h1), C(doc1,h1), D(doc1,h2)]`。多样性稳定扫描保留 A、B；C 触发同 heading 最多 2 条而跳过；D 仍可保留（同 doc 此时共 3 条）。若 reranker 只返回 A 两次、漏 B/C/D，coverage 不合格，输出必须回到原 RRF 顺序而非只保留 A。

## 8. 常见误解

“`format=json` 就足以保证业务格式”错误，它只保证合法 JSON；Schema 与 Go exact coverage 都需要。“fallback 表示结果无效”错误，基础设施/协议 fallback 保留合法 RRF。“任何错误都可 fallback”错误，越权和 identity 错误必须停下。“这是专用 cross-encoder”错误，当前是通用 `qwen:7b` 评分器。

## 9. 当前实现与生产边界

首次只用 `format=json` 的真实实验 16 次全 fallback；Schema 后仍有 5 次重复 candidate ID，证明第二层 Go 检查不可省。模型记录为 `qwen:7b`、family qwen2、8B、Q4_0。当前黄金 corpus 不触发 diversity 上限，测试用集中候选验证规则：底层 `ApplyDiversity` 可提供 skip ID/reason，但完整 report 只汇总 `diversity_skipped`，不逐项输出 reason。若换专用本地 reranker，应重新跑本课 validation 门禁，不能由 L38 代替决定。

## 10. 面试怎么说

我把 reranker 设计成窄接口：只能为原候选 ID 打有限相关性分。Ollama 用 temperature 0 与 JSON Schema 限制输出，Go 再检查 ID 精确覆盖；连接或协议失败就保留 RRF 原顺序并告警，权限/ownership 错误硬失败。随后稳定执行 document≤3、heading≤2 多样性。当前 validation 更慢且 NDCG 更差，所以默认关闭。

## 11. 自检题与答案

问：模型可否新增一个 candidate ID？答：不可以，立即协议 fallback。问：同一 document+heading 的上限？答：2。问：Reranker 失败是否总可降级？答：只有服务/JSON/coverage 等可回退错误；scope/ownership 不可。问：L38 网格包含 `rerank_enabled` 吗？答：不包含。

## 12. 事实锚点

- 源课：[reranker-diversity-sop.md](../reranker-diversity-sop.md)
- 入口：`cmd/retrieval-quality-demo/rerank.go`
- 实现：`internal/retrievalquality/{rerank,rerank_ollama,diversity}.go`
- 前置：[L36](L36-hybrid-rrf.md)，后续：[L38](L38-retrieval-decision-policy.md)
