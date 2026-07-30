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
