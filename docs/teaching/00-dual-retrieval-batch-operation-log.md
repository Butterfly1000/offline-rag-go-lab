# Dual Retrieval Batch Operation And Impact Log

主题：第 24-28 节执行过程、外部状态影响和验证证据。

本日志记录实现行为，不表示用户已经学会对应课程。

## 授权边界

- 只修改 /offline-rag-go-lab
- 每节 RED -> GREEN -> 实践/SOP -> review -> 独立 commit
- 不执行 git push
- 只允许新建和写入 offline_rag_document_chunks_v1
- 不修改 ollama_chat_memory
- memory 查询强制 user_id，document 查询强制 knowledge_scope
- MySQL 是 memory 事实源，Qdrant 失败不能反向修改 MySQL

## 第 24 节：统一 Hit 与 Ownership 边界

### 影响分析

本节只新增纯 Go 类型、校验器、错误分类、单元测试、demo 和文档。

未执行：

- MySQL 连接或写入
- Ollama 请求
- Qdrant 请求或 collection 写入
- 外部网络请求

### RED 证据

命令：

    go test ./internal/contextretrieval -run 'Test(ValidateHit|SourceError)'

结果：FAIL。失败原因是 Hit、ValidateHit、InfrastructureFailure 等目标 API 尚不存在。

### GREEN 证据

命令：

    go test ./internal/contextretrieval
    go test -race ./internal/contextretrieval

结果：PASS。

### 实践行为

命令：

    go run ./cmd/context-hit-demo

目标：展示合法 memory、合法 document 和被拒绝的混合 ownership。

### 当前边界

本节只定义结果边界。真实 Qdrant filter、document collection 和双路召回在后续课程实现。

### Review 发现与修复

Review 发现调用方可以直接构造未知 source 的 SourceError；旧版
IsInfrastructureFailure 只检查 kind，可能把这种畸形错误误判为可降级错误。

修复过程：

1. 新增 TestIsInfrastructureFailureRejectsMalformedSourceError
2. 确认测试先失败
3. 分类时同时检查已知 source、infrastructure kind 和非空 cause
4. 重新运行 package 与全量门禁
