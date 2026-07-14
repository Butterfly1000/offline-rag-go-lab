# Long-term Memory Batch Operation And Impact Log

主题：第 19 到第 23 节 Long-term Memory Item 真实落地记录

设计基线：`ae481de`

## 授权边界

- 允许修改当前仓库、运行测试和教学命令、创建本地 commit
- 禁止 push、修改现有 Qdrant collection、访问本项目之外的代码或数据
- MySQL schema、Qdrant collection/point 和其他外部持久化变更前停止说明影响并请求许可
- 低风险只读检查和本机服务调用可以直接执行，但必须在本文记录行为和影响

## 设计前环境确认

### 执行内容

- 启动本机 Docker Desktop
- 启动已有 `qdrant` container
- 只读检查 Qdrant 根接口、collection 列表和 collection 配置
- 只读检查 Ollama 模型能力
- 下载官方 Ollama `bge-m3` 模型
- 用固定中文文本调用 `/api/embed`

### 观察结果

- Docker Server：`24.0.6`
- Qdrant：`v1.18.0`
- 已有 collection：`ollama_chat_memory`
- 已有 collection 配置：`384` 维、Cosine、1 point
- 未读取已有 point payload，未修改已有 collection
- `qwen:7b` capability：`completion`，不能用于 embedding
- `bge-m3` capability：`embedding`
- `bge-m3` 参数：`566.70M`、F16、BERT family
- 固定中文输入返回 1 个 1024 维有限数值向量
- 首次验证请求约 `2481 ms`，`prompt_eval_count=12`

### 外部状态影响

- Docker Desktop 和已有 Qdrant container 已启动
- Ollama 本地新增约 1.2 GB 的 `bge-m3` 模型资产
- 未创建新 Qdrant collection，未写入或删除 point
- 未连接或修改 MySQL
- 未修改项目配置、凭据和 Git 跟踪数据

## 后续记录规则

第 19-23 节分别追加：

1. RED/GREEN 证据
2. 执行的低风险操作
3. 外部状态写入范围和授权
4. Review 发现、修复和回归证据
5. 最终验证命令与结果

只有已经发生的操作才写为完成事实；尚未执行的外部实践保留在实施计划，不提前记录为成功。

## 第 19 节：Memory Item 边界、类型与来源校验

### RED/GREEN

- 先新增 validator 测试，因 `SourceMessage`、`Candidate`、`Operation` 和 `ValidateAndNormalizeCandidate` 不存在而编译 RED
- 新增领域类型和校验器后，`go test ./internal/memoryitem` GREEN
- 设计复核发现只检查 user 仍可能混入同一用户另一 session 的消息，因此入口同时接收并校验 `session_id`

### 状态影响

- 新增纯 Go memory item 类型、候选校验器、测试、demo 和 SOP
- 未访问 MySQL、Ollama 或 Qdrant
- 未修改 `/chat`、已有消息、summary、collection 或 point

### 实践结果

- `Implementation Language` 被规范化为 `implementation_language`
- 合法用户消息 `101` 成为来源证据
- assistant 消息 `102` 作为唯一来源时被明确拒绝
- demo 输出与 SOP 记录一致

### 验证与 Review

- Review 确认 user/session/role/content 四个来源边界都在 Go 侧强制校验
- Review 确认 NaN/Inf、空值、长度、非法 key 和重复来源都有确定行为
- Review 确认文档没有把本节描述成已完成 MySQL 或 Qdrant
- `go test ./...` 通过
- `go test -race ./internal/memoryitem` 通过
- `go vet ./...` 通过
- `go build ./cmd/...` 通过
- `git diff --check` 通过
- 最终 Review：未发现未处理的 Critical 或 Important 问题

## 第 20 节：真实 Ollama 结构化候选提取

### RED/GREEN

- prompt/extractor 测试先因 API 不存在而编译 RED，最小实现后 GREEN
- Ollama adapter 测试先因 `Format` 和 `GenerateJSON` 不存在而编译 RED，接入 `/api/chat` 后 GREEN
- schema compatibility、explicit temperature、confidence 格式和 destructive forget gate 都保留独立 RED/GREEN 证据

### 外部调用与状态影响

- 多次调用本机 Ollama `qwen:7b` 做只读生成验证
- 只读检查 `ollama ps`、`/api/version` 和本机 Ollama server log
- 未访问 MySQL，未创建/修改 Qdrant collection 或 point
- 未修改模型资产、Ollama 配置或已有聊天数据

### 真实故障与定位

- 初始完整嵌套 schema 可重复得到 HTTP 500：`model runner has unexpectedly stopped`
- server log 证明 runner 发生 `SIGSEGV` 并以 status 2 退出
- 普通 chat 成功，单字段 schema 成功，pattern、numeric range 和 source array 单独测试都成功
- 保持完整候选对象结构、减少非必要嵌套约束后 runner 稳定
- 当前兼容 schema 保留结构/required/enum/key pattern/confidence range，其余规则由 Go validator 强制执行

### 真实质量问题与修复

- 第一次兼容 schema 返回合法空 candidates，增加明确提取要求和 `temperature=0`
- 模型先输出中文 key，被 key pattern 和 Go validator 拒绝
- 模型两次输出 `confidence=100`，增加明确 0-1 小数提示；Go validator 始终拒绝越界值
- 模型把“我叫小黄”误判为 forget，新增来源正文 destructive gate 并强化 upsert/forget 示例
- 最终输出 2 条通过校验的 upsert：`identity/name=小黄`、`project_fact/language=Go`
- assistant 的 Rust 推测没有进入候选

### 验证与 Review

- Review 发现 broad phrase `忘记我` 会误匹配“请不要忘记我的名字”并错误放行 forget
- 新增回归测试先 RED，移除 broad substring 后 GREEN；明确遗忘请求仍通过
- Review 确认兼容 schema 省略的长度、范围和来源规则都由 Go validator 测试覆盖
- Review 确认 strict decoder 拒绝尾随 JSON、顶层/候选额外字段和缺失 required 字段
- Review 确认普通 GenerateText 不会自动携带 temperature，只有 GenerateJSON 显式 temperature=0
- `go test ./...` 通过
- `go test -race ./internal/memoryitem ./internal/recentchat` 通过
- `go vet ./...` 通过
- `go build ./cmd/...` 通过
- `git diff --check` 通过
- 最终 Review：未发现未处理的 Critical 或 Important 问题

## 第 21 节：规范化与确定性生命周期决策

### RED/GREEN

- resolver 测试先因 `Resolve`、`ResolveBatch` 和 action 类型不存在而编译 RED，最小实现后 GREEN
- batch review 发现 identity 在规范化前建立，新增混合格式 key 回归测试先 RED，改为排序前统一规范化后 GREEN

### 状态影响

- 新增纯 Go resolver、batch resolver、测试、demo 和 SOP
- 未访问 Ollama、MySQL 或 Qdrant
- 未修改 `/chat`、消息、summary、collection 或 point

### 实践结果

- `implementation_language` 按顺序得到 INSERT v1、NOOP v1、UPDATE v2、FORGET v3、恢复 UPDATE v4
- equivalent value 的空白/大小写差异不会增加 version
- FORGET 保留审计 value，但 status 变为 forgotten；本节没有宣称物理擦除
- batch 按最小来源 ID 稳定处理，并能链式处理同一 identity

### 验证与 Review

- Review 发现 batch 在 identity 规范化前查 state，混合格式 key 可能产生两个 INSERT；回归测试先 RED，修复后 GREEN
- Review 确认 public Resolve 拒绝无有效 ID/version 的 persisted current，batch 只对本批新 INSERT 使用 provisional ID
- Review 确认 resolver 不修改调用方 current/candidate，missing forget 不创建 item
- Review 确认 SOP 区分停止召回和物理数据擦除
- `go test ./...` 通过
- `go test -race ./internal/memoryitem` 通过
- `go vet ./...` 通过
- `go build ./cmd/...` 通过
- `git diff --check` 通过
- 最终 Review：未发现未处理的 Critical 或 Important 问题

## 第 22 节：MySQL Memory Store 与来源证据事务

### RED/GREEN

- 先新增 store/adapter 测试，因 `MemoryStore`、事务接口和 MySQL queries 不存在而编译 RED，实现后 GREEN
- 测试覆盖 INSERT、NOOP、UPDATE、FORGET、恢复、evidence 幂等、rollback、commit 失败、version/duplicate conflict 和 user scope
- review 后新增 demo fixture/终态测试，先因固定专用身份和 `classifyDemoState` 不存在而 RED，实现后 GREEN

### 外部写入与授权

- 用户在执行前通过权限确认批准真实 MySQL 实践
- 幂等创建 `memory_items`、`memory_item_evidence`
- 只向 `recent_chat_messages` 写入 session `memory-store-demo-20260712-a`、user `memory-store-demo-user-20260712-a` 的 6 条 user fixture
- 只为该 user 写入 2 条 memory item 和 6 条 evidence
- 未修改现有聊天、summary、其他用户数据或任何 Qdrant collection/point

### 真实实践结果

- `implementation_language`：INSERT v1 -> NOOP v1 -> UPDATE Rust v2 -> UPDATE Go v3
- `temporary_tool`：INSERT Vim v1 -> FORGET v2
- 最终 active item 为 1，evidence 为 6
- 第二次运行识别完整终态并输出 `no writes applied`，active 仍为 1、evidence 仍为 6

### Review 发现与处理

- P1：demo 原本允许任意 `--user-id/--session-id`，可能污染真实身份；已移除覆盖参数并固定专用身份
- P2：已有 6 条任意消息会被硬映射成 fixture candidate；已逐条校验 role 与正文
- P2：重跑会再次执行 Go/Rust/Go 和 Vim/forget，造成 version 增长；已增加终态识别，完整终态只读返回，部分状态报错停止
- P2：补充 forgotten 恢复、reader/locked item 跨用户负向测试
- 真实并发重试策略和 evidence 复合外键属于生产增强，记录到 optimization backlog，不扩大本节范围

### 验证与 Review

- `go test ./...` 通过
- `go test -race ./internal/memoryitem ./cmd/memory-store-demo` 通过
- `go vet ./...` 通过
- `go build ./cmd/...` 通过
- `git diff --check` 和本次 diff 敏感信息扫描通过
- 最终 Review：此前 P1/P2 findings 已修复，没有遗留的 Critical 或 Important 问题

## 第 23 节：bge-m3 Embedding 与 Qdrant 用户隔离检索

### RED/GREEN

- embedding/Qdrant/demo 测试先因 `Embedder`、`QdrantIndexer`、`SearchResult` 和 demo helpers 不存在而编译 RED
- 实现标准库 HTTP clients 和 demo 后 GREEN
- collection 白名单测试先因 `validateDemoCollection` 不存在而 RED，改为只允许 `offline_rag_memory_items_v1` 后 GREEN

### 测试环境问题

- 沙箱内 `httptest.NewServer` 因回环端口 bind 被拒绝而 panic
- 堆栈证明失败发生在 listener 创建，尚未进入 HTTP client 断言
- 在允许本地回环监听的权限下重跑同一纯本地测试后通过
- 未用真实外部服务替代单元测试，也未降低 request body/filter 断言

### 外部写入与授权

- 用户在执行前通过权限确认批准真实 MySQL/Ollama/Qdrant 实践
- MySQL 只新增第二个固定测试用户的 1 条消息、1 条 active item 和 1 条 evidence
- 创建 `offline_rag_memory_items_v1`：1024 维、Cosine
- 创建 `user_id`、`kind` 两个 keyword payload index
- upsert 两个测试用户各 1 个 active point；对 lesson 22 forgotten item ID 执行幂等 delete
- 代码只允许专用 collection；现有 `ollama_chat_memory` 不读取 payload、不修改

### 真实实践结果

- bge-m3 批量 item embedding 与 query embedding 均为 1024 维
- 第一个用户 top result 为 `project_fact/implementation_language`，score `0.634897`
- 第二个相近文本用户拥有独立 top point，第一个用户结果不含第二用户
- forgotten `temporary_tool` point 不存在
- 重跑时第二 MySQL fixture action 为 NOOP，point ID 不变，检索结果不变
- 新 collection 最终 2 points；旧 collection 仍为 384/Cosine、1 point

### Review 发现与处理

- 本地审查发现仅禁止旧 collection 仍可能误写其他已存在集合；新增 RED 测试并改成专用 collection 白名单
- embedding 数量、维度、有限数，Qdrant payload user/item ID 和返回 kind/key/version 均做 Go 二次校验
- Qdrant 错误不写回 MySQL；outbox/rebuild/drift scan 记录到 backlog

### 验证与 Review

- `go test ./...` 通过
- `go test -race ./internal/memoryitem ./internal/recentchat ./cmd/memory-store-demo ./cmd/memory-qdrant-demo` 通过
- `go vet ./...` 通过
- `go build ./cmd/...` 通过
- `gofmt -d`、staged/unstaged `git diff --check` 和敏感信息扫描通过
- 最终本地 Review：HTTP 请求形状、user/kind filter、active/forgotten、专用 collection、MySQL 事实源和重跑幂等均符合设计，没有遗留 Critical 或 Important 问题
