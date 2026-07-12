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
