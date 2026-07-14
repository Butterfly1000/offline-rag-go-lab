# Memory Item Qdrant SOP

主题：第 23 节，用 bge-m3 和 Qdrant 建立用户隔离的长期记忆语义索引

## 1. 这一节解决什么问题

第 22 节已经把长期记忆可靠保存到 MySQL，但按 `kind + key` 精确查询不能解决自然语言检索：

```text
memory item: project_fact/implementation_language = Go
用户问题:    这个项目使用什么编程语言？
```

两段文本没有完全相同的词，却表达相近含义。本节链路是：

```text
MySQL active item
-> 生成稳定 embedding 文本
-> Ollama bge-m3 /api/embed
-> 1024 维向量
-> Qdrant point + payload
-> query vector + user_id filter
-> 相似 memory item
```

核心边界不变：

- MySQL 是唯一事实源
- Qdrant 是可从 MySQL 重建的派生索引
- 只有 active item 可以写入索引
- forgotten item 必须删除 point
- 每次检索必须带 `user_id`

## 2. Part 1：生成模型和 embedding 模型不是一回事

本项目当前使用：

```text
qwen:7b -> completion/chat，负责生成文字和提取候选
bge-m3  -> embedding，负责把文本转换成向量
```

调用 bge-m3：

```http
POST /api/embed
Content-Type: application/json

{
  "model": "bge-m3",
  "input": [
    "project_fact/implementation_language: Go",
    "project_fact/implementation_language: Rust"
  ]
}
```

等价 curl：

```bash
curl http://127.0.0.1:11434/api/embed \
  -H 'Content-Type: application/json' \
  -d '{
    "model":"bge-m3",
    "input":["project_fact/implementation_language: Go"]
  }'
```

响应中的 `embeddings[0]` 是一组浮点数，不是 token IDs，也不是模型回答。本机真实 bge-m3 返回 1024 个数。

维度由 embedding 模型决定，不是所有模型都一样。collection 的 vector size 必须与真实响应一致。

## 3. Part 2：Go 如何调用 /api/embed

代码：

- [embed.go](/offline-rag-go-lab/internal/memoryitem/embed.go:1)

接口：

```go
type Embedder interface {
    // texts 有几条，返回值就必须有几个 vector。
    Embed(ctx context.Context, model string, texts []string) ([][]float32, error)
}
```

关键请求代码：

```go
body, err := json.Marshal(struct {
    Model string   `json:"model"`
    Input []string `json:"input"`
}{
    Model: model,
    Input: texts,
})

// NewRequestWithContext 把超时和取消传给 HTTP 请求。
request, err := http.NewRequestWithContext(
    ctx,
    http.MethodPost,
    baseURL+"/api/embed",
    bytes.NewReader(body),
)

// Header.Set 声明正文使用 JSON，Ollama 才能按 JSON 解码。
request.Header.Set("Content-Type", "application/json")

response, err := client.Do(request)
defer response.Body.Close()
```

这里几个基础函数的作用：

- `json.Marshal`：把 Go struct 编码成 JSON bytes
- `bytes.NewReader`：让 bytes 可以作为 HTTP body 被读取
- `http.NewRequestWithContext`：建立带取消能力的请求
- `defer response.Body.Close()`：函数返回前关闭连接响应体
- `json.NewDecoder(...).Decode`：把 JSON 响应解码回 Go struct

## 4. Part 3：为什么不能拿到数组就直接写 Qdrant

`validateEmbeddingVectors` 会强制检查：

1. 返回 vector 数量等于输入文本数量
2. 每个 vector 非空
3. 同一批 vector 维度完全相同
4. 每个值都不是 NaN 或正负 Inf

简化代码：

```go
for vectorIndex, vector := range vectors {
    if len(vector) == 0 {
        return fmt.Errorf("embedding %d is empty", vectorIndex)
    }
    if len(vector) != dimension {
        return fmt.Errorf("embedding dimension mismatch")
    }
    for _, value := range vector {
        // math.IsNaN / math.IsInf 检查不能参与正常相似度计算的数值。
        if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
            return fmt.Errorf("embedding value is not finite")
        }
    }
}
```

如果批量输入 2 条，却只返回 1 个 vector，不能猜测它对应哪条 item；整个批次应失败。

## 5. Part 4：embedding 文本如何构造

demo 使用稳定格式：

```go
func memoryEmbeddingText(item memoryitem.Item) string {
    return fmt.Sprintf("%s/%s: %s", item.Kind, item.Key, item.Value)
}
```

示例：

```text
project_fact/implementation_language: Go
```

为什么不只编码 `Go`：

- `Go` 可能是编程语言，也可能是英文动词
- 加上 kind/key 能提供事实类型和字段语义
- 格式稳定后，重建索引不会因随机 prompt 变化产生不同文本

这不是唯一最佳格式。生产中应通过检索样例评估是否加入自然语言模板、来源摘要或多语言字段，而不是凭感觉不断加文本。

## 6. Part 5：创建或验证专用 collection

代码：

- [qdrant.go](/offline-rag-go-lab/internal/memoryitem/qdrant.go:1)

本节只允许：

```text
collection = offline_rag_memory_items_v1
size       = 1024
distance   = Cosine
```

创建请求：

```http
PUT /collections/offline_rag_memory_items_v1

{
  "vectors": {
    "size": 1024,
    "distance": "Cosine"
  }
}
```

但代码不是每次直接创建，而是：

```text
GET collection
-> 404: PUT 创建
-> 已存在: 检查 size 和 distance
-> 不匹配: 返回错误，绝不自动删除重建
```

自动删除很危险，因为集合名配置错误时可能清空其他业务数据。

demo 还把 collection 白名单固定为 `offline_rag_memory_items_v1`，所以配置成 `ollama_chat_memory` 或任何其他集合都会在发请求前失败。

## 7. Part 6：为什么需要 payload index

创建两个 keyword index：

```http
PUT /collections/offline_rag_memory_items_v1/index?wait=true

{"field_name":"user_id","field_schema":"keyword"}
```

```http
PUT /collections/offline_rag_memory_items_v1/index?wait=true

{"field_name":"kind","field_schema":"keyword"}
```

`keyword` 表示精确匹配，不做中文分词。它们的职责是过滤，不是计算语义相似度。

真实 collection 状态：

```json
{
  "vectors": {"size": 1024, "distance": "Cosine"},
  "payload_schema": {
    "user_id": {"data_type": "keyword", "points": 2},
    "kind": {"data_type": "keyword", "points": 2}
  }
}
```

## 8. Part 7：point ID 和 payload 放什么

每个 active item 写成一个 point：

```json
{
  "id": 1,
  "vector": ["1024 个 float32"],
  "payload": {
    "user_id": "memory-store-demo-user-20260712-a",
    "memory_item_id": 1,
    "kind": "project_fact",
    "memory_key": "implementation_language",
    "value": "Go",
    "version": 3,
    "embedding_model": "bge-m3"
  }
}
```

point ID 直接使用 MySQL `memory_items.id`，因此同一个 item 更新后 upsert 会覆盖同一个 point，不会产生多个历史副本。

payload 的 `version` 用于判断索引是否落后：

```text
Qdrant payload version < MySQL item version
-> Qdrant 是旧索引
-> 重新从 MySQL 生成 embedding 并 upsert
```

Qdrant payload 不能反向更新 MySQL。

## 9. Part 8：检索请求如何强制用户隔离

Go 构造的 query body：

```json
{
  "query": ["1024 维 query vector"],
  "filter": {
    "must": [
      {"key":"user_id","match":{"value":"memory-store-demo-user-20260712-a"}},
      {"key":"kind","match":{"value":"project_fact"}}
    ]
  },
  "limit": 5,
  "with_payload": true,
  "with_vector": false
}
```

调用：

```http
POST /collections/offline_rag_memory_items_v1/points/query
```

`Search` 的 `userID` 为空时直接报错，没有“不带 filter 搜全部”的降级路径。即使 Qdrant 错误返回其他用户 payload，Go 还会二次检查并拒绝结果。

`with_vector=false` 表示结果不返回 1024 个浮点数，只返回 score 和 payload，减少网络响应体。

## 10. Part 9：Cosine 相似度是什么

Cosine 比较两个向量方向：

```text
cosine(a, b) = dot(a, b) / (norm(a) * norm(b))
```

其中：

- `dot` 是对应维度相乘后求和
- `norm` 是向量长度
- 越接近同一方向，相似度通常越高

本节不在 Go 中手写 1024 维计算，因为 Qdrant collection 已声明 `Cosine`，检索服务负责高效计算和排序。业务层只检查 score 是有限数，并保留 Qdrant 顺序。

真实查询 `这个项目使用什么编程语言？` 的 top score 是：

```text
0.634897
```

score 不是事实正确率，也没有跨模型通用阈值。阈值需要用真实正负样例评估。

## 11. Part 10：forgotten item 怎么处理

MySQL 中 `temporary_tool=Vim` 已是：

```text
status=forgotten, version=2
```

同步命令执行：

```go
// Delete 使用 MySQL item ID 定位 Qdrant point，并等待操作完成。
err := indexer.Delete(ctx, forgotten.ID)
```

HTTP：

```http
POST /collections/offline_rag_memory_items_v1/points/delete?wait=true

{"points":[2]}
```

删除 point 是幂等行为：point 不存在时再次删除仍可成功。MySQL item/evidence 保留用于审计，但检索不到。

## 12. Part 11：本地配置

Git 忽略的本地文件：

```text
/offline-rag-go-lab/config/recent-chat.env
```

新增三个非敏感配置：

```dotenv
OLLAMA_EMBED_MODEL=bge-m3
QDRANT_BASE_URL=http://127.0.0.1:6333
QDRANT_MEMORY_COLLECTION=offline_rag_memory_items_v1
```

同时继续使用已有：

```dotenv
RECENT_CHAT_MYSQL_DSN=本地真实 DSN
OLLAMA_BASE_URL=http://127.0.0.1:11434
```

项目不依赖 shell 环境变量；配置由 `fileconfig.Load` 从文件读取。

## 13. Part 12：真实运行 SOP

前置条件：

```bash
curl http://127.0.0.1:11434/api/tags
curl http://127.0.0.1:6333/collections
```

执行：

```bash
go run ./cmd/memory-qdrant-demo \
  --config config/recent-chat.env \
  --ensure-collection
```

`--ensure-collection` 是显式确认：该命令可能创建专用 collection。没有 flag 时命令在外部写入前停止。

首次真实结果：

```text
Embedding model: bge-m3
Vector dimension: 1024
Collection: offline_rag_memory_items_v1 (Cosine)
Secondary MySQL fixture: action=insert item_id=3
Upserted active items: 2
Search filter user_id=memory-store-demo-user-20260712-a
Top result: project_fact/implementation_language score=0.634897
Cross-user point present: false
Secondary user top item: 3
Forgotten item present: false
```

为什么需要第二个用户：两条文本语义几乎相同，如果不带 filter，两者都可能被召回。真实结果证明第一个用户的查询没有第二个用户 point。

第二次运行：

```text
Secondary MySQL fixture: action=noop item_id=3
Upserted active items: 2
Cross-user point present: false
Forgotten item present: false
```

说明：

- 第二用户 item 没有增加 version
- 相同 MySQL ID 只覆盖相同 Qdrant point
- 已有 collection 通过配置核对，没有重建
- forgotten delete 再次执行仍安全

## 14. Part 13：旧集合未修改的证据

只读检查：

```bash
curl http://127.0.0.1:6333/collections/ollama_chat_memory
curl http://127.0.0.1:6333/collections/offline_rag_memory_items_v1
```

结果：

```text
ollama_chat_memory:
  size=384, distance=Cosine, points_count=1

offline_rag_memory_items_v1:
  size=1024, distance=Cosine, points_count=2
  payload indexes=user_id,kind
```

旧集合在本批次开始前就是 384 维、1 point，执行后仍相同；没有读取旧 point payload。

## 15. 本次遇到的坑与解决

### httptest 在受限沙箱不能绑定端口

初次运行测试出现：

```text
httptest: failed to listen on a port
listen tcp6 [::1]:0: bind: operation not permitted
```

定位依据：panic 发生在 `httptest.NewServer`，尚未进入 client 请求断言；这是沙箱禁止监听回环随机端口，不是 HTTP client 失败。

解决：在允许本地回环监听的测试权限下运行同一命令。没有因此改成弱 mock，也没有访问真实 Qdrant：

```bash
go test ./internal/memoryitem -run TestHTTPOllamaEmbedder
go test ./internal/memoryitem -run TestQdrantIndexer
```

### 本地配置最初没有 Qdrant 三个键

解决：只在 Git 忽略的 `config/recent-chat.env` 增加 model、base URL 和专用 collection；同步更新可提交的 `.env.example`，不把 DSN 或密码写进 Git。

### 已有 collection 维度不同

旧集合是 384 维，而 bge-m3 真实响应是 1024 维。不能把 1024 维 point 写入 384 维集合，也不能删除旧集合重建。

解决：使用独立 `offline_rag_memory_items_v1`，并在每次运行前核对 size/distance。

## 16. 测试

运行：

```bash
go test ./internal/memoryitem ./cmd/memory-qdrant-demo
```

测试覆盖：

- `/api/embed` 路径、批量 input、非 2xx
- 空 vector、数量不匹配、维度不一致、NaN/Inf
- context cancellation
- collection URL escape、1024/Cosine 创建和配置不匹配
- `user_id`/`kind` keyword index
- point ID、payload version/model
- search 强制 user filter、可选 kind filter
- delete `wait=true`
- 跨用户响应二次拒绝
- 错误 body 截断
- demo 只允许专用 collection

## 17. 当前实现和生产级差异

当前是显式同步命令：

```text
MySQL 已提交
-> 手动运行同步
-> Qdrant upsert/delete
```

真实线上通常还需要：

- MySQL outbox 与异步 worker
- 失败重试和 dead-letter 处理
- 按 item version 幂等同步
- 全量 rebuild 命令
- 定时扫描 MySQL/Qdrant 漂移
- 跨 key 语义去重评估
- 真实检索样例集、Recall@K 和阈值评估

这些增强已进入 optimization backlog。当前没有把 Qdrant 失败伪装成 MySQL 失败，也没有让 Qdrant 反向覆盖事实源。

## 18. 总结与重点

1. qwen 生成文本，bge-m3 生成向量，两种模型职责不同。
2. 维度必须来自真实 embedding 响应；本机 bge-m3 是 1024，不是所有模型都相同。
3. MySQL 保存事实，Qdrant 只保存 active item 的可重建索引。
4. collection 已存在时核对 size/Cosine，不匹配就报错，绝不自动删除。
5. point ID 使用 MySQL item ID，payload 带 user/kind/key/value/version/model。
6. 每次 search 强制 `user_id` filter，Go 还二次检查返回 payload。
7. forgotten item 保留 MySQL 审计状态，但删除 Qdrant point。
8. 第 19-23 节已经形成“提取 -> 校验 -> 决策 -> MySQL -> embedding -> Qdrant 检索”的独立闭环。
9. 下一章是把 memory retrieval 与 knowledge document retrieval 合并后，再按 token budget 注入 `/chat`。
