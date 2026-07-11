# Five Token Section Operation And Impact Log

主题：2026-07-11 第 08 到第 12 节 token 聊天接入操作记录

设计基线 commit：`4aff6d5`

## 授权边界

本批次允许：

- 读取和修改当前仓库文件
- 运行格式化、单元测试、race、vet、build 和教学命令
- 只读调用本机 Ollama `/api/show`
- 使用现有配置对本机 recent-chat 做真实验证
- 在当前仓库创建本地 commit

本批次禁止：

- `git push`
- 修改数据库 schema、Qdrant 或 Ollama 模型文件
- 写入其他项目
- 破坏性 Git 或文件操作
- 读取、修改、提交或输出凭据

## 第 08 节：Qwen 历史消息格式化

### 执行操作

1. 先新增格式器测试并确认因实现缺失而失败
2. 新增独立 `internal/chatprompt` 消息格式器
3. 新增 `message-format-demo` 和教学 SOP
4. 运行目标测试、全量回归、vet、build 和实践命令
5. 提交前 review，并创建独立 commit

### 状态影响

- 仓库：新增纯 Go 格式组件、测试、命令和文档
- Ollama：没有访问
- MySQL/Qdrant：没有访问
- 现有 `/chat`：没有修改
- Git：只创建本地 commit，不 push

### 风险分析

- role 使用白名单，避免生成模型不认识的消息角色
- 格式规则来自当前 Qwen ChatML 结构，但不冒充 Ollama 内部多轮 prompt 的严格复刻
- 本节没有 tokenizer、网络或数据库依赖，影响范围只在新增组件

### RED 证据

执行：

```bash
go test ./internal/chatprompt
```

失败原因：`QwenFormatter` 和 `Message` 尚不存在，符合测试先行预期。

### GREEN 与 review 证据

- 目标测试：`go test ./internal/chatprompt` 通过
- 回归测试：`go test ./...` 通过
- 并发检查：`go test -race ./internal/chatprompt` 通过
- 静态检查：`go vet ./internal/chatprompt ./cmd/message-format-demo` 通过
- 命令构建：`go build ./cmd/...` 通过
- 合法实践：输出 user role、正文和完整 Qwen 消息边界
- 非法实践：`unknown` role 以退出码 `1` 拒绝
- 敏感信息扫描：本节文件未出现完整本机路径、DSN、密码、secret 或 API key
- Review：未发现 Critical 或 Important 问题

## 第 09 节：完整对话 prompt 计数

### 执行操作

1. 新增完整对话渲染和一次性计数测试
2. 确认测试因 `Render`、`TokenCounter` 和 `TokenUsage` 缺失而失败
3. 实现对话组合和完整 tokenizer 计数
4. 新增 `conversation-token-demo` 和教学 SOP
5. 运行真实 tokenizer 命令、回归检查和提交前 review

### 状态影响

- 仓库：扩展 `chatprompt`，新增计数代码、测试、命令和文档
- tokenizer：只读本地 `tokenizer.json`
- Ollama/MySQL/Qdrant：没有访问
- 现有 `/chat`：没有修改
- Git：只创建本地 commit，不 push

### 风险分析

- tokenizer 对完整 conversation 只调用一次，避免正文分段相加造成的漏算
- formatter 错误会带消息下标，tokenizer 错误会保留操作上下文
- 示例 token 数取决于本地资产，文档不把本机数字描述成通用常量

### RED 证据

执行：

```bash
go test ./internal/chatprompt
```

失败原因：`Render`、`NewTokenCounter` 和相关类型尚不存在，符合测试先行预期。

### GREEN 与 review 证据

- 目标测试：`go test ./internal/chatprompt` 通过
- 回归测试：`go test ./...` 通过
- 并发检查：`go test -race ./internal/chatprompt` 通过
- 静态检查：`go vet ./internal/chatprompt ./cmd/conversation-token-demo` 通过
- 命令构建：`go build ./cmd/...` 通过
- 完整实践：4 条消息的 rendered conversation 为 `122` tokens
- 对照实践：去掉两条历史后，2 条消息为 `62` tokens
- Review：未发现 Critical 或 Important 问题

## 第 10 节：模板感知的 recent window

### 执行操作

1. 新增格式化消息计数、严格超预算和非法角色测试
2. 确认测试因新构造器缺失而失败
3. 扩展 `TokenBudgetWindowBuilder`，同时保留 legacy 和 formatted strict 模式
4. 把真实 `recent-chat` 入口切换到 formatted strict 模式
5. 新增教学 SOP，并运行回归与提交前 review

### 状态影响

- 仓库：修改 recent window 选择器和服务装配，新增测试与 SOP
- `/chat`：手动 `recent_token_budget` 的计数从正文升级为完整 Qwen 消息
- tokenizer：服务启动时仍只读现有本地资产
- Ollama/MySQL/Qdrant：本节自动验证没有访问或修改
- Git：只创建本地 commit，不 push

### 风险分析

- 旧构造器和测试保留，已有 content-only 教学行为可继续对照
- 真实入口改用严格模式，历史消息不会突破 token budget
- 严格模式可能返回空历史，这是容量安全行为，不影响当前用户消息
- user/assistant 成对保留尚未实现，已明确作为后续上下文质量增强

### RED 证据

执行：

```bash
go test ./internal/recentchat -run 'TestFormattedTokenWindow'
```

失败原因：`NewFormattedTokenBudgetWindowBuilder` 不存在，符合测试先行预期。

### 验证环境说明

第一次运行整个 `internal/recentchat` package 时，sandbox 禁止 `httptest` 监听回环端口。使用已授权的本项目 Go 测试命令在非 sandbox 环境重跑后通过；这不是代码测试失败。

### GREEN 与 review 证据

- 目标测试：formatted 和 legacy token window 测试通过
- 回归测试：`go test ./...` 通过
- 并发检查：目标 window 测试使用 `-race` 通过
- 静态检查：`go vet ./internal/recentchat ./cmd/recent-chat` 通过
- 命令构建：`go build ./cmd/...` 通过
- Review 发现：strict 模式收到 `budget = 0` 时会沿用旧逻辑返回全部历史，可能导致自动预算溢出
- 修复证据：先新增 zero-budget 测试并观察到失败，再让 strict 模式返回空窗口；legacy 行为不变
- 最终 Review：未发现其他 Critical 或 Important 问题

## 第 11 节：自动历史预算规划

### 执行操作

1. 新增模型 context、完整固定 prompt、错误传播和超限测试
2. 新增 Ollama `ContextLength` adapter 测试
3. 确认测试分别因 planner 和 adapter 缺失而失败
4. 实现自动规划器、Ollama adapter 和真实教学命令
5. 运行真实 Ollama/tokenizer 实践、回归检查和提交前 review

### 状态影响

- 仓库：新增自动预算代码、测试、命令和 SOP
- Ollama：实践只读调用 `/api/show`，不生成回答、不修改模型
- tokenizer：只读本地资产
- MySQL/Qdrant：没有访问
- 现有 `/chat`：本节尚未接入自动模式
- Git：只创建本地 commit，不 push

### 风险分析

- provider 和 counter 均为接口，纯规划器不依赖 HTTP 或数据库
- 任一步失败直接返回 error，不使用字符估算兜底
- 复用现有 `Plan` 算术，避免两套容量公式漂移
- 当前每次规划读取一次 model metadata，缓存优化留到多模型阶段

### RED 证据

- `go test ./internal/promptbudget` 因 `NewAutomaticPlanner` 缺失而失败
- Ollama adapter 测试因 `ContextLength` 方法缺失而失败

### GREEN 与 review 证据

- 目标测试：`go test ./internal/promptbudget` 通过
- Ollama adapter 测试：`TestHTTPOllamaClientContextLengthUsesShowMetadata` 通过
- 正常实践：context `32768`、fixed `64`、reserve `2048`、history `30656`
- 算术核对：`64 + 2048 + 30656 = 32768`
- 失败实践：fixed + reserve 为 `32832` 时以退出码 `1` 拒绝
- sandbox 说明：首次真实命令被本地网络限制拒绝，使用已授权的只读 Ollama 命令重跑成功
- 回归测试：`go test ./...` 通过
- 并发检查：`chatprompt`、`promptbudget`、`recentchat` 的 `-race` 测试通过
- 静态检查：`go vet ./...` 通过
- 命令构建：`go build ./cmd/...` 通过
- Review：未发现 Critical 或 Important 问题
