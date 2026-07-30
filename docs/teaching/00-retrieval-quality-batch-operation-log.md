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
