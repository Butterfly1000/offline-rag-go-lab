# Retrieval Quality Batch Operation And Impact Log

主题：第 34-38 节执行过程、外部状态影响和验证证据。

本日志记录实现行为，不表示用户已经学会对应课程。

## 授权和资源边界

- 直接在当前分支执行，不创建或切换分支
- 每节执行 RED -> GREEN -> 真实实践/SOP -> review -> 独立 commit
- 不执行 `git push`
- 只使用本地 Ollama、Qdrant 和仓库 tokenizer
- 只允许写 `offline_rag_retrieval_quality_lab_v1` 和
  `offline_rag_retrieval_quality_lab_active`
- 不删除物理 collection
- 不修改 Memory、recent-chat、L25 或 L31-L33 collection
- 三个无关的未跟踪 AirDroid 文档不读取、不 stage、不修改

## 第 34 节：版本化 Production Golden Dataset

### 影响分析

本节只新增纯 Go 数据集/指标/Dense 内存基线、固定 JSON、CLI 和教学文档。真实实践
只读取仓库文件并调用本地 Ollama `bge-m3`；没有连接 MySQL，没有读写 Qdrant。

### RED 证据

领域层：

```bash
go test ./internal/retrievalquality
```

结果：FAIL。编译器报告 `SearchResult`、`Query`、`Dataset`、`LoadDataset` 等目标 API
不存在。

命令层：

```bash
go test ./cmd/retrieval-quality-demo
```

结果：FAIL。编译器报告 `buildDatasetReport` 不存在。

失败都发生在生产实现之前，且原因是目标能力缺失。

### GREEN 证据

```bash
go test ./internal/retrievalquality ./cmd/retrieval-quality-demo
```

结果：PASS。

测试覆盖 dataset checksum、固定 split/category/scope、分级 relevance、未知/跨 scope
引用、相关与 forbidden 重叠、Recall/MRR/NDCG、负例、p50/p95、评估跨 scope 硬失败、
Dense scope-first filtering 和稳定 cosine 排序。

### 真实运行证据

```bash
go run ./cmd/retrieval-quality-demo dataset \
  --config config/recent-chat.env \
  --dataset internal/retrievalquality/testdata/golden/v1
```

本地模型身份：

```text
bge-m3:latest
digest=7907646426070047a77226ac3e684fbbe8410524f7b4a74d02837e43f2146bab
parameter_size=566.70M
quantization=F16
```

结果：

```text
cases=40 (train=24, validation=16)
train Recall@3=1 Recall@10=1 MRR@10=0.975 NDCG@10=0.981546
train p50=95.057ms p95=178.670ms negative_pass=0
validation Recall@3=1 Recall@10=1 MRR@10=1 NDCG@10=1
validation p50=97.155ms p95=150.814ms negative_pass=0
scope_isolation=1 forbidden_hits=0 passed=true
```

### 当前边界

当前 Dense baseline 不会 abstain，因此 8 个负例全部失败；该结果被保留为 L38 阈值
门禁。Corpus 每个 scope 只有 12 chunks 而 K=10，所以 Recall@10 很高不能证明生产
泛化。L35 将在不改 dataset 的前提下建立独立 Sparse baseline。

## 第 35 节：Field-aware Sparse Retrieval

### 影响分析

Docker Desktop 原先未运行，本节先启动已安装的 Docker Desktop，再启动已有
`qdrant` 容器。没有新建容器或存储卷。

写操作严格限定为：

```text
offline_rag_retrieval_quality_lab_v1
offline_rag_retrieval_quality_lab_active
```

执行前 collection 清单只有：

```text
offline_rag_document_chunks_v1
offline_rag_document_ingestion_lab_v1
offline_rag_document_ingestion_lab_v2
offline_rag_memory_items_v1
ollama_chat_memory
```

原 alias `offline_rag_document_ingestion_lab_active -> ..._v2` 保持不变。

### RED 证据

领域层第一次编译报告 `NewQdrant`、`SparseVector`、`IndexedChunk`、
`StablePointID` 等不存在。第二轮报告 `Inspect`、`NewQdrantDenseStrategy` 和
`NewQdrantSparseStrategy` 不存在。

命令层：

```bash
go test ./cmd/retrieval-quality-demo -run 'TestIndexDataset|TestBuildSparseReport'
```

结果：FAIL，`indexDataset` 和 `buildSparseReport` 不存在。

### GREEN 证据

```bash
go test ./cmd/retrieval-quality-demo ./internal/retrievalquality
```

结果：PASS。包含真实 `httptest` HTTP 边界，不只检查 fake。

覆盖：

- 字段权重、BM25 饱和和长度归一化
- Sparse indices 排序/合并与非法 token/value
- named dense/sparse schema 和 `modifier=idf`
- named-vector upsert 与稳定 point ID
- Dense/Sparse 请求的强制 scope filter
- payload scope、content hash、model、encoder identity 重校验
- collection 验证先于 alias 激活

### 真实 index 证据

```bash
go run ./cmd/retrieval-quality-demo index \
  --config config/recent-chat.env \
  --dataset internal/retrievalquality/testdata/golden/v1 \
  --apply --activate
```

结果：

```text
collection=offline_rag_retrieval_quality_lab_v1
status=green points=24 indexed_vectors=24
dense=size 1024 distance Cosine
sparse=modifier idf
payload indexes=knowledge_scope,document_id,chunk_id
alias=offline_rag_retrieval_quality_lab_active
```

执行后原五个 collections 全部保留，只新增专用 retrieval-quality collection。全局
alias 同时保留 ingestion alias 和新增 retrieval-quality alias。

同一 `index --apply --activate` 再执行一次命中已有 schema 校验和 alias no-op 分支，
仍为 green/24 points，没有增加重复 point。

### 真实 Sparse 评估

```text
train Recall@10=0.775 MRR@10=0.8 NDCG@10=0.789358
validation Recall@10=0.666667 MRR@10=0.666667 NDCG@10=0.666667
validation exact/code/mixed NDCG@10=1
validation semantic NDCG@10=0
validation p50=12.408ms p95=15.021ms
scope_isolation=1 forbidden_hits=0
```

Sparse 的精确能力和语义缺口都由固定 L34 数据集暴露，没有为结果修改 case。

## 第 36 节：Explainable Hybrid RRF

### 影响分析

本节只读取已建好的 retrieval-quality alias，并调用本地 `bge-m3`；没有 Qdrant 写入、
collection/alias 变更、MySQL 连接或远端操作。

### RED 证据

领域层：

```bash
go test ./internal/retrievalquality -run 'TestFuseRRF|TestHybrid'
```

初始结果：FAIL，`FuseRRF`、`FusionWeights`、`NewHybridStrategy` 等不存在。

命令层：

```bash
go test ./cmd/retrieval-quality-demo -run TestBuildHybridReport
```

结果：FAIL，`buildHybridReport` 不存在。

### GREEN 过程中的真实缺陷

第一次实现后的测试不是立即通过，而是发现融合候选复制了 Dense 原始 `Score`：

```text
got=1.022522...
want=0.032522...
```

另一个测试把原始 score 改成百万和负数后，融合顺序也错误改变。修复为新融合候选的
score 从 0 开始，只累计两路 contribution。

Review 又新增“一条 leg 混合多个 scopes”测试，确认先 FAIL，再在 leg validation 中
锁定单一 scope。最终 package/command 全部 GREEN。

### 真实运行

命令连续执行两次，固定参数：

```text
dense_weight=1
sparse_weight=1
rank_constant=60
```

第二次精简结果：

```text
Dense validation  Recall@10=1 NDCG@10=1 p95=115.459ms
Sparse validation Recall@10=0.666667 NDCG@10=0.666667 p95=14.605ms
Hybrid validation Recall@10=1 NDCG@10=1 p95=148.116ms

Dense train  NDCG@10=0.981546
Hybrid train NDCG@10=0.979338

scope_isolation=1 forbidden_hits=0 passed=true
```

两次运行的 trace top 5 顺序和 contribution 相同。真实 `rq-pava` 同时为
dense_rank=1、sparse_rank=1，两个 contribution 均为 0.01639344，最终
RRF=0.03278689。

### 当前边界

等权 Hybrid 在 validation 与 Dense 持平，在 train 略低，当前证据不足以声称默认
优于 Dense。L37 只实现可插拔 Reranker；是否默认启用仍由 validation 结果决定。

## 第 37 节：Local Reranker and Diversity

### 影响分析

本节只读取 `offline_rag_retrieval_quality_lab_active`，调用本地 `bge-m3` 和
`qwen:7b`。没有 Qdrant 写入、collection/alias 变更、Docker 数据变更、MySQL 或
远端操作。

### RED / GREEN 证据

领域、HTTP、策略和命令报告依次通过 RED：

```text
RerankScore / ApplyReranker / ApplyDiversity missing
NewRerankedStrategy missing
buildRerankReport / sameCandidateOrder missing
```

实现后聚焦测试全部 GREEN，覆盖：

- exact ID coverage、unknown/missing/duplicate/non-finite
- `temperature=0`、untrusted candidate prompt、strict response decode
- backend/protocol failure 保留原 RRF
- document≤3、heading≤2、稳定顺序和不修改输入
- scope/ownership hard failure
- validation-only 报告与“未严格提升就不默认启用”

### 首次真实失败与根因

首轮 16/16 回退：15 次返回合法 JSON 但字段为 `candidate_ id`，一次漏掉 11 个
候选。forced fallback 顺序正确，但真实 reranker 使用次数为 0，所以命令按执行门禁
失败。

系统化定位确认请求只有通用 `"format":"json"`，无法约束业务字段。新增 RED 测试后，
把 format 改为动态 JSON Schema：候选数固定、`candidate_id` 枚举固定、未知字段
禁止、relevance 为 number。两候选本地最小实验通过后才重跑整批。

### 最终真实运行

```text
Hybrid validation:
Recall@10=1 NDCG@10=1 p50=105.242ms p95=2.339s

Rerank + diversity validation:
Recall@3=0.833333 Recall@10=0.916667
MRR@10=0.808333 NDCG@10=0.834815
p50=28.665s p95=34.156s

calls=16 used=11 fallbacks=5 input_candidates=192
diversity_skipped=0 reranker_p50=26.948s reranker_p95=31.894s
scope_isolation=1 forbidden_hits=0
forced_fallback_same_rrf_order=true
default_enabled=false
```

剩余 5 次协议回退都是 duplicate candidate ID。JSON Schema 保证结构和枚举，但不能
保证 enum 数组不重复；Go 的 exact coverage 校验正确拦截并保留原 RRF。

### 当前边界

`qwen:7b` 是通用生成模型，不是专用 cross-encoder。本次 validation 质量下降且延迟
显著增加，因此只保留可替换实现与安全回退，默认策略关闭 Reranker。L38 将只用 train
选择策略，validation 仅做最终回归门禁。

## 第 38 节：Calibrated Retrieval Decision Policy

### 影响分析

本节只读本地 retrieval-quality Qdrant collection 和 Ollama embedding。唯一写入是
Git 忽略的 `.cache/retrieval-quality/policy-v1.json`；没有修改 collection、alias、
Docker 数据、MySQL 或远端状态。

### RED / GREEN 证据

依次完成并验证：

- PAVA 相邻块合并、重复 score、单调/clamp、非法输入；
- Brier/ECE 固定算例；
- 固定 split 候选采样和二元 label；
- validation trap 不参与 selection；
- exact/code/semantic/mixed/unknown 路由；
- grid canonical tie、dataset/encoder/checksum identity；
- calibrated threshold abstain；
- infrastructure fallback 与 integrity hard failure；
- scope、forbidden、negative、Recall、NDCG、3× p95 六类回归门禁；
- 162/81 完整/可行网格和缓存重放。

### 真实策略选择

完整 grid 162，L37 默认关闭的 rerank=true 排除后评估 81 组，形成 396 条 train
outcome。validation 在 policy 确定后才运行。

最终路由：

```text
code     -> Dense  weights=1/1   quota=10 threshold=0.25 rerank=false
exact    -> Hybrid weights=.5/.5 quota=10 threshold=0.25 rerank=false
mixed    -> Hybrid weights=.5/.5 quota=10 threshold=0.25 rerank=false
semantic -> Hybrid weights=.5/.5 quota=10 threshold=0    rerank=false
unknown  -> Hybrid weights=1/1   quota=10 threshold=0    rerank=false
```

### 确定性缺陷与修复

首版用纳秒 p95 打破质量并列，重复运行的 semantic 权重从 `.5/2` 变成 `1/2`，
checksum 漂移。25ms/250ms 细等级仍会被模型 warm/cold 跨越。

最终改为操作级 latency tier：`<=5s`、`<=60s`、`>60s`；同等级使用 canonical
params。精确性能由 validation 3× 门禁负责。修复后连续两次：

```text
checksum=737d487569d553249941f979450f9751fae34715fa393cc1543b7ba116d30310
routes/params 完全相同
```

### 最终真实门禁

最后一次运行：

```text
Dense validation  Recall@10=1 MRR@10=1 NDCG@10=1 p95=113.665ms
Policy validation Recall@10=1 MRR@10=1 NDCG@10=1 p95=111.473ms
negative_pass=0
scope_isolation=1 forbidden_hits=0
regression_passed=true failures=[]
```

校准 validation Brier/ECE：

```text
Dense  0.012097 / 0.017650
Sparse 0.010801 / 0.016990
Hybrid 0.031773 / 0.014947
```

### 当前边界

负例通过率仍为 0，只满足“不低于 Dense baseline”，没有证明无依据拒答。L39-L43
继续解决 evidence、citation、refusal 和回答质量；本批次按用户要求停在 L38。
