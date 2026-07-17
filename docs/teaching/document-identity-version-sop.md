# 第 29 节：生产文档身份、版本与稳定 Chunk ID

本节先解决生产 ingestion 的第一个问题：**系统如何知道这是同一篇文档、同一个版本，以及同一个 chunk。**

本节只运行 Go 代码和读取 SQL 文件，不连接 MySQL、Qdrant 或 Ollama。

## 1. 先实践效果

在仓库根目录执行：

```bash
go run ./cmd/document-identity-demo
```

关键输出：

```text
== Stable Chunk Identity ==
Base:      5b656571fd18ddd1b46f56e84b14ba4a96a7f6c3e971d63518904d7cbd66b67b
Unchanged: 5b656571fd18ddd1b46f56e84b14ba4a96a7f6c3e971d63518904d7cbd66b67b (same=true)
Changed:   383c81d2253eae4315eae2f0b8ef5908cc820328d48f0d6e49d934460b2e211c (same=false)
Moved:     7347cf961a05523ffa9dd0c67daae37b5ea6eb47385fc7d02d74f310504da89b (same=false)
Duplicate: fc06ded9e57602a1171f769cfbad102593aca3022d195e6ed410362dde68f32a (same=false)
```

先只看行为：

1. 内容和结构都没变，ID 不变。
2. 内容改变，ID 改变。
3. 内容没变但移动到其他章节，ID 改变。
4. 同一章节重复出现相同内容，用 `duplicate_ordinal` 得到不同 ID。

这就是“稳定”的真实含义：**不是永远不变，而是没有影响身份的变化时不变。**

## 2. Part 1：三种身份不能混在一起

### 2.1 逻辑文档身份

一篇文档由下面两个字段共同确定：

```go
KnowledgeScope string
DocumentID     string
```

例如：

```text
knowledge_scope = document-ingestion-course
document_id     = course-markdown
```

不用绝对文件路径做主键，因为同一项目在公司电脑和家里电脑的根目录不同。`source_ref` 只保存可移植的仓库相对路径，用于定位和引用。

相关代码：

- [types.go](/offline-rag-go-lab/internal/documentingest/types.go:1)
- [identity.go](/offline-rag-go-lab/internal/documentingest/identity.go:1)

### 2.2 文档版本身份

版本不是手工写 `v1`、`v2`，而是由构建输入确定：

```text
content_hash + parser_version + chunk_policy_hash + target_collection
```

其中：

- `content_hash`：规范化后的完整源文件 SHA256。
- `parser_version`：解析行为版本，例如 `markdown-v1`。
- `chunk_policy_hash`：格式、最大 token 和 overlap 等策略的 SHA256。
- `target_collection`：本次构建写入的物理 Qdrant collection。

文件没变但切块策略变了，也必须产生新构建，不能误判为 no-op。

### 2.3 Chunk 身份

稳定 `chunk_id` 的输入是：

```text
knowledge_scope NUL document_id NUL structure_kind NUL heading_path NUL
normalized_content_hash NUL duplicate_ordinal
```

`NUL` 是 `\x00` 分隔字节。它避免下面两组字段拼接成相同字符串：

```text
ab + c
a  + bc
```

## 3. Part 2：Go 如何计算 SHA256 身份

核心调用是：

```go
sum := sha256.Sum256([]byte(canonical))
id := hex.EncodeToString(sum[:])
```

相关内置/标准库行为：

- `[]byte(canonical)`：把字符串转成参与哈希的 UTF-8 字节。
- `sha256.Sum256`：返回固定 32 字节摘要；相同字节输入一定得到相同结果。
- `sum[:]`：把固定长度数组转换成可传给编码函数的切片。
- `hex.EncodeToString`：每个字节编码为两个十六进制字符，所以结果长度是 64。

SHA256 不是相似度算法。文本只改一个字，结果通常就完全不同；这正适合做确定性身份，不适合做语义检索。

## 4. Part 3：为什么先规范化文本

当前 `normalizePortableText` 做三件事：

1. 把 Windows `\r\n` 和旧式 `\r` 统一为 `\n`。
2. 删除每行末尾的空格和 tab。
3. 删除文件首尾的空白行，但保留正文内部换行和行首缩进。

因此下面两个文件不会因为操作系统换行和无意义行尾空格产生两个版本：

```text
heading\ntext\n
heading  \r\ntext\r\n\r\n
```

不能简单使用“删除所有空白”，因为 Go 代码、Markdown 代码块和自然语言内部空格可能有意义。

## 5. Part 4：版本为什么需要状态机

允许的状态变化只有：

```text
pending -> building -> ready -> active
                    -> failed
failed  -> building
```

相关代码：

- [state.go](/offline-rag-go-lab/internal/documentingest/state.go:1)

状态含义：

- `pending`：MySQL 已登记构建，但 worker 还没领取。
- `building`：正在解析、切块、embedding 和写 Qdrant。
- `ready`：chunk manifest 和 Qdrant points 已验证一致，但还未对外发布。
- `active`：稳定 alias 已指向包含它的快照。
- `failed`：构建失败，可以重试回到 `building`。

为什么禁止 `active -> building`：active 版本是已经发布的事实记录。重建应该创建新版本，不能原地改写线上版本。

## 6. Part 5：MySQL 三张表分别负责什么

Schema 文件：

- [document_ingestion.sql](/offline-rag-go-lab/sql/document_ingestion.sql:1)

三张表：

1. `document_sources`：逻辑文档和当前生效版本指针。
2. `document_versions`：每次不可变构建的输入、状态和失败原因。
3. `document_chunk_manifests`：某一版本预期拥有的全部 chunks。

关键约束：

- `(knowledge_scope, document_id)` 唯一，防止逻辑文档重复。
- 构建输入组合唯一，保证相同构建可以幂等复用。
- `(document_version_id, chunk_id)` 唯一，防止版本内重复 chunk。
- `(document_version_id, ordinal)` 唯一，保证输出顺序确定。
- version 指向 source、chunk 指向 version，并使用 MySQL 外键。

`active_version_id` 没有建立反向外键。否则 source 和 version 会形成循环依赖，普通 `CREATE TABLE IF NOT EXISTS` 无法安全重复应用。后续激活事务必须查询并确认版本属于同一个 source，再更新指针。

## 7. 测试如何证明行为

执行：

```bash
go test ./internal/documentingest
go test -race ./internal/documentingest
```

测试覆盖：

- 文档身份和相对路径校验
- 跨操作系统文本规范化
- 未变化/内容变化/章节移动/重复块身份
- chunk policy 变化产生不同 hash
- 合法和非法状态迁移
- 输入 `[]byte` 不会被返回对象引用并受后续修改影响
- Go 字段上限与 MySQL schema 一致，路径和标题拒绝控制字符

`-race` 会启用 Go 数据竞争检测器。本节没有并发代码，但它建立后续 ingestion worker 继续沿用的验证基线。

## 8. 当前边界与生产级边界

本节已经真实完成：

- 稳定、可测试的文档/chunk 身份
- 版本构建 hash
- 封闭状态机
- 生产方向的三表 schema

本节尚未执行：

- MySQL 建表或写入
- Markdown/Go 结构化切块
- Ollama embedding
- Qdrant upsert、alias 或删除

这些行为会在第 30-32 节逐层接入，而不是在身份规则未固定前混在一个 demo 中。

## 9. 总结与重点

1. 文档、版本、chunk 是三种不同身份。
2. 稳定 `chunk_id` 不包含版本号和全局行号，否则无关插入也会让所有后续 ID 改变。
3. 内容、结构路径或重复位置变化时，chunk ID 应该变化。
4. SHA256 用于确定性身份，不用于语义相似度。
5. MySQL 保存权威版本和 manifest；Qdrant 后续只是可重建索引。
6. active 版本不可原地修改，新内容必须走新版本构建和发布。
7. 构建 hash 还绑定 embedding model；换模型必须创建新 vector build，不能复用旧向量空间。
