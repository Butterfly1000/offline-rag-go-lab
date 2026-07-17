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

## 第 30 节：Markdown 与 Go 结构化分块

### 影响分析

本节新增 Markdown scanner、Go AST parser、真实 tokenizer adapter、统一 split/pack、
fixtures、CLI、单元测试和 SOP。

运行时只读取：

- `config/recent-chat.env` 中的 tokenizer 路径
- `assets/tokenizers/qwen2/tokenizer.json`
- `internal/documentingest/testdata` 源文件

未连接或修改 MySQL、Qdrant、Ollama，也未访问外部网络。

### RED 证据

命令：

```bash
go test ./internal/documentingest -run 'Test(Markdown|GoSource|ChunkDocument|QwenTokenCounter)'
```

结果：FAIL。编译器报告 `ChunkPolicy`、`ChunkDocument`、`TokenCounter` 等目标 API 不存在。

后续 review 修复也分别保留了 RED：

- 非单调 BPE 前缀使二分实现失败
- 多行在第一次超限后停止，漏掉后续可合并候选
- wrapper 使用加法 overhead 无法得到合规片段
- overlap-only 产生只有重复行的 chunk
- 真实中文 added token 被 tokenizer fork 丢弃

### GREEN 证据

命令：

```bash
go test ./internal/tokenizerdemo ./internal/documentingest
```

结果：PASS。最终 race、vet、build 和全量门禁在本节提交前再次执行。

### 实践行为

命令：

```bash
go run ./cmd/document-chunk-demo --config config/recent-chat.env --format markdown --source internal/documentingest/testdata/course.md --max-tokens 160
go run ./cmd/document-chunk-demo --config config/recent-chat.env --format go --source internal/documentingest/testdata/service.go.txt --max-tokens 160
go run ./cmd/document-chunk-demo --config config/recent-chat.env --format markdown --source internal/documentingest/testdata/course.md --max-tokens 30 --overlap-lines 1
```

真实结果：Markdown 默认策略输出 5 块，小预算输出 11 块且全部 `<=30`；Go 默认策略
输出 5 块，能看到 package/import/type/method/function 路径和 doc comment。

### Tokenizer 根因调试

初次 Markdown demo 出现非空中文段落 `tokens=0`。按数据流定位得到三个根因：

1. `regexp2.Match.Index/Length` 是 rune offset，NormalizedString 要求 byte offset
2. `PreTokenizedString.Normalize` 没有保留已经附带 token 的 split
3. added-token 匹配被全局 ID 排序打乱，不符合 leftmost-longest

对应回归先失败后修复。当前已验证：

```text
我 -> 1 token, id=56023
未 -> 1 token, id=73306
我叫小黄，这个项目是 Go 写的。 -> 15 tokens
版本从 pending ... 允许重试。 -> 41 tokens
```

旧教学输出中的 8-token 数值属于 tokenizer bug，不再作为正确证据。

### Review 发现与修复

除 tokenizer 根因外，review 还修复：

1. 前缀二分错误假设 BPE count 单调，改为完整候选逐个实测
2. 行组合遇到第一次超限就停止，改为扫描全部候选并选最长合规结果
3. fenced wrapper 用 token 加法估算，改为直接计算带 fence 的完整候选
4. overlap 无法携带新行时会生成重复-only chunk，改为跳到未覆盖行
5. 测试预算与真实 overlap 不可同时满足时，修正测试参数而不是放宽上限
6. Go AST declaration 范围不包含独立注释和文件尾注释；新增丢失回归后，把声明间
   非空白源码归入后续声明，并把文件尾非空白源码归入最后一个声明

### 当前边界

本节只完成“源文件 -> chunks”。第 31 节才应用 MySQL schema、调用本地 bge-m3 并
写隔离 Qdrant collection。当前 tokenizer 资产来源与 Ollama 模型同源性仍未证明，
已保留官方 runtime/token IDs 对照 backlog。

## 第 31 节：真实幂等 MySQL、Ollama 与 Qdrant 入库

### 影响分析

本节只连接用户批准的本地服务：MySQL `offline_rag`、Ollama `bge-m3` 和隔离 Qdrant
collection `offline_rag_document_ingestion_lab_v1`。既有三个 collections 未修改，未访问
外部网络，未 push。

### RED/GREEN 证据

初始 RED 因 `Version`、`BuildIdentity`、`VectorPoint`、`IngestionService` API 不存在而
编译失败。后续分别保留两个 review RED：非有限 embedding 被错误写入 fake index；caller
取消后 failed 写回沿用 cancelled context。修复后 focused tests 通过。

### 真实运行证据

第一次运行：version=1、noop=false、chunks=5、embed batches=1、upsert batches=1、
manifest rows=5。第二次同参数运行：version=1、noop=true、embed/upsert batches 均为 0，
manifest 仍为 5。

Qdrant 实测：status=green、points=5、size=1024、Cosine，并建立 scope/document/chunk 三个
keyword indexes。

### 执行问题

第一次真实命令耗时异常长，是 Codex 执行授权等待，不是 Ollama 推理耗时。批准并保存
`go run ./cmd/document-ingest-demo` 项目前缀后，第二次完整命令约 1 秒返回。

### Review 修复

1. 编排层显式拒绝 `NaN/Inf`，不依赖 Qdrant adapter 兜底
2. failed 写回使用脱离 caller cancellation、上限 5 秒的 cleanup context
3. `LastInsertId=0,nil` 返回明确错误，不生成 `%!w(<nil>)`
4. collection 在领域和 CLI 两层限制为隔离 ingestion 前缀/配置值
