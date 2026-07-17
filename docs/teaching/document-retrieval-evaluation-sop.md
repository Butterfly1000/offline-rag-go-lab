# 第 33 节：Golden Cases 检索评估

本节把“检索感觉不错”变成可重复门禁。真实入口：

```bash
go run ./cmd/document-eval-demo --config config/recent-chat.env \
  --alias offline_rag_document_ingestion_lab_active \
  --golden internal/documentingest/testdata/golden_queries.json --k 3
```

## 1. 真实结果

```json
{
  "case_count": 12,
  "mean_recall_at_3": 1,
  "mean_mrr_at_3": 1,
  "scope_isolation": 1,
  "passed": true
}
```

12 个 cases 覆盖 Markdown identity/version/storage/snapshot、Go struct/method/function/code，以及 3 个 analytics competing-scope 问题。每个 case 的 forbidden hits 都为空。

## 2. Recall@3 如何计算

代码：[evaluate.go](/offline-rag-go-lab/internal/documentingest/evaluate.go:1)

```text
Recall@3 = 前三名中命中的不同 expected IDs 数 / expected IDs 总数
```

例如 expected=`[a,b]`，前三名只出现 `a`，Recall@3=0.5。重复 `a` 不算两次；当前实现把重复结果 ID 当数据错误直接失败。

## 3. MRR@3 如何计算

```text
MRR case = 1 / 第一个 expected hit 的排名
```

第 1/2/3 名首次命中分别为 1、0.5、0.333；前三名没有 expected 则为 0。最终 `mean_mrr_at_3` 是所有 cases 的平均值。

## 4. Scope 隔离为什么不是普通扣分

Qdrant 查询先按 `knowledge_scope` filter，返回后 Go 再检查 payload ownership。任何 hit 的 scope 与 case 不同会让整个评估返回错误，不继续计算一个看似正常的分数。

每个 Golden Case 还必须包含 forbidden IDs。主课程 cases 使用 analytics IDs 作为 forbidden；analytics cases 使用主课程 IDs。这同时验证服务端 filter 和返回后校验。

## 5. Golden Case 规则

数据：[golden_queries.json](/offline-rag-go-lab/internal/documentingest/testdata/golden_queries.json:1)

- 至少 10 cases
- case ID 唯一，输出按 case ID 排序
- expected IDs 为 1-3 个且唯一
- forbidden IDs 至少 1 个，与 expected 不重叠
- 每个 case 只 embedding 一次
- search limit 固定为 3，CLI 不接受其他 K
- empty/duplicate IDs、跨 scope、非有限 embedding 都硬失败

## 6. 生产边界

本次 1.0 只说明这 12 个本地 fixture 问题通过，不能证明真实用户问题也有同样效果。生产中应从失败日志、人工标注和真实措辞持续扩充 dataset，并按版本保存评估报告。

后续优化（reranker、动态 quota、chunk policy）必须先增加能暴露问题的 cases，再比较基线，不能只改参数后挑几个成功示例。
