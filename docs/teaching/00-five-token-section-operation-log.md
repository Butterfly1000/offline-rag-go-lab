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
