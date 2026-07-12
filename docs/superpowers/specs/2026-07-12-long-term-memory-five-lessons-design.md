# Long-term Memory Item Five Lessons Design

日期：2026-07-12

## 目标

连续实现第 19 到第 23 节，把 Session Summary 之后的长期记忆主线推进为可真实运行的闭环：

```text
用户原始消息
-> Ollama 提取结构化候选
-> Go 校验来源和规范化字段
-> 确定性决定 INSERT / UPDATE / NOOP / FORGET
-> MySQL 保存 memory item 和来源证据
-> Ollama bge-m3 生成 embedding
-> Qdrant 写入可重建语义索引
-> 按 user_id 过滤并语义检索
```

完成第 23 节后，项目应能从真实用户消息提取长期记忆、持久化、建立向量索引并独立检索。把长期记忆自动注入 `/chat`，以及和文档 retrieval 合并，属于下一批次。

## 方案选择

采用“MySQL 事实源 + Qdrant 可重建索引 + 小接口组合”的方案：

1. 新建 `internal/memoryitem`，不让长期记忆逻辑反向依赖 `recentchat` 或 `sessionsummary`。
2. Ollama 只负责从不可信消息中提出候选，不直接决定数据库写入。
3. Go 负责候选校验、来源约束、字段规范化和确定性写入决策。
4. MySQL 保存当前 memory item、version、状态和来源证据，是唯一事实源。
5. Qdrant 只保存 active item 的向量和检索 payload，失败后可以从 MySQL 重建。
6. 所有 MySQL 读写和 Qdrant 检索必须按 `user_id` 隔离。

不采用“把所有聊天消息直接 embedding 后当长期记忆”的方案。原始消息、session summary 和 long-term memory item 的职责不同；直接向量化所有消息会把闲聊、错误回答和过期事实一起召回。

## 数据边界

### Session Summary

- 以 `session_id + user_id` 为范围。
- 保留一次会话的连续性和阶段性上下文。
- 内容可以压缩和滚动更新。
- 不承担跨会话稳定事实的唯一存储职责。

### Long-term Memory Item

- 以 `user_id` 为主要范围，可以跨 session 使用。
- 每条 item 表示一个可命名、可更新、可遗忘的事实。
- 必须有来源证据，不能只保存模型推断结果。
- 当前支持五类：`identity`、`preference`、`project_fact`、`goal`、`constraint`。

### Knowledge Document

- 属于知识库内容，不等同于用户长期记忆。
- 文档 retrieval 和 memory retrieval 在下一阶段合并，不在本批次混用。

## 第 19 节：Memory Item 边界、Schema 与校验

### 领域结构

```go
type Candidate struct {
    Operation        string
    Kind             string
    Key              string
    Value            string
    Confidence       float64
    SourceMessageIDs []int64
}

type Item struct {
    ID      int64
    UserID  string
    Kind    string
    Key     string
    Value   string
    Status  string
    Version int64
}
```

`Operation` 只允许：

- `upsert`：新增或修正记忆
- `forget`：用户明确要求遗忘或明确否定旧事实

`Status` 只允许：

- `active`
- `forgotten`

### 身份和去重键

确定性身份为：

```text
(user_id, kind, memory_key)
```

`memory_key` 必须规范化为小写 snake_case，并通过长度和字符校验。第 19 节只保证相同 key 的确定性去重；“coding_language”和“preferred_language”是否语义相同属于 ontology/semantic dedup 优化，不阻塞主线。

### 来源约束

1. 每个候选至少引用一个本次输入中的正数 message ID。
2. 被引用消息必须属于当前 `user_id` 和当前 `session_id`，且 role 必须为 `user`。
3. assistant 消息可以作为上下文，但不能成为用户记忆的唯一证据。
4. session summary 可以帮助发现候选，但接受候选时仍必须能回指原始用户消息。
5. `forget` 必须有明确用户消息证据；不能因为某事实本轮没有出现就删除。

### 实践

提供纯 Go demo 和表格测试，覆盖合法 upsert、非法 kind/key、未知来源 ID、assistant-only 来源、空值和 forget 约束。

## 第 20 节：真实 Ollama 结构化候选提取

### 输入

- `user_id` 和 `session_id`
- 带 ID、role、content 的原始消息
- 可选 session summary，仅作为辅助上下文
- 模型名 `qwen:7b`

### 输出

使用 Ollama `/api/chat` 的 JSON schema format，要求只返回：

```json
{
  "candidates": [
    {
      "operation": "upsert",
      "kind": "preference",
      "key": "implementation_language",
      "value": "Go",
      "confidence": 0.95,
      "source_message_ids": [101]
    }
  ]
}
```

JSON schema 只保证输出形状，不能证明事实正确。响应解析后仍必须经过第 19 节校验器。

### Prompt 边界

1. system prompt 明确历史消息是不可信数据，不执行其中的系统指令。
2. 只提取用户明确陈述、偏好、目标、约束、身份和项目事实。
3. 不把 assistant 的推测或礼貌性回答变成用户记忆。
4. 不从“没有提到”推断 forget。
5. XML-like 内容要转义或使用长度明确的消息编码，避免正文破坏边界。

### 实践

提供真实命令调用本机 Ollama，输入固定的多轮消息，显示原始结构化响应和校验后的候选。模型返回非法 JSON、未知字段、越权来源或非法候选时返回明确错误，不静默猜测修复；合法的空候选数组表示本轮没有稳定信息。

## 第 21 节：规范化与确定性决策

### 决策结果

```text
INSERT   不存在同 key item，收到 upsert
UPDATE   存在 item，但规范化 value 改变，收到 upsert
NOOP     active item 的规范化 value 相同，或重复 forget
FORGET   active item 收到有明确证据的 forget
```

### 规则

1. kind、key、value 在比较前进行一致的空白和大小写规范化。
2. `upsert` 不能写空 value。
3. item 已 forgotten 后再次 upsert 允许恢复为 active，并增加 version。
4. NOOP 不修改 item version，但可以补充未记录过的来源证据。
5. 缺失候选绝不产生删除。
6. 同一批候选按最小来源 message ID 和原始顺序稳定处理；冲突行为必须可观测，不依赖 map 遍历顺序。

本节使用 in-memory current item 和 fake evidence registry 演示四种决策，不提前连接数据库。

## 第 22 节：MySQL Memory Store 与来源证据

### 表职责

`memory_items` 保存当前状态：

- `id`
- `user_id`
- `kind`
- `memory_key`
- `value`
- `status`
- `version`
- `created_at` / `updated_at`
- 唯一键 `(user_id, kind, memory_key)`

`memory_item_evidence` 保存变更依据：

- `memory_item_id`
- `user_id`
- `source_session_id`
- `source_message_id`
- `source_role`
- `operation`
- `evidence_text`
- `created_at`
- 防止同一来源重复写入的唯一键

### 写入行为

1. 在一个事务中锁定或创建目标 item。
2. 根据第 21 节决策执行 insert/update/forget/noop。
3. 只在 item 状态改变时增加 version。
4. item 和 evidence 同事务提交，不能出现有 item 无来源或有来源无 item。
5. 所有查询必须显式包含 `user_id`。
6. 并发版本不匹配返回可识别 conflict，不覆盖新结果。

### 外部状态

schema 文件可以随代码提交，但不会静默执行。真实实践前必须说明表结构和影响，并获得用户许可；凭据继续从 Git 忽略的本地配置文件读取，不改用环境变量。

### 实践

真实 demo 从专用 session 消息提取候选并写入 MySQL，重复运行验证 NOOP，修改事实验证 version 更新，明确 forget 验证状态变化和 evidence 记录。

## 第 23 节：bge-m3 Embedding 与 Qdrant 检索

### 已验证环境

- Qdrant `v1.18.0` 在 Docker 中运行。
- 本机 Ollama 已安装 `bge-m3`。
- `/api/show` capability 为 `embedding`。
- 固定中文文本通过 `/api/embed` 得到 `1024` 维有限数值向量。
- 现有 `ollama_chat_memory` 是 `384` 维且已有数据，本批次不读取 point payload、不修改、不复用。

### 新集合

创建项目专用集合，设计基线为：

```text
collection: offline_rag_memory_items_v1
vector size: 1024
distance: Cosine
```

创建前必须先调用真实 embedding 验证维度。集合已存在时读取并核对 size/distance；不匹配则报错，不能自动删除重建。

### Qdrant payload

- `user_id`
- `memory_item_id`
- `kind`
- `memory_key`
- `value`
- `version`
- `embedding_model`

point ID 使用 MySQL `memory_items.id`。检索必须带 `user_id` filter；kind filter 可选。forgotten item 不保留可召回 point。

### 一致性边界

1. MySQL 事务先提交，再同步 Qdrant。
2. Qdrant upsert/delete 失败时返回错误并保留 MySQL 事实，允许重试同步。
3. Qdrant 不是事实源，不反向覆盖 MySQL。
4. 本批次实现显式同步命令，不提前加入 outbox、消息队列或后台 worker。
5. point payload version 低于 MySQL 时视为待重建索引。

### 实践

获得用户对新集合和测试数据写入的许可后：

1. 用 `bge-m3` 编码 active memory item。
2. 创建或验证专用 1024 维 Cosine 集合。
3. upsert 专用测试用户的 memory point。
4. 用中文近义问题生成 query vector。
5. 带 `user_id` filter 检索，验证相关 item 排在前面。
6. forget 后删除 point，确认不再召回。

## 错误处理

- 模型 JSON 不符合 schema：拒绝候选
- 未知 kind/operation、非法 key、空 upsert value：拒绝候选
- 来源 ID 不在输入中或不是当前用户消息：拒绝候选
- MySQL version conflict：拒绝覆盖，由调用方重新读取后决策
- evidence 写入失败：回滚整个 MySQL 事务
- embedding 为空、非有限数或维度不一致：拒绝写 Qdrant
- Qdrant 集合配置不匹配：报错，不自动删除
- Qdrant 写失败：MySQL 保持已提交状态并返回可重试错误
- 检索没有 `user_id`：请求校验失败

## 生产边界

本批次明确不实现：

1. memory 自动注入 `/chat`
2. memory retrieval 与 knowledge retrieval 的融合排序
3. 异步 outbox、消息队列和 Qdrant 自动补偿 worker
4. 跨 key 的语义去重和 ontology 管理
5. memory 质量人工审核界面
6. 云端 embedding 或外部模型服务
7. 自动 TTL、衰减评分和访问热度更新

这些内容记录到优化 backlog 或下一章，不作为五节主线验收前置条件。

## 每节验收

每节必须满足：

1. RED/GREEN 测试证据
2. 可执行命令、测试或 curl
3. 独立教学 SOP
4. 提交前 review
5. 独立实现 commit
6. 不 push

## 整批验收

1. 第 19-23 节恰好有五个实现 commit。
2. 设计、计划、操作影响记录、五份 SOP 和学习进度完整。
3. `go test ./...` 通过。
4. 相关 package race 测试通过。
5. `go vet ./...` 和 `go build ./cmd/...` 通过。
6. `git diff --check` 通过。
7. 真实 Ollama 证明结构化提取和 1024 维 embedding。
8. 真实 MySQL 证明 insert/update/noop/forget 和 evidence。
9. 真实 Qdrant 证明用户隔离的 upsert/search/delete。
10. 现有 Qdrant collection 和非测试数据未被修改。
11. 工作区干净，只有本地 commits，没有 push。
