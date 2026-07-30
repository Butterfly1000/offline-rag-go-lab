# 第 37 节：本地 Reranker、多样性与安全回退

本节在 L36 Hybrid top 20 之后增加一个可替换的 `Reranker` 接口。第一版用本地
Ollama `qwen:7b` 做相关性评分，但课程不会把通用生成模型描述成专用
cross-encoder，也不会因为“多了一层 AI”就默认启用。

真实入口：

```bash
go run ./cmd/retrieval-quality-demo rerank \
  --config config/recent-chat.env \
  --dataset internal/retrievalquality/testdata/golden/v1 \
  --model qwen:7b
```

该命令只读取本地 Qdrant，调用本地 `bge-m3` 和 `qwen:7b`，不会写 collection。

## 1. Reranker 到底做什么

Hybrid 先返回最多 20 个候选。Reranker 只接收：

```text
query
candidate_id + content
```

它为每个输入候选返回一个 relevance：

```json
{
  "scores": [
    {"candidate_id": "rq-pava", "relevance": 0.91}
  ]
}
```

Go 不允许模型创造或改写 identity。响应必须满足：

1. 每个输入 `candidate_id` 恰好出现一次；
2. 不允许未知、缺少或重复 ID；
3. relevance 必须是有限数；
4. 分数降序；相同分数按原 RRF rank，再按 chunk ID；
5. 原候选 trace 保留，并新增 reranker score 和 original rank。

候选正文在 prompt 中被标记为不可信数据。正文里的“忽略先前指令”等文字不能改变
评分任务。

## 2. 为什么不能只写 `format=json`

第一次真实运行使用：

```json
{"format": "json", "options": {"temperature": 0}}
```

16 次全部进入安全回退。其中 15 次把字段写成了 `candidate_ id`，另一次缺少 11 个
ID。根因不是 Go 解码器太严格，而是 `format=json` 只保证结果是合法 JSON，不保证
业务 schema。

修复后，`format` 直接传入 JSON Schema：

```text
top-level additionalProperties=false
scores.minItems = scores.maxItems = 输入候选数
candidate_id.enum = 全部输入 ID
relevance.type = number
```

Ollama 负责生成期结构约束，Go 仍在返回后独立检查 exact coverage。两层不能互相
替代：JSON Schema 能限制字段和值域，但数组仍可能重复同一个 enum 值。

两候选最小真实实验首先通过，随后才重跑完整 validation，避免直接重复一个六分钟
批次。

## 3. 失败时怎么处理

以下属于可回退错误：

- Ollama 连接失败或超时；
- 非法 JSON；
- 字段、ID coverage 或 relevance 协议不合法。

回退行为固定为：

```text
保留原 RRF candidate 顺序
reason=reranker_fallback
记录 warning 和耗时
```

以下属于 hard failure，不能降级掩盖：

- 输入候选跨 `knowledge_scope`；
- 空、重复或非法 chunk ID；
- 缺少 document/heading ownership；
- 查询 scope 与候选 scope 不一致。

真实 forced fallback 使用 `http://127.0.0.1:1`。连接被拒绝后，10 个候选 ID 与原
RRF 顺序逐项相同，warning 明确记录断连原因。

## 4. 多样性怎么做

只在合法 rerank 或合法 fallback 候选上做一次稳定扫描：

```text
同一 document 最多 3 条
同一 document + heading path 最多 2 条
```

先出现的候选先保留，不重新排序。被跳过项记录 `document_limit` 或
`heading_limit`。算法不会修改输入 slice 或原 trace。

当前黄金 corpus 每个候选的 document/heading 分布没有触发上限，所以完整
validation 的 `diversity_skipped=0`；单元测试使用集中候选证明上限和跳过原因。

## 5. 2026-07-30 真实结果

本机模型：

```text
qwen:7b
digest=2091ee8c8d8f27f790c298265b1353da099a276e6c612e3b4864e72f1993cc33
family=qwen2
parameter_size=8B
quantization=Q4_0
```

固定 validation 对比：

| Strategy | Recall@3 | Recall@10 | MRR@10 | NDCG@10 | p50 | p95 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Hybrid RRF | 1.0000 | 1.0000 | 1.0000 | 1.0000 | 105.24 ms | 2.339 s |
| Rerank + diversity | 0.8333 | 0.9167 | 0.8083 | 0.8348 | 28.665 s | 34.156 s |

Reranker 统计：

```text
calls=16
used=11
fallbacks=5
input_candidates=192
diversity_skipped=0
reranker p50=26.948s
reranker p95=31.894s
scope_isolation=1
forbidden_hits=0
forced_fallback_same_rrf_order=true
```

5 次 fallback 都是模型重复候选 ID，证明 JSON Schema 不能代替 Go 的 exact
coverage 校验。

结论：validation NDCG 从 1.0 降到 0.8348，延迟增加两个数量级。因此实现保留，
但 `default_enabled=false`。未来替换为本地专用 reranker 时，不需要修改 Hybrid、
diversity 或 evaluator 接口。

## 6. 面试复述

> 我先用 Hybrid top 20 召回，再通过窄 Reranker 接口给原候选评分。Ollama 请求用
> temperature 0 和 JSON Schema 约束字段、数量与 ID 枚举，Go 返回后还会检查每个
> ID 恰好一次。服务或协议失败就保留原 RRF 顺序并告警，scope/ownership 错误必须
> 硬失败。重排后做 document≤3、heading≤2 的稳定多样性过滤。真实 qwen:7b 没有
> 改善 validation，反而更慢更差，所以代码保留但默认关闭。
