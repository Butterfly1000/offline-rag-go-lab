# Qdrant 基础课：先看清数据，再看项目代码

本目录是 `examples/01-qdrant-ingest` 的前置课。目标不是立刻做 RAG，
而是先认识 Qdrant 原生的数据结构和最小 HTTP 操作。

## 0. 这节课的结论

```text
collection：一个有名字的、规定了向量规则的检索空间
point：collection 内的一条记录
point = id + vector + payload
```

在 Qdrant 中，真正被检索和管理的基本单位是 point，而不是单独的文本或
单独的 vector。

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "vector": [0.12, -0.08, 0.44],
  "payload": {
    "title": "Payload introduction",
    "text": "Qdrant 的 payload 保存原文和元数据。",
    "knowledge_scope": "qdrant-lesson"
  }
}
```

这段 JSON 是本课最重要的模型：

- `id`：point 在同一个 collection 内的唯一标识。可以是无符号 64 位整数或 UUID。
- `vector`：数字数组，供 Qdrant 计算“语义是否相近”。
- `payload`：可读的 JSON 元数据，供过滤、展示、追溯和取回原文。

## 1. collection 有哪些“固定字段”？

collection 不像 MySQL 表那样先定义 `title`、`text` 等列。它最关键的固定
配置是**向量配置**，例如：

```json
{
  "vectors": {
    "size": 1024,
    "distance": "Cosine"
  }
}
```

- `size`：向量长度。本项目的 `bge-m3` 产生 1024 个浮点数，所以使用 `1024`。
- `distance`：比较向量的规则。本项目使用 `Cosine`（余弦相似度）。
- `vectors`：这是 collection 级别的配置，不是某条 point 自己的数据。

因此，向量长度为 3 的教学数据不能写进 1024 维的 collection；先创建
collection，再写入 point，是因为 Qdrant 必须先知道它将接收怎样的向量。

payload 则是灵活 JSON：不同 point 可以拥有不同的 payload 字段。比如一条
有 `author`，另一条有 `created_at`，并不违反 collection 的规则。

## 2. 当前项目写入的真实 point

运行 `examples/01-qdrant-ingest` 后，Qdrant UI 的 Points 页面会显示一条
point。2026-10-03 本机读取到的真实结构如下；为了可读性，1024 个 vector
数字只保留前三个。

```json
{
  "id": "20aa4e70-136c-48b2-85fc-bbacee91335a",
  "payload": {
    "knowledge_scope": "qdrant-lesson",
    "document_id": "payload-introduction",
    "chunk_id": "payload-introduction-001",
    "title": "Payload introduction",
    "source_ref": "manual-demo",
    "text": "Qdrant 的 payload 保存原文和元数据，vector 用来做语义检索。",
    "content_hash": "abe01e259c433c0786093934638125b26bdbf4025efb76f08b69df6779884d83",
    "embedding_model": "bge-m3"
  },
  "vector": [-0.027713802, -0.03488869, -0.036709417, "... 共 1024 个数字"]
}
```

字段的含义：

| 字段 | 作用 | 是否参与语义计算 |
| --- | --- | --- |
| `id` | 唯一找到或覆盖这一条 point | 否 |
| `vector` | 与用户问题的向量比较相似度 | 是 |
| `text` | 真正要取回并交给大模型的原文 | 否 |
| `knowledge_scope` | 限制只能搜索某个知识范围 | 否，但可作为 filter |
| `document_id` / `chunk_id` | 识别文档和其中的文本块 | 否 |
| `title` / `source_ref` | 给人展示和追溯来源 | 否 |
| `content_hash` | 验证 payload 的文本没有被意外替换 | 否 |
| `embedding_model` | 记录这个 vector 来自哪个模型 | 否 |

注意：payload 不会自动进入 vector。只有把某段内容传给 embedding 模型后，
模型返回的数字数组才是 vector。

## 3. UI 和原生 JSON，各自看什么？

Qdrant Dashboard 的 Points 页面是很好的**观察视图**：

- 可以看到 point ID、payload 字段和 vector 长度；
- 可以复制、删除 point；
- 后续可以使用 Find Similar 检查相似检索结果。

但它不是第一次学习数据结构的最佳主视图：1024 个浮点数通常不会完全展开，
而且 UI 将“collection 配置”和“point 数据”分散在不同标签页。

第一次学习时，以 JSON 为主；UI 用来确认“JSON 确实已经进入了数据库”。

读取当前真实 point 的命令如下。URL 必须用单引号包住，否则 shell 会把 `&`
当成命令分隔符。

```bash
curl -sS 'http://127.0.0.1:6333/collections/offline_rag_lesson_ingest_v1/points/20aa4e70-136c-48b2-85fc-bbacee91335a?with_payload=true&with_vector=true'
```

## 4. 最小的“玩具 collection”

真实的 `bge-m3` vector 有 1024 个数字，不适合手写。为了学习 Qdrant API，
下一步将创建一个独立的 3 维 collection。这个 collection 只能用于本课，
绝不能与项目的 bge-m3 数据混用。

### 4.1 创建 collection

```bash
curl -X PUT 'http://127.0.0.1:6333/collections/qdrant_api_basics_v1' \
  -H 'Content-Type: application/json' \
  --data-raw '{
    "vectors": {
      "size": 3,
      "distance": "Cosine"
    }
  }'
```

### 4.2 写入一条 point

```bash
curl -X PUT 'http://127.0.0.1:6333/collections/qdrant_api_basics_v1/points?wait=true' \
  -H 'Content-Type: application/json' \
  --data-raw '{
    "points": [
      {
        "id": 1,
        "vector": [0.12, -0.08, 0.44],
        "payload": {
          "title": "第一条教学数据",
          "text": "这是一条只有三维向量的 Qdrant 教学数据。",
          "knowledge_scope": "qdrant-basics"
        }
      }
    ]
  }'
```

这里的 `PUT .../points` 是 upsert：ID `1` 不存在时插入，已经存在时覆盖它。

### 4.3 按 ID 读取 point

```bash
curl -sS 'http://127.0.0.1:6333/collections/qdrant_api_basics_v1/points/1?with_payload=true&with_vector=true'
```

## 5. CRUD 词汇表

| 名称 | Qdrant 中的基本做法 |
| --- | --- |
| Create | 创建 collection；upsert 新 ID 的 point |
| Read | 按 ID 获取；scroll 列表读取；vector query 语义查询 |
| Update | 对同一个 ID 再次 upsert；或单独更新 vector / payload |
| Delete | 按 ID 删除；或按 payload filter 批量删除 |

Qdrant 不以 SQL 作为主要接口。项目可以通过 HTTP、gRPC 或 SDK 操作它；本项目
的 Go 封装底层使用 HTTP + JSON。

## 6. 接下来学习顺序

1. 执行上面的 3 维 toy collection 命令，亲眼确认 JSON 如何变成 UI 里的 point。
2. 学习 query：查询 vector、filter、`limit` 和返回的 `score`。
3. 学习 payload index：为什么 `knowledge_scope` 需要 `keyword` 索引。
4. 再回到 `examples/01-qdrant-ingest`，逐行对应真实 HTTP 请求。
