# Current Implementation

这是唯一保留的技术附录。

它只回答一个问题：

**当前代码现实落地到了哪一步。**

## 1. 当前真实状态

当前实现是 **Go 版最小闭环**，不是早期文档里设想的 Python Gateway。

现在已经落地的链路是：

`ingest -> chunk -> store -> retrieve -> compress -> prompt/generate -> log`

其中：

- `store` 是内存版
- `retrieve` 是教学版 token 相似度
- `generate` 是 mock answer
- `log` 是 JSONL

## 2. 当前目录骨架

最值得关心的是：

- [cmd/rag-gateway/main.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/cmd/rag-gateway/main.go:1)
- [internal/gateway/](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway)

其中 `internal/gateway/` 的分层大致是：

- `facade.go`
  对外统一入口，保持 `internal/gateway` 使用方式不碎
- `level1_world/types.go`
  配置和输入输出结构
- `level2_hq/ports.go`
  系统边界接口
- `level2_hq/app.go`
  总编排器
- `level3_store/store_memory.go`
  默认知识库存储
- `level4_retrieval/retrieval_service.go`
  检索流程收口
- `level4_retrieval/retrieval.go`
  标准化、切 token、算分
- `level5_chunking/chunker.go`
  文档切块
- `level6_compression/compressor.go`
  命中去重、限量、裁剪
- `boss/prompt.go`
  prompt 组装
- `boss/generator_mock.go`
  默认 mock answer
- `boss/logging.go`
  JSONL 日志
- `shared/util.go`
  关卡之间共用的小工具

## 3. 对外接口

当前服务暴露：

- `GET /healthz`
- `POST /debug/split`
- `GET /debug/retrieval`
- `GET /debug/prompt`
- `POST /ingest`
- `POST /chat`

HTTP 入口在：

- [cmd/rag-gateway/main.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/cmd/rag-gateway/main.go:1)

当前代码里监听端口是：

- `:18092`

## 4. 主链路怎么读

### 导入知识

`POST /ingest`

流程：

1. 收到文档
2. `BuildChunks`
3. `store.Upsert`
4. 原文落盘

### 调试检索

`GET /debug/retrieval`

流程：

1. `Retrieve(question)`
2. 返回标准化后的问题和命中结果

### 调试 prompt

`GET /debug/prompt`

流程：

1. `Retrieve`
2. `Compress`
3. `BuildPrompt`

### 问答

`POST /chat`

流程：

1. 校验输入
2. `Retrieve`
3. `Compress`
4. `Generate`
5. `AppendLog`

## 5. 目前已经显式成层的部分

早期还只是概念，现在已经真正独立出来的层有：

- `Retriever`
- `Compressor`
- `PromptBuilder`
- `AnswerGenerator`
- `ConversationLogger`

这意味着后续如果要接真实组件，不必重写主链路，只要替换默认实现。

## 6. 目前测试覆盖的重点

看这些测试：

- [internal/gateway/boss/gateway_test.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/boss/gateway_test.go:1)
- [internal/gateway/level5_chunking/chunker_test.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/level5_chunking/chunker_test.go:1)
- [internal/gateway/level6_compression/compressor_test.go](/Users/huangyanyu/go/src/chat-api/offline-rag-go-lab/internal/gateway/level6_compression/compressor_test.go:1)

主要验证了：

- ingest 后可以检索
- split preview 可观察
- prompt preview 会带知识
- chat 命中知识时会使用知识
- chat 未命中时会回退
- chat 会写日志
- chunker 会识别标题并切长行
- compressor 会去重、限量、截断

## 7. 现在没做什么

当前还没落地：

- 真实 `Qdrant`
- 真实 `Ollama`
- 真实 embedding
- `memory_items`
- 摘要式 compression

所以如果你看到旧讨论里提到这些，把它们理解成“未来区域”，不要当成现状。
