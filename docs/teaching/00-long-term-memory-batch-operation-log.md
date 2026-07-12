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
