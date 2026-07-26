# L33：文档检索评估

对应源课：`docs/teaching/document-retrieval-evaluation-sop.md`

## 1. 本课一句话

对固定 golden fixture 的每个 query 只取 top 3：Recall@3 是找到的 expected 唯一 chunk ID 数除以 expected 总数，MRR@3 是第一个 expected 的倒数排名；所有 case Recall=1 且 forbidden=0 才 Passed，跨 scope 则直接硬失败。

## 2. 先用人话理解

评估不是“模型回答听起来不错”。先写好题目、正确资料 ID 和绝不能出现的 ID，再让同一个 embedding model 到指定 alias 检索三条。找全正确资料看 Recall，第一条正确资料排第几看 MRR；拿错 scope 的资料是越权，不能用平均分稀释。

## 3. 系统里有什么

`golden_queries.json` 有 12 个 case，每个包含 query、knowledge scope、1–3 个 expected unique IDs 与至少一个 forbidden ID。`Evaluate` 按 case_id 排序；每 case 用 Ollama embedding 一次，再以该 case scope 在 Qdrant alias 搜索固定 3 条。结果含逐 case 的 Recall@3、MRR@3、forbidden hits、retrieved IDs，以及均值、scope isolation、Passed。

## 4. 一条完整链路

读取 fixture → 校验至少 10 case、ID 不重复且 expected/forbidden 无交集 → 每 case 调 Ollama embedding → 对 configured alias 进行带 scope filter 的 top-3 Qdrant 查询 → 重验每个 hit 的 scope、chunk ID 非空且不重复 → 算指标 → 输出 JSON。任一步错误（包含跨 scope hit、embedding/Qdrant 错误）立即失败；指标不达 Passed 也以 exit 1 结束。

## 5. 实际怎么做

```bash
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go test ./internal/documentingest -run 'TestEvaluate' -count=1
env GOCACHE="$PWD/.cache/go-build" GOSUMDB=off go run ./cmd/document-eval-demo --config /absolute/path/to/your/recent-chat.env --alias offline_rag_document_ingestion_lab_active --golden internal/documentingest/testdata/golden_queries.json --k 3
```

- test 用 fake embedder/searcher 检查公式、稳定排序和 hard gate；安全重复，只写 build cache。
- demo 需要从 `config/recent-chat.env.example` 派生的私有配置、可用 Ollama embedding、以及 Qdrant alias；它对 alias 只读，但会为每个 case 调一次 Ollama embedding。`--alias` 必须精确等于配置的 `DOCUMENT_INGEST_ALIAS`，`--k` 必须为 3；失败或 `Passed=false` 都 exit 1。可安全重复；恢复时先检查服务、alias target、fixture 和 model，再重跑，而不是把失败当成分数 0。

## 6. 结果怎么看

`recall_at_3=1` 表示该 case 的所有 expected unique IDs 都在前三；`mrr_at_3=1` 表示首条就是 expected，`0.5` 表示首个 expected 位于第 2 位，没找到则 0。`passed=true` 只要求每 case Recall=1 且 forbidden hits 为空；没有“MRR 必须至少多少”的门槛。成功报告的 `scope_isolation` 为 1；跨 scope 是 error，根本不会作为较低的 isolation 分数继续输出。

## 7. 算一遍或走一遍

某 case 的 expected 是 `{A,B}`，top 3 返回 `[C,A,B]`，且 C 不 forbidden：Recall@3=`2/2=1`，第一个 expected 在 rank 2，所以 MRR@3=`1/2=0.5`，该 case 仍可通过。若返回 `[A,X,Y]`，其中 X 是 forbidden：Recall=0.5，forbidden 非空，失败。若任何 hit 的 scope 不是 case scope，立即 error，不会以“Recall 降一点”记账。

## 8. 常见误解

“MRR=0.5 就必然不通过”错误，目前没有 MRR 下限。“同一个 expected 出现两次可算两次”错误，重复 hit 直接 error，Recall 以 expected 唯一 ID 集合计。“跨 scope 是扣分项”错误，它是 integrity hard gate。“12 条全过就是生产质量”错误，12 条是受控 fixture，不代表真实用户、语料漂移、权限或延迟表现。

## 9. 当前实现与生产边界

当前 cutoff 固定 top 3，fixture 至少需 10 个 case，仓库样例正好 12 个。输出数字只能绑定在一次具体运行：记录 fixture 文件及其版本/内容、`OLLAMA_EMBED_MODEL`、`DOCUMENT_INGEST_ALIAS` 及 resolve 得到的 target、运行时间和代码版本；当前 JSON 报告本身不自动携带 model、alias 或时间，不能脱离这些上下文横向比较。它不替代生产抽样、人工相关性标注、在线质量或延迟监控。

## 10. 面试怎么说

我用固定 golden case 评估检索而不是评估生成回答：每题只搜 top 3，Recall@3 检查 expected 是否找全，MRR@3 描述首个正确结果的位置。发布 gate 是每题 Recall=1 且无 forbidden hit；scope 泄漏直接报错。每次报表都与 fixture、模型、alias target 和运行时间一起记录，避免把不可比的数字混在一起。

## 11. 自检题与答案

问：expected 为 3 个、找到 2 个，Recall@3 是多少？答：`2/3`。问：首个 expected 在第 3 位，MRR@3 是多少？答：`1/3`。问：MRR=0.5、Recall=1、无 forbidden 是否 Passed？答：是。问：scope 错误为何不计入平均分？答：它是越权完整性错误，函数直接返回 error。问：本课 fixture 有几条？答：12 条。

## 12. 事实锚点

代码锚点：`internal/documentingest/evaluate.go`、`internal/documentingest/evaluate_test.go`、`internal/documentingest/testdata/golden_queries.json`、`cmd/document-eval-demo/main.go`。建议先读 [L29](L29-document-identity.md) → [L30](L30-structured-chunking.md) → [L31](L31-idempotent-ingestion.md) → [L32](L32-snapshot-alias.md) → 本课；全课程阶段索引在 [课程地图](00-course-map.md)。
