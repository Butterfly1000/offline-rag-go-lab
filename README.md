# offline-rag-go-lab

`offline-rag-go-lab` 是一个独立的 Go 版离线 RAG 骨架项目。

它放在 `/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab`，但不依赖 `chat-api` 目录下的其他代码。你后面可以把整个文件夹直接拎走，作为单独仓库或单独模块继续演进。

## 当前目标

先把最小闭环跑通：

`Open WebUI / 任意客户端 -> Go RAG Gateway -> knowledge retrieval -> answer -> JSONL log`

当前版本默认使用 mock 风格的内存检索和回答生成，重点是先把：

- 项目结构
- 接口契约
- 检索行为
- 问答链路
- 原始日志

这些边界固定下来。

## 相关子项目

如果你要看七牛上传、手机端上传任务、断点续传等独立案例，入口已经整理到：

- [projects/qiniu-uploader/README.md](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/projects/qiniu-uploader/README.md:1)

如果你要看这个 RAG lab 本身的阅读入口，只保留了两份：

- [world/game-world-map.md](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/world/game-world-map.md:1)
- [appendix/current-implementation.md](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/appendix/current-implementation.md:1)

## 目录

```text
offline-rag-go-lab/
├── go.mod
├── README.md
├── world/
│   └── game-world-map.md
├── appendix/
│   └── current-implementation.md
├── cmd/
│   └── rag-gateway/
│       └── main.go
└── internal/
    └── gateway/
        ├── facade.go
        ├── level1_world/
        ├── level2_hq/
        ├── level3_store/
        ├── level4_retrieval/
        ├── level5_chunking/
        ├── level6_compression/
        ├── boss/
        └── shared/
```

## 当前提供的接口

- `GET /healthz`
- `POST /debug/split`
- `GET /debug/retrieval?question=...`
- `GET /debug/prompt?question=...`
- `POST /ingest`
- `POST /chat`

## 运行测试

```bash
cd /Users/huangyanyu/go/src/chat-api/offline-rag-go-lab
GOTOOLCHAIN=local GOWORK=off GOCACHE=/tmp/offline-rag-go-lab-gocache go test ./...
```

## 启动服务

```bash
cd /Users/huangyanyu/go/src/chat-api/offline-rag-go-lab
GOTOOLCHAIN=local GOWORK=off GOCACHE=/tmp/offline-rag-go-lab-gocache go run ./cmd/rag-gateway
```

当前代码监听端口：

```text
http://127.0.0.1:18092
```

## 最快体验

### 1. 导入知识

```bash
curl -X POST http://127.0.0.1:18092/ingest \
  -H 'Content-Type: application/json' \
  -d '{
    "document_id":"refund-policy",
    "title":"退款政策",
    "source_ref":"refund-policy.md",
    "text":"用户在购买后 7 天内可以申请退款。\n处理流程包括提交订单号、说明退款原因、等待人工审核。",
    "tags":["faq","refund"]
  }'
```

### 2. 单独查看切分结果

```bash
curl -X POST http://127.0.0.1:18092/debug/split \
  -H 'Content-Type: application/json' \
  -d '{
    "document_id":"refund-policy",
    "title":"退款政策",
    "source_ref":"refund-policy.md",
    "text":"用户在购买后 7 天内可以申请退款。\n\n处理流程包括提交订单号、说明退款原因、等待人工审核。",
    "tags":["faq","refund"]
  }'
```

这个接口最适合观察：

- 文档被切成几个 chunk
- 每个 chunk 的 `chunk_id`
- 每个 chunk 的边界长什么样

### 3. 查看检索结果

```bash
curl 'http://127.0.0.1:18092/debug/retrieval?question=退款需要什么步骤'
```

这个接口最适合观察：

- query 标准化后是什么
- 命中了哪些 chunk
- 每个 chunk 为什么会得这个分数

### 4. 查看 prompt 拼接结果

```bash
curl 'http://127.0.0.1:18092/debug/prompt?question=退款需要什么步骤'
```

这个接口最适合观察：

- 最终会选哪几个 chunk 进上下文
- 上下文会怎样被拼成 prompt

### 5. 发起问答

```bash
curl -X POST http://127.0.0.1:18092/chat \
  -H 'Content-Type: application/json' \
  -d '{
    "session_id":"session-001",
    "user_id":"local-user",
    "question":"退款需要什么步骤？",
    "model":"mock-chat",
    "use_knowledge":true
  }'
```

## 后续演进建议

你下一步最自然的升级顺序是：

1. 把内存检索替换成真实 `Qdrant`
2. 把 mock answer 替换成真实 `Ollama`
3. 增加更稳定的 chunking
4. 增加 `memory_items`
5. 再做候选知识沉淀

## 现在最值得先看的代码

如果你想先把“内部 RAG 环节”理解透，我建议按这个顺序读：

1. [level5_chunking/chunker.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/level5_chunking/chunker.go:1)
   它负责“文档怎么切分”

2. [level3_store/store_memory.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/level3_store/store_memory.go:1)
   它负责“模拟版 Qdrant 怎么存和怎么搜”

3. [level4_retrieval/retrieval_service.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/level4_retrieval/retrieval_service.go:1)
   它负责“把 query 处理、阈值配置和 store 调用收口成检索服务”

4. [level4_retrieval/retrieval.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/level4_retrieval/retrieval.go:1)
   它负责“query 如何标准化、如何计算相似度”

5. [boss/prompt.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/boss/prompt.go:1)
   它负责“命中的知识怎样被拼成 prompt”

6. [level2_hq/app.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/level2_hq/app.go:1)
   它负责“把这些环节串起来”
