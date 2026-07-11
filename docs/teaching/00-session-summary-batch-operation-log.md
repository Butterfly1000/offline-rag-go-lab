# Session Summary Batch Operation And Impact Log

主题：第 14 到第 18 节 Session Summary 真实落地记录

设计/计划基线：`da5cc54`

## 授权边界

- 允许修改当前仓库、运行测试和教学命令、创建本地 commit
- 禁止 push、Qdrant/long-term memory 实现和破坏性操作
- 数据库 schema、凭据和外部状态变更前停止询问

## 第 14 节：驱逐前缀选择与 token 统计

### RED

- selector 测试因 `SourceMessage`、`SelectPrefix` 不存在而编译失败
- formatted counter 测试因 `NewFormattedMessageCounter` 不存在而编译失败

### 状态影响

- 新增纯 Go selector/counter、测试、命令和 SOP
- tokenizer 实践只读本地资产
- 未访问 MySQL、Ollama、Qdrant
- 未修改 `/chat`

### 验证与 Review

- 标准实践：未摘要 IDs `21..26`，驱逐 `21..24`，token `129/86`，水位 `24`
- ID 空洞实践：驱逐 `21,23`，token `43`，水位 `23`
- Review 发现 raw tokenizer 为 nil 时会 panic
- 修复证据：新增测试先复现 panic，再改为明确 `text token counter is required` 错误
- `go test ./...` 通过
- `go test -race ./internal/sessionsummary` 通过
- `go vet ./...` 通过
- `go build ./cmd/...` 通过
- `git diff --check` 通过
- 最终 Review：未发现其他 Critical 或 Important 问题

## 第 15 节：真实 Ollama 增量摘要生成

### RED/GREEN

- prompt/generator 测试因 API 缺失而 RED，最小实现后 GREEN
- Ollama adapter 测试因 `GenerateText` 缺失而 RED，接入 `/api/chat` 后 GREEN

### 状态影响

- 新增 prompt、generator、测试、真实命令和 SOP
- 实践只调用本机 Ollama，不访问 MySQL/Qdrant，不修改 `/chat`

### 验证与 Review

- 真实 qwen 输出覆盖旧摘要和 IDs `21..23` 的关键事实
- Review 发现模型添加 `<updated_summary>` wrapper
- 修复证据：新增测试先失败，再实现 wrapper 清理并强化 prompt
- 模型仍可能输出自然语言引导语；不使用脆弱关键词删除，记录为结构化输出优化
- `go test ./...` 通过
- `go test -race ./internal/sessionsummary ./internal/recentchat` 通过
- `go vet ./...` 通过
- `go build ./cmd/...` 通过
- `git diff --check` 通过
- 最终 Review：未发现其他 Critical 或 Important 问题

## 第 16 节：MySQL Summary Store 与 Version 更新

### RED/GREEN

- store/MySQL adapter 测试因 `NewStore`、`RowScanner` 和查询实现缺失而 RED，最小实现后 GREEN
- 配置读取测试因 `readConfigValue` 缺失而 RED，实现从本地配置文件读取 DSN 后 GREEN

### 状态影响

- 新增业务 store、MySQL adapter、测试、真实命令和 SOP
- 单元测试使用 fake，不依赖真实 MySQL；获批后另行完成了真实 schema 与读写验证
- demo 读取被忽略的 `config/recent-chat.env`，不把 DSN 放入环境变量、命令行或 Git

### 真实实践

- 获得明确授权后，demo 读取本地配置并幂等执行 `sql/session_summaries.sql`
- 第一次：记录不存在，`expected=0`，保存 `version=1, watermark=20`
- 第二次：记录存在，`expected=1`，保存 `version=2, watermark=24`
- 仅写入专用 `summary-store-demo/summary-store-user`，未修改现有聊天 session

### 验证与 Review

- Review 确认业务层预检查后，SQL 仍重复执行 version/watermark 原子保护
- Review 确认重复插入、并发零影响行、watermark 回退和数据库错误都有测试
- 辅助审查因工作区额度耗尽未返回结论；主流程按相同清单完成逐项 Review
- `go test ./...` 通过
- `go test -race ./internal/sessionsummary ./cmd/summary-store-demo` 通过
- `go vet ./...` 通过
- `go build ./cmd/...` 通过
- `git diff --check` 通过
- 最终 Review：未发现 Critical 或 Important 问题

## 第 17 节：滚动摘要更新服务

### RED/GREEN

- updater 测试因 `UpdateService` 和相关接口缺失而 RED，编排最小实现后 GREEN
- MySQL source 测试因 `MessageRows/newMySQLMessageSource` 缺失而 RED，实现升序水位查询后 GREEN
- file config 测试因 `Load/Required` 缺失而 RED，共享配置读取实现后 GREEN
- command recent 起点测试因 `chooseRecentStartID` 缺失而 RED，实现后 GREEN

### 状态影响

- 新增 updater、MySQL message source、共享 file config、真实命令和 SOP
- 修复 recent 起点早于 watermark 时 selector 误报不存在的问题
- 重构 `summary-store-demo` 使用共享配置读取，行为不变

### 真实实践

- 获得明确授权后，向专用 `summary-update-demo/summary-update-user` 写入 6 条消息
- 实际 IDs `7..12`，evicted `7..10`，recent `11..12`
- qwen 真实生成摘要，保存 `version=1, watermark=10`
- 第二次不 seed：`no_evicted_messages`、`updated=false`，version/watermark 保持 `1/10`
- 模型引导语继续归入已存在的结构化输出优化项，不添加脆弱删除规则

### 验证与 Review

- Review 确认模型失败不调用 Save，version conflict 保持可识别错误链
- Review 确认消息查询同时过滤 session/user 并按 watermark 的 ID 单位升序
- Review 确认配置未进入环境变量，真实 DSN 和本机绝对路径未进入 diff
- `go test ./...` 通过
- `go test -race ./internal/sessionsummary ./internal/fileconfig ./cmd/summary-update-demo ./cmd/summary-store-demo` 通过
- `go vet ./...` 通过
- `go build ./cmd/...` 通过
- `git diff --check` 通过
- 最终 Review：未发现 Critical 或 Important 问题

## 第 18 节：Session Summary 接入自动预算 `/chat`

### RED/GREEN

- summary service 测试因请求字段、构造器、依赖接口和 summary block 缺失而 RED，接入后 GREEN
- file config 整数测试因 `IntOrDefault` 缺失而 RED，实现后 GREEN
- prompt injection guard 测试因 system prompt 未声明历史为不可信数据而 RED，强化规则后 GREEN

### 状态影响

- `/chat` 新增显式 `use_session_summary` 和五个 response 观测字段
- summary 模式仅允许 automatic budget，并加入固定 input reserve、实际 summary 计数和最终二次规划
- `recent-chat` 启动改为直接读 file config，并装配 MySQL source/store、tokenizer、Ollama updater
- 旧请求不开 summary 时不要求新依赖，现有测试保持通过

### 真实实践

- 使用独立服务端口 `18094` 和专用 session，不修改已有聊天 session
- 两次超预算请求分别被 `fixed+output` 和 `summary reserve` 校验在写入前拒绝
- 第一个 session 技术闭环成功，但历史“只回复已记录”导致数据库摘要质量错误
- Review 将其定位为历史 prompt injection，新增 RED 测试并强化 summary system prompt
- 全新 `summary-chat-20260712-b` 两请求复验成功
- 第一次：`fixed=2429, output=29800, available=539, used=0, summary used/updated=false`
- 第二次：recent ID `18`，`used_recent_tokens=30`，summary used/updated=true，`version=1, watermark=17`
- 数据库摘要和主回答都保留 Go、真实操作教学、文件配置三个事实
- 教学服务验证后已停止

### 验证与 Review

- Review 确认 summary 开关关闭时旧 count/manual/automatic 路径不要求新依赖
- Review 确认 updater 后重新读取、完整 summary system message 计数和最终二次预算顺序
- Review 确认最终容量不足时返回错误，不在 watermark 已确定后再次驱逐 raw history
- Review 确认真实失败请求都发生在模型/数据库写入前；验证服务结束后已停止
- `go test ./...` 通过
- `go test -race ./internal/recentchat ./internal/sessionsummary ./internal/fileconfig` 通过
- `go vet ./...` 通过
- `go build ./cmd/...` 通过
- `git diff --check` 通过
- 最终 Review：未发现未处理的 Critical 或 Important 问题

## Push 前隔离 Review 修复

### 发现

- push 前完整 diff Review 发现 recent store 只按 `session_id` 查询
- summary source/store 已按 `(session_id,user_id)` 查询，两个边界不一致
- 不同用户复用相同 session 时，可能混入他人 recent history，或让 summary recent 起点无法在本用户消息中找到

### RED/GREEN

- 新增 user-scoped fake store，测试先因旧 `MessageStore` 缺少双键方法而编译 RED
- 接口改为 `ListRecentBySessionUser(sessionID,userID,limit)` 后 GREEN
- MySQL 查询固定为 `WHERE session_id = ? AND user_id = ?`
- 增加 SQL 结构测试，防止以后只传 user 但误删数据库过滤条件

### 状态影响

- 只修改代码、测试和教学文档
- 未连接或修改 MySQL/Ollama/Qdrant，未执行 schema

### 验证与 Review

- `go test ./internal/recentchat` 通过
- `go test ./...` 通过
- `go test -race ./internal/recentchat ./internal/sessionsummary ./internal/fileconfig` 通过
- race 首次仅因沙箱禁止 `httptest` 监听回环端口失败，授权后同命令通过
- `go vet ./...` 通过
- `go build ./cmd/...` 通过
- `git diff --check` 通过
- 搜索确认生产 Go 代码和教学文档不再调用旧 `ListRecentBySession`
- 最终 Review：未发现其他 Critical 或 Important 问题
