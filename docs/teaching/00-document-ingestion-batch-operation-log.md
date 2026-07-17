# Document Ingestion Batch Operation And Impact Log

主题：第 29-33 节执行过程、外部状态影响和验证证据。

本日志记录实现行为，不表示用户已经学会对应课程。

## 授权边界

- 只修改 `/offline-rag-go-lab`
- 每节执行 RED -> GREEN -> 实践/SOP -> review -> 独立 commit
- 不执行 `git push`
- 可直接使用本地 MySQL `127.0.0.1:3306/offline_rag`
- 可直接使用本地 Qdrant `127.0.0.1:6333`
- 可直接使用本地 Ollama `127.0.0.1:11434`
- 只允许写 `offline_rag_document_ingestion_lab_v1`、`_v2` 和对应 active alias
- 不修改 `offline_rag_document_chunks_v1`、memory collections 或 `ollama_chat_memory`
- 不删除物理 collection
- 不访问云端数据库或远程模型 API

## 计划复核修正

执行前发现 `document_sources.active_version_id` 和
`document_versions.document_source_id` 如果都使用外键，会形成循环依赖。普通幂等
`CREATE TABLE IF NOT EXISTS` 无法可靠地重复增加反向约束。

处理方式：version -> source 和 chunk -> version 继续由 MySQL 外键强制；nullable
`active_version_id` 由后续激活事务查询并验证版本归属。设计与计划已先修正并独立提交。

## 第 29 节：生产文档身份、版本和稳定 Chunk ID

### 影响分析

本节只新增纯 Go 身份/状态代码、SQL schema 文件、单元测试、demo 和教学文档。

未执行：

- MySQL 连接、建表或写入
- Ollama 请求
- Qdrant 请求、collection 或 alias 变更
- 外部网络请求

### RED 证据

命令：

```bash
go test ./internal/documentingest -run 'Test(StableChunkID|NormalizeDocument|ContentHash|ChunkPolicyHash|ValidateTransition|VersionStatus)'
```

结果：FAIL。编译器报告 `NormalizeDocument`、`Document`、`FormatMarkdown`、
`ChunkIdentityInput` 等目标 API 尚不存在。失败发生在实现前，证明测试没有误测旧代码。

### GREEN 证据

命令：

```bash
go test ./internal/documentingest
go test -race ./internal/documentingest
```

结果：PASS。

### 实践行为

命令：

```bash
go run ./cmd/document-identity-demo
```

实际结果：unchanged ID 相同；changed、moved、duplicate ID 均不同；合法状态迁移全部
通过，`active -> building` 被拒绝。

### 当前边界

本节定义权威身份和状态，不应用 schema。第 30 节先完成结构化切块，第 31 节才连接
本地 MySQL、Ollama 和隔离 Qdrant collection。

### Review 发现与修复

Review 发现最初的 Go 校验允许 `source_ref`、`heading_path` 和 `structure_kind` 超过
MySQL 对应字段上限，并允许路径或标题包含控制字符。这样错误会延迟到数据库写入，
而 NUL/换行还可能破坏身份边界或日志格式。

修复过程：

1. 新增超长 source/path/kind 和控制字符测试
2. 确认 5 个用例先真实失败
3. 在领域层增加与 schema 一致的 1024/1024/64 字节上限
4. source/path 在参与身份和持久化前拒绝控制字符
5. 重新运行 package、race、vet、build 和 demo 门禁

验证时还发现 `go build ./cmd/document-identity-demo` 会在仓库根目录生成同名二进制。
该未跟踪生成物已删除；后续 main package 构建验证统一使用显式
`go build -o /tmp/<name> ...`，避免工作树被本地二进制污染。
