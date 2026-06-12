# Game World Map

这是这个项目唯一保留的“世界大纲”。

如果你以后再回来理解 `offline-rag-go-lab`，先看这一份就够了。

## 这个世界是什么

`offline-rag-go-lab` 不是生产版 RAG 服务，而是一个 **Go 版 RAG 训练场**。

它现在已经能做这些事：

- 导入知识文档
- 把文档切成 chunk
- 做最小版检索
- 压缩命中结果
- 组装 prompt
- 生成 mock answer
- 写 JSONL 日志

它现在还没有完全开放这些区域：

- 真实 `Qdrant`
- 真实 `Ollama`
- 真实 embedding
- `memory_items`
- 摘要式 compression

## 主线地图

这个世界的主线就 6 关：

1. 世界入口
   先知道项目现在能做什么、不能做什么

2. 总指挥部
   `App` 是主编排层

3. 知识仓库
   `store_memory.go` 是默认知识库存储实现

4. 检索技能
   `retrieval_service.go` + `retrieval.go`

5. 切块术
   `chunker.go`

6. 压缩术
   `compressor.go`

最后的 Boss 战是：

- `retrieve -> compress -> prompt/generate -> log`

## 每一关对应哪里

### 第一关：世界入口

- [README.md](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/README.md:1)

### 第二关：总指挥部

- [internal/gateway/facade.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/facade.go:1)
- [internal/gateway/level2_hq/ports.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/level2_hq/ports.go:1)
- [internal/gateway/level2_hq/app.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/level2_hq/app.go:1)

### 第三关：知识仓库

- [internal/gateway/level3_store/store_memory.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/level3_store/store_memory.go:1)

### 第四关：检索技能

- [internal/gateway/level4_retrieval/retrieval_service.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/level4_retrieval/retrieval_service.go:1)
- [internal/gateway/level4_retrieval/retrieval.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/level4_retrieval/retrieval.go:1)

### 第五关：切块术

- [internal/gateway/level5_chunking/chunker.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/level5_chunking/chunker.go:1)
- [internal/gateway/level5_chunking/chunker_test.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/level5_chunking/chunker_test.go:1)

### 第六关：压缩术

- [internal/gateway/level6_compression/compressor.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/level6_compression/compressor.go:1)
- [internal/gateway/level6_compression/compressor_test.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/level6_compression/compressor_test.go:1)

### Boss 战：完整链路

- [internal/gateway/level2_hq/app.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/level2_hq/app.go:1)
- [internal/gateway/boss/prompt.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/boss/prompt.go:1)
- [internal/gateway/boss/generator_mock.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/boss/generator_mock.go:1)
- [internal/gateway/boss/logging.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/boss/logging.go:1)
- [internal/gateway/boss/gateway_test.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/boss/gateway_test.go:1)

## 通关奖励

打完整张图后，你应该拿到这几个理解：

1. 这个项目的价值是“骨架清楚”，不是“依赖齐全”
2. `App` 是编排层，不是全能大文件
3. 当前检索和压缩都是教学版，但边界已经拆开
4. 未来如果接真实 `Qdrant/Ollama/memory`，知道该替换哪里

## 未开放区域

现在仍然属于待开发：

- 真实 `Qdrant` store
- 真实 `Ollama` generator
- 更完整的 logging 字段
- `memory_items`
- summary / rerank / long-session compression

## 一句话版

如果把这个项目只留一句话：

**这是一个把 RAG 主链路拆成几个可独立观察环节的 Go 训练场。**
