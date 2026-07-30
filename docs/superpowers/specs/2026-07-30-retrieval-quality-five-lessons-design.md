# Retrieval Quality Five Lessons Design

**Date:** 2026-07-30

**Status:** Approved：方案 A，直接在当前分支实施

## 1. Goal

完成 L34-L38，让当前离线 RAG 项目能够在固定、版本化的数据集上比较
Dense、Sparse、Hybrid 和 Reranker，最终通过经数据校准的决策策略选择检索路径。

每节必须包含：

- 聚焦且有明确边界的 Go 实现
- 生产代码之前观察到正确失败的 RED 测试
- 通过的单元测试和可重复执行的真实本地命令
- `docs/teaching` 下对应的教学 SOP
- 质量、延迟、资源成本和生产边界记录
- 审核和独立 commit

实现完成只更新为“已实现待学习”，不能记录为用户“已学习”。

## 2. Approved Local Boundary

只使用已有本地资产：

- Qdrant：`http://127.0.0.1:6333`
- Ollama：`http://127.0.0.1:11434`
- Dense embedding model：`bge-m3`
- 可选本地 Reranker model：`qwen:7b`
- tokenizer：`assets/tokenizers/qwen2/tokenizer.json`
- 忽略的本地配置：`config/recent-chat.env`

本批次创建隔离资源：

- physical collection：`offline_rag_retrieval_quality_lab_v1`
- stable alias：`offline_rag_retrieval_quality_lab_active`

不删除任何 physical collection，不修改 L25、L31-L33、Memory 或 recent-chat
已有 collection。新 collection 必须通过精确名称校验；命令不能接受空名称或将既有
collection 误作为目标。旧数据和本批数据之间不做原地迁移。

不增加 Elasticsearch/OpenSearch、云模型、远程数据库或新的常驻服务。不开新分支，
不 push。

## 3. Selected Architecture

新增聚焦包 `internal/retrievalquality`，而不是继续扩张
`internal/documentingest`。现有 L33 evaluator 作为固定 Dense 教学基线保留，新包
负责生产数据集、Sparse 编码、Hybrid 融合、Reranker、多样性、校准和决策策略。

数据流为：

```text
versioned corpus + production golden dataset
  -> deterministic validation and checksum
  -> local bge-m3 dense vectors
  -> local tokenizer field-aware sparse vectors
  -> isolated Qdrant named-vector snapshot
  -> dense and sparse candidate legs
  -> weighted RRF
  -> optional local reranker
  -> diversity constraint
  -> calibrated retrieval decision policy
  -> validation-only quality/latency/cost report
```

领域计算保持纯 Go，可由单元测试直接验证。Qdrant 和 Ollama 通过窄接口隔离，HTTP
协议使用 `httptest` 验证；课程验收再使用真实本地服务。

## 4. L34: Versioned Production Golden Dataset

### 4.1 Dataset contract

数据集目录为 `internal/retrievalquality/testdata/golden/v1`，包含：

- `manifest.json`：dataset ID、version、固定 split、corpus checksum 和 case checksum
- `corpus.json`：可以独立重建检索 snapshot 的已审核 chunks
- `cases.json`：固定查询与标注

v1 至少包含 40 个 cases、两个隔离的 knowledge scopes，并覆盖五类查询：

- `exact`：错误码、配置键、完整术语
- `code`：函数、类型、标识符
- `semantic`：没有原文关键词的释义
- `mixed`：同时包含概念与精确锚点
- `negative`：当前 scope 内没有可支持结果

每个非 negative case 包含 1-5 个分级相关标注，等级为 1-3；每个 case 包含至少一个
forbidden chunk。negative case 的相关标注为空，但 forbidden chunk 非空。case
显式保存 `split=train|validation`，v1 固定 24 个 train 和 16 个 validation cases；
策略参数只能读取 train，validation 只允许生成报告。

Corpus chunk 保存 knowledge scope、document ID、chunk ID、title、heading path、
source ref、content、content hash 和结构类型。Dataset loader 校验唯一性、引用完整性、
scope 一致性、相关/forbidden 不相交、split 数量、类别覆盖及 SHA256 checksum。

“Production”表示它具有生产评估所需的版本、审阅、隔离、负例和可复现契约，不表示
这 40 个仓库内样例能够证明真实业务泛化。接入真实匿名生产 queries 是后续运营工作。

### 4.2 Metrics and baseline

评估器支持任意 K，并固定报告：

- Recall@3、Recall@10
- MRR@10
- NDCG@10（使用分级相关性）
- negative pass rate
- scope isolation 和 forbidden-hit count
- latency p50/p95
- 每种 query kind 的同组指标
- failure stage：dataset、parsing、chunking、dense recall、sparse recall、fusion、
  rerank、diversity、isolation 或 policy

L34 的 baseline 使用现有 `bge-m3` Dense 查询，参数和 dataset checksum 一起记录。
L34 不要求分数达到人为设定的高阈值；它要求 40 个 case 全部执行、scope isolation
为 100%、forbidden hit 为 0，并产出可供 L35-L38 比较的不可变 baseline。

## 5. L35: Field-aware Sparse Retrieval

Qdrant collection 使用 named vectors：

```text
dense:
  size: 1024
  distance: Cosine
sparse:
  modifier: idf
```

Sparse encoder 使用仓库内 Qwen tokenizer 产生的 token ID 作为 sparse index。同一
chunk 分别编码 title、heading path、source path、body/code，再合并 term frequency。
固定字段权重为：

```text
title=3.0
heading_path=2.0
source_ref=1.5
body_or_code=1.0
```

每个字段采用 BM25 风格的 TF 饱和和长度归一化，固定 `k1=1.2`、`b=0.75`；
Qdrant `idf` modifier 负责 collection 级 IDF。零值被移除，indices 严格递增，values
必须为有限正数。Payload 保存 tokenizer checksum、sparse policy version 和 embedding
model，查询结果重新校验 scope、point identity、content hash 和 encoder identity。

Sparse 查询仍强制服务端 `knowledge_scope` filter。L35 独立报告与 L34 Dense baseline
相同的指标，重点呈现 exact/code 两类差异，但不以牺牲 semantic 或隔离门禁换取提升。

## 6. L36: Explainable Hybrid Fusion

一次 Hybrid 查询分别执行 Dense 和 Sparse 两路，二者使用同一 scope filter，并各自
取 top 20。融合在 Go 中执行 weighted Reciprocal Rank Fusion：

```text
rrf_score = dense_weight / (60 + dense_rank)
          + sparse_weight / (60 + sparse_rank)
```

默认权重均为 1.0。相同 chunk 按稳定 identity 合并；分数相同依次按最佳单路 rank、
chunk ID 排序。每个结果保留 dense rank/score、sparse rank/score、各路 RRF
contribution 和最终原因。禁止直接相加不可比较的 Dense/Sparse 原始分数。

运行时 Sparse 基础设施不可用时可以带 warning 降级到 Dense；payload、ownership、
scope 或 point identity 错误仍是 hard failure。在评估命令中任一路不可用会使该策略
报告失败，不能把降级结果冒充 Hybrid 对比。

## 7. L37: Reranker and Diversity

`Reranker` 是窄接口，输入 query 和固定候选，输出每个原候选 ID 的有限 relevance
score。第一版真实实现使用本地 Ollama `qwen:7b`，temperature 为 0，并要求严格 JSON：

- 每个输入 candidate 恰好出现一次
- 不能缺少、重复或增加 candidate ID
- relevance score 必须为有限数
- response 不能依赖模型重写 chunk identity

只重排 Hybrid top 20。Ollama 超时、协议错误或非法 JSON 时，运行时保留原始 RRF
顺序并记录 warning；ownership 或输入候选非法仍 hard fail。课程不会把通用生成模型
描述为专用 cross-encoder，未来可以在不改上层接口的情况下替换为
`bge-reranker`。

Rerank 后执行确定性 diversity：同一 document 最多 3 条，同一 heading path 最多
2 条；被跳过项及原因进入诊断报告。若 Reranker 在 validation 上没有改善 NDCG@10，
实现仍保留，但默认策略不启用它。

报告同时包含无 Reranker fallback，记录 Recall/NDCG、rerank p50/p95、调用次数、
输入 candidate 数和使用的本地模型。

## 8. L38: Calibrated Retrieval Decision Policy

L38 不训练新模型。它从 train split 的标注和候选特征中产生版本化策略：

- 用 deterministic isotonic regression（PAVA）分别校准 Dense、Sparse 和 Hybrid
  score 到 `[0,1]`
- 报告 train 与 validation 的 Brier score、ECE 和阈值行为
- 按 query kind 选择候选策略：exact/code 偏 Sparse，semantic 偏 Dense，mixed
  选择 Hybrid，unknown 使用等权 RRF
- 根据 train 数据在有限候选网格中选择 dense/sparse weights、candidate quota、
  rerank enabled 和 minimum relevance threshold
- 选择顺序固定为 validation-independent 的 train NDCG@10、Recall@10、p95 latency、
  参数字典序，保证重复运行得到同一 policy

Policy artifact 保存 dataset checksum、corpus checksum、encoder identity、候选参数、
校准分段、生成时间之外的确定性 policy checksum。加载时身份不匹配即拒绝运行。

每个请求输出策略名称、query kind、权重、quota、Reranker 是否启用、fallback 和
decision reasons。validation 只用于最终报告与回归门禁，不能参与选参。

回归门禁为：

- validation 的 scope isolation 必须为 100%
- forbidden-hit count 必须为 0
- negative pass rate 不低于 L34 baseline
- 最终策略 NDCG@10 和 Recall@10 均不得低于 Dense baseline
- 最终策略 retrieval p95 不得超过同一批运行 Dense baseline 的 3 倍
- 任一依赖退化时能回到确定性等权 RRF 或 Dense baseline，并明确报告原因

## 9. Commands and Artifacts

使用一个可组合命令 `cmd/retrieval-quality-demo`，以 subcommand 表示课程边界：

```text
dataset    validate dataset and run dense baseline
index      create/validate and populate the isolated named-vector collection
sparse     run sparse-only evaluation
hybrid     run dense/sparse legs and weighted RRF evaluation
rerank     run optional Ollama reranking and diversity evaluation
policy     fit on train, evaluate on validation, and write the policy report
```

命令默认只读；`index --apply` 是唯一写 Qdrant 的 subcommand。它只能写固定的隔离
physical collection。Alias 激活必须显式 `--activate`，且只指向已验证 collection。
所有报告输出稳定 JSON 到标准输出；操作日志记录真实命令、服务身份、dataset checksum、
观察结果和时间。

新增教学文件：

- `docs/teaching/production-golden-dataset-sop.md`
- `docs/teaching/field-aware-sparse-retrieval-sop.md`
- `docs/teaching/hybrid-retrieval-fusion-sop.md`
- `docs/teaching/reranker-diversity-sop.md`
- `docs/teaching/retrieval-decision-policy-sop.md`
- `docs/teaching/00-retrieval-quality-batch-operation-log.md`

实现完成后同步：

- `docs/teaching/00-course-blueprint.md`
- `docs/teaching/00-learning-status.md`
- `docs/teaching/00-handoff-guide.md`
- `docs/teaching/00-optimization-backlog.md`
- `world/game-world-map.md`

课程实现坐标移动到 L38，下一次新实践为 L39；教学坐标仍从 L24 继续。

## 10. Error and Security Boundaries

Hard failures：

- 数据集 checksum、split、引用、scope 或标注非法
- collection/vector/sparse 配置与固定契约不一致
- query 缺少 scope 或服务端 filter 未发送
- Qdrant 返回跨 scope、未知 point、错误 identity/content hash
- encoder、embedding model、dataset 或 policy identity 不一致
- 评估时 Dense/Sparse leg 缺失
- Reranker 修改、遗漏或虚构 candidate identity

Runtime warnings with deterministic fallback：

- Sparse 查询基础设施不可用：降级 Dense
- Reranker 超时或响应协议非法：保留 RRF 顺序
- policy artifact 不可用但索引身份有效：使用等权 RRF

Fallback 不能隐藏 hard failure，也不能把降级运行记录为完整策略验收。

## 11. Test and Commit Gates

每节按 RED、GREEN、真实实践、SOP、审核、commit 顺序执行。单元测试至少覆盖：

- dataset 契约、checksum、NDCG/negative/latency 和失败分类
- sparse vector 的字段权重、BM25 归一化、确定顺序和非法值
- Qdrant named-vector schema、scope filter、payload revalidation
- weighted RRF、tie-break、单路降级与评估 hard failure
- Reranker 严格协议、fallback 和 diversity cap
- PAVA 单调性、policy identity、train-only 调参和回归门禁

最终验证：

```text
go test ./...
go run ./cmd/retrieval-quality-demo dataset ...
go run ./cmd/retrieval-quality-demo index ... --apply --activate
go run ./cmd/retrieval-quality-demo sparse ...
go run ./cmd/retrieval-quality-demo hybrid ...
go run ./cmd/retrieval-quality-demo rerank ...
go run ./cmd/retrieval-quality-demo policy ...
```

所有单元测试通过；真实 Qdrant/Ollama 运行记录完整；五节各有独立 commit。只 stage
本批次已知文件，不 stage 三个无关的未跟踪文档。整个批次不创建分支、不 push。
